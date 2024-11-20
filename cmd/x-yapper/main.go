package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type TweetRequest struct {
	Text string `json:"text"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

var (
	authState      string
	codeVerifier   string
	codeChallenge  string
	authTokenChan  = make(chan string)
	callbackServer *http.Server
	tokenResponse  *TokenResponse
	scopes         = "tweet.read tweet.write users.read offline.access"
)

const (
	authEndpoint     = "https://twitter.com/i/oauth2/authorize"
	tokenEndpoint    = "https://api.twitter.com/2/oauth2/token"
	callbackPort     = "8080"
	callbackEndpoint = "/callback"
)

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func generateCodeVerifier() string {
	return generateRandomString(128)
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
	return strings.ReplaceAll(strings.ReplaceAll(challenge, "+", "-"), "/", "_")
}

func getEnvVar(key string) string {
	value := os.Getenv(key)
	if value == "" {
		fmt.Printf("Environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return value
}

func startCallbackServer(wg *sync.WaitGroup) {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackEndpoint, handleCallback)

	callbackServer = &http.Server{
		Addr:    ":" + callbackPort,
		Handler: mux,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := callbackServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	state := queryParams.Get("state")
	code := queryParams.Get("code")

	if state != authState {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	w.Write([]byte("Authorization successful! You can close this window."))
	authTokenChan <- code

	go func() {
		if err := callbackServer.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
	}()
}

func exchangeCodeForToken(clientID, clientSecret, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", fmt.Sprintf("http://localhost:%s%s", callbackPort, callbackEndpoint))

	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creating token request: %v", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending token request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting token, status code: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("error decoding token response: %v", err)
	}

	return &tokenResp, nil
}

func postTweet(text string, accessToken string) error {
	tweetURL := "https://api.twitter.com/2/tweets"
	tweetReq := TweetRequest{Text: text}

	jsonData, err := json.Marshal(tweetReq)
	if err != nil {
		return fmt.Errorf("error marshaling tweet request: %v", err)
	}

	req, err := http.NewRequest("POST", tweetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error posting tweet, status code: %d", resp.StatusCode)
	}

	return nil
}

func main() {
	clientID := getEnvVar("TWITTER_CLIENT_ID")
	clientSecret := getEnvVar("TWITTER_CLIENT_SECRET")

	codeVerifier = generateCodeVerifier()
	codeChallenge = generateCodeChallenge(codeVerifier)
	authState = generateRandomString(32)

	var wg sync.WaitGroup
	startCallbackServer(&wg)

	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		authEndpoint,
		clientID,
		url.QueryEscape(fmt.Sprintf("http://localhost:%s%s", callbackPort, callbackEndpoint)),
		url.QueryEscape(scopes),
		authState,
		codeChallenge,
	)

	fmt.Printf("Please open this URL in your browser to authorize the application:\n%s\n", authURL)

	code := <-authTokenChan

	tokenResponse, err := exchangeCodeForToken(clientID, clientSecret, code)
	if err != nil {
		log.Fatalf("Error exchanging code for token: %v", err)
	}

	fmt.Print("Enter your tweet text: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	tweetText := scanner.Text()

	if len(tweetText) > 280 {
		fmt.Println("Tweet is too long! Maximum length is 280 characters")
		os.Exit(1)
	}

	err = postTweet(tweetText, tokenResponse.AccessToken)
	if err != nil {
		fmt.Printf("Error posting tweet: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Tweet posted successfully!")

	wg.Wait()
}
