package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	"x-dev/internal/models"
)

var (
	maxPostLength  int
	callbackServer *http.Server
)

const (
	authEndpoint     = "https://twitter.com/i/oauth2/authorize"
	tknEndpoint      = "https://api.twitter.com/2/oauth2/token"
	callbackPort     = "8080"
	callbackEndpoint = "/callback"
)

func StartCallbackServer(ctx context.Context, wGroup *sync.WaitGroup, authState string) {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackEndpoint, func(w http.ResponseWriter, r *http.Request) {
		handleCallback(ctx, w, r, authState)
	})

	callbackServer = &http.Server{
		Addr:              ":" + callbackPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          log.New(os.Stderr, "http: ", log.LstdFlags),
	}

	wGroup.Add(1)

	go func() {
		defer wGroup.Done()

		if err := callbackServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := callbackServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()
}

func handleCallback(ctx context.Context, w http.ResponseWriter, r *http.Request, authState string) {
	queryParams := r.URL.Query()
	state := queryParams.Get("state")
	code := queryParams.Get("code")

	if state != authState {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)

		return
	}

	_, err := w.Write([]byte("Authorization successful! You can close this window."))
	if err != nil {
		log.Printf("error writing response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	if !models.SendAuthToken(code) {
		log.Println("Timeout sending auth code")
	}

	go func(ctx context.Context) {
		if err := callbackServer.Shutdown(ctx); err != nil {
			log.Printf("error shutting down server: %v", err)
		}
	}(ctx)
}

func CheckAccountType(ctx context.Context, accessToken string) (int, models.UserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	userURL := "https://api.twitter.com/2/users/me?user.fields=id,name,most_recent_tweet_id,username,verified,verified_type"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return 0, models.UserResponse{}, fmt.Errorf("error creating user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
		Jar: nil,
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, models.UserResponse{}, fmt.Errorf("error sending user request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)

		return 0, models.UserResponse{}, fmt.Errorf("error fetching user info, status code: %d, response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var userResp models.UserResponse

	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return 0, models.UserResponse{}, fmt.Errorf("error decoding user response: %w", err)
	}

	formattedJSON, err := json.MarshalIndent(userResp, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		return 0, userResp, err
	}

	fmt.Printf("Full Response:\n%s\n", string(formattedJSON))

	var maxPostLength int
	if userResp.Data.Verified {
		maxPostLength = 4000
	} else {
		maxPostLength = 280
	}

	return maxPostLength, userResp, nil
}

func PostTweet(ctx context.Context, text string, accessToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tweetURL := "https://api.twitter.com/2/tweets"
	tweetReq := models.TweetRequest{Text: text}

	jsonData, err := json.Marshal(tweetReq)
	if err != nil {
		return fmt.Errorf("error marshaling tweet request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tweetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
		Jar: nil,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error posting tweet, status code: %d, response: %s",
			resp.StatusCode, string(body))
	}

	return nil
}
