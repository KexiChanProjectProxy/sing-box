package firefoxvpn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
)

const (
	fxaClientID      = "5882386c6d801776"
	fxaOAuthScope    = "profile https://identity.mozilla.com/apps/vpn"
	piclProtocol     = "identity.mozilla.com/picl/v1/"
	quickStretchInfo = "quickStretch:"
	authPWInfo       = "authPW"
	pbkdf2Rounds     = 1000
	derivedKeyLength = 32
)

type FxALoginRequest struct {
	Email  string `json:"email"`
	AuthPW string `json:"authPW"`
}

type FxALoginResponse struct {
	SessionToken string `json:"sessionToken"`
	UID          string `json:"uid"`
	Verified     bool   `json:"verified"`
	AuthAt       int64  `json:"authAt"`
}

type FxAOAuthTokenRequest struct {
	ClientID     string `json:"client_id"`
	GrantType    string `json:"grant_type"`
	Scope        string `json:"scope"`
	AccessType   string `json:"access_type,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type FxATokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

func (c *ControlPlaneClient) Login(ctx context.Context, email string, password string) (*FxALoginResponse, error) {
	authPW, err := deriveAuthPW(email, password)
	if err != nil {
		return nil, fmt.Errorf("derive authPW: %w", err)
	}

	requestBody, err := json.Marshal(FxALoginRequest{
		Email:  email,
		AuthPW: hex.EncodeToString(authPW),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal login request: %w", err)
	}

	request, err := newJSONRequest(ctx, http.MethodPost, c.endpoints.fxaBaseURL+"/account/login", requestBody)
	if err != nil {
		return nil, fmt.Errorf("create login request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform login request: %w", err)
	}

	body, err := readResponseBody(response)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed (HTTP %d): %s", response.StatusCode, string(body))
	}

	var loginResponse FxALoginResponse
	if err := json.Unmarshal(body, &loginResponse); err != nil {
		return nil, fmt.Errorf("decode login response: %w", err)
	}
	if loginResponse.SessionToken == "" {
		return nil, fmt.Errorf("login response missing session token")
	}
	return &loginResponse, nil
}

func (c *ControlPlaneClient) ExchangeOAuthToken(ctx context.Context, sessionToken string) (*FxATokenResponse, error) {
	hawkID, hawkKey, err := deriveHawkCredentials(sessionToken, "sessionToken")
	if err != nil {
		return nil, fmt.Errorf("derive hawk credentials: %w", err)
	}

	requestBody, err := json.Marshal(FxAOAuthTokenRequest{
		ClientID:   fxaClientID,
		GrantType:  "fxa-credentials",
		Scope:      fxaOAuthScope,
		AccessType: "offline",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal oauth request: %w", err)
	}

	tokenURL := c.endpoints.fxaBaseURL + "/oauth/token"
	hawkAuthorization, err := c.hawkHeader(http.MethodPost, tokenURL, hawkID, hawkKey, requestBody)
	if err != nil {
		return nil, fmt.Errorf("build hawk authorization: %w", err)
	}

	request, err := newJSONRequest(ctx, http.MethodPost, tokenURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create oauth request: %w", err)
	}
	request.Header.Set("Authorization", hawkAuthorization)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform oauth request: %w", err)
	}

	body, err := readResponseBody(response)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth token failed (HTTP %d): %s", response.StatusCode, string(body))
	}

	var tokenResponse FxATokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("decode oauth response: %w", err)
	}
	if tokenResponse.AccessToken == "" {
		return nil, fmt.Errorf("oauth response missing access token")
	}
	if tokenResponse.RefreshToken == "" {
		return nil, fmt.Errorf("oauth response missing refresh token")
	}
	return &tokenResponse, nil
}

func (c *ControlPlaneClient) RefreshOAuthToken(ctx context.Context, refreshToken string) (*FxATokenResponse, error) {
	requestBody, err := json.Marshal(FxAOAuthTokenRequest{
		ClientID:     fxaClientID,
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		Scope:        fxaOAuthScope,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal refresh request: %w", err)
	}

	request, err := newJSONRequest(ctx, http.MethodPost, c.endpoints.fxaBaseURL+"/oauth/token", requestBody)
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform refresh request: %w", err)
	}

	body, err := readResponseBody(response)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh failed (HTTP %d): %s", response.StatusCode, string(body))
	}

	var tokenResponse FxATokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}
	if tokenResponse.AccessToken == "" {
		return nil, fmt.Errorf("refresh response missing access token")
	}
	if tokenResponse.RefreshToken == "" {
		tokenResponse.RefreshToken = refreshToken
	}
	return &tokenResponse, nil
}

func deriveAuthPW(email string, password string) ([]byte, error) {
	quickStretchedPassword := pbkdf2.Key(
		[]byte(password),
		[]byte(piclProtocol+quickStretchInfo+email),
		pbkdf2Rounds,
		derivedKeyLength,
		sha256.New,
	)

	reader := hkdf.New(sha256.New, quickStretchedPassword, []byte{0x00}, []byte(piclProtocol+authPWInfo))
	authPW := make([]byte, derivedKeyLength)
	if _, err := io.ReadFull(reader, authPW); err != nil {
		return nil, fmt.Errorf("expand authPW: %w", err)
	}
	return authPW, nil
}

func deriveHawkCredentials(tokenHex string, contextName string) (string, []byte, error) {
	token, err := hex.DecodeString(tokenHex)
	if err != nil {
		return "", nil, fmt.Errorf("decode hawk token hex: %w", err)
	}

	reader := hkdf.New(sha256.New, token, nil, []byte(piclProtocol+contextName))
	keyMaterial := make([]byte, derivedKeyLength*3)
	if _, err := io.ReadFull(reader, keyMaterial); err != nil {
		return "", nil, fmt.Errorf("expand hawk credentials: %w", err)
	}

	return hex.EncodeToString(keyMaterial[:derivedKeyLength]), keyMaterial[derivedKeyLength : derivedKeyLength*2], nil
}
