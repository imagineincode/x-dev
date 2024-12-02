package xauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"x-dev/internal/models"
)

const (
	codeVerifierLength = 128
	callbackPort       = "8080"
	callbackEndpoint   = "/callback"
	tknEndpoint        = "https://api.twitter.com/2/oauth2/token"
)

func GenerateCodeVerifier() string {
	return GenerateRandomString(codeVerifierLength)
}

func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)

	for i := range b {
		randomByte := make([]byte, 1)

		if _, err := rand.Read(randomByte); err != nil {
			log.Fatalf("failed to generate random byte: %v", err)
		}

		b[i] = charset[int(randomByte[0])%len(charset)]
	}

	return string(b)
}

func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])

	return strings.ReplaceAll(strings.ReplaceAll(challenge, "+", "-"), "/", "_")
}

func ExchangeCodeForToken(ctx context.Context, clientID, clientSecret, codeVerifier, code string) (*models.TokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", fmt.Sprintf("http://localhost:%s%s", callbackPort, callbackEndpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tknEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creating token request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:           100,
			MaxIdleConnsPerHost:    10,
			IdleConnTimeout:        90 * time.Second,
			Proxy:                  nil,
			OnProxyConnectResponse: nil,
			DialContext:            nil,
			Dial:                   nil,
			DialTLSContext:         nil,
			DialTLS:                nil,
			TLSClientConfig:        nil,
			TLSHandshakeTimeout:    0,
			DisableKeepAlives:      false,
			DisableCompression:     false,
			MaxConnsPerHost:        0,
			ResponseHeaderTimeout:  0,
			ExpectContinueTimeout:  0,
			TLSNextProto:           nil,
			ProxyConnectHeader:     nil,
			GetProxyConnectHeader:  nil,
			MaxResponseHeaderBytes: 0,
			WriteBufferSize:        0,
			ReadBufferSize:         0,
			ForceAttemptHTTP2:      false,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
		Jar: nil,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending token request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting token, status code: %d", resp.StatusCode)
	}

	var tokenResp models.TokenResponse

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("error decoding token response: %w", err)
	}

	return &tokenResp, nil
}
