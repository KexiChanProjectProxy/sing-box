package warpapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	stdTLS "crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/ntp"
)

const (
	DefaultBaseURL    = "https://api.cloudflareclient.com/v0a4471"
	DefaultEndpointV4 = "162.159.198.1"
	DefaultEndpointV6 = "2606:4700:103::"
	DefaultPort       = 443
	DefaultModel      = "PC"
	DefaultLocale     = "en_US"

	userAgent     = "WARP for Android"
	clientVersion = "a-6.35-4471"
	contentType   = "application/json; charset=UTF-8"

	// CodeInvalidPublicKey is returned when the enrolled key is rejected.
	CodeInvalidPublicKey = 1001

	maxErrorBody = 512
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	now        func() time.Time
}

func NewClient(httpClient *http.Client, baseURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		now:        time.Now,
	}
}

// NewHTTPClient builds an HTTP client for the registration API. A nil dialer
// uses the system dialer.
func NewHTTPClient(ctx context.Context, dialer N.Dialer) *http.Client {
	var dialContext func(context.Context, string, string) (net.Conn, error)
	if dialer == nil {
		dialContext = (&net.Dialer{}).DialContext
	} else {
		dialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
		}
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig: &stdTLS.Config{
				Time:    ntp.TimeFuncFromContext(ctx),
				RootCAs: adapter.RootPoolFromContext(ctx),
			},
			DialContext: dialContext,
		},
	}
}

type RegisterOptions struct {
	Model     string
	Locale    string
	Name      string
	AccessJWT string
	License   string
}

type registerRequest struct {
	Key          string `json:"key"`
	InstallID    string `json:"install_id"`
	FcmToken     string `json:"fcm_token"`
	Tos          string `json:"tos"`
	Model        string `json:"model"`
	SerialNumber string `json:"serial_number"`
	OSVersion    string `json:"os_version"`
	KeyType      string `json:"key_type"`
	TunnelType   string `json:"tunnel_type"`
	Locale       string `json:"locale"`
}

type deviceUpdateRequest struct {
	Key        string `json:"key"`
	KeyType    string `json:"key_type"`
	TunnelType string `json:"tunnel_type"`
	Name       string `json:"name,omitempty"`
}

type licenseRequest struct {
	License string `json:"license"`
}

// Register creates a new device, enrolls key for MASQUE and optionally applies
// a WARP+ license.
func (c *Client) Register(ctx context.Context, key *ecdsa.PrivateKey, options RegisterOptions) (*Registration, error) {
	if options.Model == "" {
		options.Model = DefaultModel
	}
	if options.Locale == "" {
		options.Locale = DefaultLocale
	}
	// The initial registration mimics the Android client's WireGuard-mode
	// registration; the key is a placeholder and is replaced by Enroll.
	placeholderKey := make([]byte, 32)
	serial := make([]byte, 8)
	_, err := io.ReadFull(rand.Reader, placeholderKey)
	if err != nil {
		return nil, err
	}
	_, err = io.ReadFull(rand.Reader, serial)
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	if options.AccessJWT != "" {
		header.Set("CF-Access-Jwt-Assertion", options.AccessJWT)
	}
	var device AccountData
	err = c.do(ctx, http.MethodPost, "/reg", "", header, registerRequest{
		Key:          base64.StdEncoding.EncodeToString(placeholderKey),
		Tos:          c.now().Format("2006-01-02T15:04:05.000-07:00"),
		Model:        options.Model,
		SerialNumber: hex.EncodeToString(serial),
		KeyType:      "curve25519",
		TunnelType:   "wireguard",
		Locale:       options.Locale,
	}, &device)
	if err != nil {
		return nil, E.Cause(err, "register device")
	}
	if device.ID == "" || device.Token == "" {
		return nil, E.New("register device: response is missing id or token")
	}
	enrolled, err := c.Enroll(ctx, device.ID, device.Token, key, options.Name)
	if err != nil {
		return nil, err
	}
	if options.License != "" {
		var account *AccountData
		account, err = c.ApplyLicense(ctx, device.ID, device.Token, options.License)
		if err != nil {
			return nil, err
		}
		enrolled.Account.License = account.Account.License
		if enrolled.Account.License == "" {
			enrolled.Account.License = options.License
		}
	}
	return NewRegistration(device.ID, device.Token, key, enrolled, c.now())
}

// Enroll replaces the device key with key and switches the device to MASQUE.
func (c *Client) Enroll(ctx context.Context, deviceID string, token string, key *ecdsa.PrivateKey, name string) (*AccountData, error) {
	publicKey, err := EncodePublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	var device AccountData
	err = c.do(ctx, http.MethodPatch, "/reg/"+deviceID, token, nil, deviceUpdateRequest{
		Key:        publicKey,
		KeyType:    "secp256r1",
		TunnelType: "masque",
		Name:       name,
	}, &device)
	if err != nil {
		return nil, E.Cause(err, "enroll MASQUE key")
	}
	return &device, nil
}

