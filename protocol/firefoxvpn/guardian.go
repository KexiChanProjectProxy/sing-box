package firefoxvpn

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ProxyPassClaims struct {
	Sub string `json:"sub"`
	Aud string `json:"aud"`
	Iat int64  `json:"iat"`
	Nbf int64  `json:"nbf"`
	Exp int64  `json:"exp"`
	Iss string `json:"iss"`
}

type ProxyPassInfo struct {
	Token          string
	Claims         ProxyPassClaims
	QuotaLimit     string
	QuotaRemaining string
	QuotaReset     string
}

func (p ProxyPassInfo) NotBefore() time.Time {
	return time.Unix(p.Claims.Nbf, 0)
}

func (p ProxyPassInfo) ExpiresAt() time.Time {
	return time.Unix(p.Claims.Exp, 0)
}

type Entitlement struct {
	Subscribed bool   `json:"subscribed"`
	UID        int    `json:"uid"`
	MaxBytes   string `json:"maxBytes"`
}

type guardianProxyPassResponse struct {
	Token string `json:"token"`
}

func (c *ControlPlaneClient) FetchProxyPass(ctx context.Context, accessToken string) (*ProxyPassInfo, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoints.guardianBaseURL+"/api/v1/fpn/token", nil)
	if err != nil {
		return nil, fmt.Errorf("create guardian proxy-pass request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform guardian proxy-pass request: %w", err)
	}

	body, err := readResponseBody(response)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("guardian proxy-pass failed (HTTP %d)", response.StatusCode)
	}

	var proxyPassResponse guardianProxyPassResponse
	if err := json.Unmarshal(body, &proxyPassResponse); err != nil {
		return nil, fmt.Errorf("decode guardian proxy-pass response: %w", err)
	}
	if proxyPassResponse.Token == "" {
		return nil, fmt.Errorf("guardian proxy-pass response missing token")
	}

	claims, err := parseProxyPassClaims(proxyPassResponse.Token)
	if err != nil {
		return nil, fmt.Errorf("parse guardian proxy-pass claims: %w", err)
	}

	return &ProxyPassInfo{
		Token:          proxyPassResponse.Token,
		Claims:         *claims,
		QuotaLimit:     response.Header.Get("X-Quota-Limit"),
		QuotaRemaining: response.Header.Get("X-Quota-Remaining"),
		QuotaReset:     response.Header.Get("X-Quota-Reset"),
	}, nil
}

func (c *ControlPlaneClient) FetchEntitlement(ctx context.Context, accessToken string) (*Entitlement, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoints.guardianBaseURL+"/api/v1/fpn/status", nil)
	if err != nil {
		return nil, fmt.Errorf("create guardian entitlement request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform guardian entitlement request: %w", err)
	}

	body, err := readResponseBody(response)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("guardian entitlement failed (HTTP %d)", response.StatusCode)
	}

	var entitlement Entitlement
	if err := json.Unmarshal(body, &entitlement); err != nil {
		return nil, fmt.Errorf("decode guardian entitlement response: %w", err)
	}
	return &entitlement, nil
}

func parseProxyPassClaims(token string) (*ProxyPassClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid jwt: expected 3 segments, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode jwt payload: %w", err)
	}

	var claims ProxyPassClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("decode jwt claims: %w", err)
	}
	return &claims, nil
}
