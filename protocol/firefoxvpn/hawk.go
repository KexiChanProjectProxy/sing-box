package firefoxvpn

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strings"
)

const hawkNonceLength = 6

func (c *ControlPlaneClient) hawkHeader(method string, rawURL string, hawkID string, hawkKey []byte, payload []byte) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse hawk url: %w", err)
	}

	nonceBytes := make([]byte, hawkNonceLength)
	if _, err := io.ReadFull(c.nonce, nonceBytes); err != nil {
		return "", fmt.Errorf("read hawk nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	timestamp := fmt.Sprintf("%d", c.now().Unix())

	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	payloadHash := ""
	if len(payload) > 0 {
		hash := sha256.New()
		if _, err := hash.Write([]byte("hawk.1.payload\napplication/json\n")); err != nil {
			return "", err
		}
		if _, err := hash.Write(payload); err != nil {
			return "", err
		}
		if _, err := hash.Write([]byte("\n")); err != nil {
			return "", err
		}
		payloadHash = hex.EncodeToString(hash.Sum(nil))
	}

	normalized := strings.Join([]string{
		"hawk.1.header",
		timestamp,
		nonce,
		strings.ToUpper(method),
		parsedURL.RequestURI(),
		parsedURL.Hostname(),
		port,
		payloadHash,
		"",
		"",
	}, "\n")

	mac := hmac.New(sha256.New, hawkKey)
	if _, err := mac.Write([]byte(normalized)); err != nil {
		return "", err
	}

	header := fmt.Sprintf(`Hawk id="%s", ts="%s", nonce="%s", mac="%s"`, hawkID, timestamp, nonce, hex.EncodeToString(mac.Sum(nil)))
	if payloadHash != "" {
		header += fmt.Sprintf(`, hash="%s"`, payloadHash)
	}
	return header, nil
}

type randReader struct{}

func (randReader) Read(p []byte) (int, error) {
	return rand.Read(p)
}