// ApplyLicense binds a WARP+ license to the device's account.
func (c *Client) ApplyLicense(ctx context.Context, deviceID string, token string, license string) (*AccountData, error) {
	var account Account
	err := c.do(ctx, http.MethodPut, "/reg/"+deviceID+"/account", token, nil, licenseRequest{License: license}, &account)
	if err != nil {
		return nil, E.Cause(err, "apply license")
	}
	return &AccountData{ID: deviceID, Account: account}, nil
}

// GetDevice returns the current device object.
func (c *Client) GetDevice(ctx context.Context, deviceID string, token string) (*AccountData, error) {
	var device AccountData
	err := c.do(ctx, http.MethodGet, "/reg/"+deviceID, token, nil, nil, &device)
	if err != nil {
		return nil, E.Cause(err, "get device")
	}
	return &device, nil
}

// DeleteDevice removes the device registration from Cloudflare.
func (c *Client) DeleteDevice(ctx context.Context, deviceID string, token string) error {
	err := c.do(ctx, http.MethodDelete, "/reg/"+deviceID, token, nil, nil, nil)
	if err != nil {
		return E.Cause(err, "delete device")
	}
	return nil
}

// NewRegistration builds a Registration from an enrolled device object.
func NewRegistration(deviceID string, token string, key *ecdsa.PrivateKey, device *AccountData, now time.Time) (*Registration, error) {
	encodedKey, err := EncodePrivateKey(key)
	if err != nil {
		return nil, err
	}
	registration := &Registration{
		Version:     RegistrationVersion,
		DeviceID:    deviceID,
		AccessToken: token,
		PrivateKey:  encodedKey,
		AccountID:   device.Account.ID,
		License:     device.Account.License,
		CreatedAt:   now,
	}
	if len(device.Config.Peers) == 0 {
		return nil, E.New("enrolled device has no MASQUE peer")
	}
	peer := device.Config.Peers[0]
	registration.EndpointV4, err = NormalizeEndpoint(peer.Endpoint.V4)
	if err != nil {
		return nil, err
	}
	registration.EndpointV6, err = NormalizeEndpoint(peer.Endpoint.V6)
	if err != nil {
		return nil, err
	}
	if peer.PublicKey != "" {
		registration.EndpointPublicKey, err = NormalizePublicKey(peer.PublicKey)
		if err != nil {
			return nil, E.Cause(err, "endpoint public key")
		}
	}
	addresses := device.Config.Interface.Addresses
	if addresses.V4 != "" {
		registration.AddressV4, err = netip.ParseAddr(addresses.V4)
		if err != nil {
			return nil, E.Cause(err, "parse assigned IPv4 address")
		}
	}
	if addresses.V6 != "" {
		registration.AddressV6, err = netip.ParseAddr(addresses.V6)
		if err != nil {
			return nil, E.Cause(err, "parse assigned IPv6 address")
		}
	}
	if !registration.AddressV4.IsValid() && !registration.AddressV6.IsValid() {
		return nil, E.New("enrolled device has no assigned tunnel address")
	}
	return registration, nil
}

// APIError is a non-2xx response from the registration API. It never contains
// request headers or tokens.
type APIError struct {
	Status  int
	Code    int
	Message string
}

func (e *APIError) Error() string {
	var builder strings.Builder
	builder.WriteString("cloudflare API status ")
	builder.WriteString(strconv.Itoa(e.Status))
	if e.Code != 0 {
		builder.WriteString(", code ")
		builder.WriteString(strconv.Itoa(e.Code))
	}
	if e.Message != "" {
		builder.WriteString(": ")
		builder.WriteString(e.Message)
	}
	return builder.String()
}

type apiErrorResponse struct {
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) do(ctx context.Context, method string, path string, token string, header http.Header, requestBody any, responseBody any) error {
	var body io.Reader
	if requestBody != nil {
		content, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(content)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	for key, values := range header {
		request.Header[key] = values
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("CF-Client-Version", clientVersion)
	if requestBody != nil {
		request.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode/100 != 2 {
		apiErr := &APIError{Status: response.StatusCode}
		var errorResponse apiErrorResponse
		if json.Unmarshal(content, &errorResponse) == nil && len(errorResponse.Errors) > 0 {
			apiErr.Code = errorResponse.Errors[0].Code
			apiErr.Message = errorResponse.Errors[0].Message
		} else {
			message := strings.TrimSpace(string(content))
			if len(message) > maxErrorBody {
				message = message[:maxErrorBody]
			}
			apiErr.Message = message
		}
		return apiErr
	}
	if responseBody == nil || len(content) == 0 {
		return nil
	}
	err = json.Unmarshal(content, responseBody)
	if err != nil {
		return E.Cause(err, "decode response")
	}
	return nil
}
