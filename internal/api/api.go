package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"x-dev/internal/models"
)

var callbackServer *http.Server

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
			log.Printf("[ERROR] HTTP server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := callbackServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("[ERROR] server shutdown error: %v", err)
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
		log.Println("[WARN] Timeout sending auth code")
	}

	go func(ctx context.Context) {
		if err := callbackServer.Shutdown(ctx); err != nil {
			log.Printf("[ERROR] error shutting down server: %v", err)
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

	var maxPostLength int
	if userResp.Data.Verified {
		maxPostLength = 4000
	} else {
		maxPostLength = 280
	}

	return maxPostLength, userResp, nil
}

func SendPost(ctx context.Context, text string, accessToken string) (*models.PostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	postURL := "https://api.twitter.com/2/tweets"
	postReq := models.ThreadPost{Text: text}

	jsonData, err := json.Marshal(postReq)
	if err != nil {
		return nil, fmt.Errorf("error marshaling post request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
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
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error posting, status code: %d, response: %s",
			resp.StatusCode, string(body))
	}

	var postResp models.PostResponse
	if err := json.Unmarshal(body, &postResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling post response: %w", err)
	}

	return &postResp, nil
}

func SendReplyPost(ctx context.Context, threadPost *models.ThreadPost, accessToken string) (*models.PostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	postURL := "https://api.twitter.com/2/tweets"

	jsonData, err := json.Marshal(threadPost)
	if err != nil {
		return nil, fmt.Errorf("error marshaling post request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
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
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error posting, status code: %d, response: %s",
			resp.StatusCode, string(body))
	}

	var postResp models.PostResponse
	if err := json.Unmarshal(body, &postResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling post response: %w", err)
	}

	return &postResp, nil
}

func GetHomeTimeline(ctx context.Context, userID string, accessToken string) (*models.TimelineResponse, error) {
	timelineURL := fmt.Sprintf("https://api.twitter.com/2/users/%s/timelines/reverse_chronological", userID)
	maxResults := 5
	tweetFields := []string{"attachments", "author_id", "created_at", "id", "public_metrics", "text"}
	userFields := []string{"id", "name", "username", "verified"}

	query := url.Values{}
	query.Set("max_results", fmt.Sprintf("%d", maxResults))
	query.Set("tweet.fields", strings.Join(tweetFields, ","))
	query.Set("user.fields", strings.Join(userFields, ","))

	// Append query parameters to the URL
	fullURL := fmt.Sprintf("%s?%s", timelineURL, query.Encode())

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating user request: %w", err)
	}

	// Add headers
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Set up HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending user request: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("Response Headers:")
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	// Check for rate limit status code
	if resp.StatusCode == http.StatusTooManyRequests {
		// Read the response body
		bodyBytes, _ := io.ReadAll(resp.Body)

		// Optional: Parse the error response if needed
		var rateLimitError struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
		}
		json.Unmarshal(bodyBytes, &rateLimitError)

		// Log the full error response
		fmt.Println("Rate Limit Error Response:")
		fmt.Printf("Status Code: %d\n", resp.StatusCode)
		fmt.Printf("Error Title: %s\n", rateLimitError.Title)
		fmt.Printf("Error Detail: %s\n", rateLimitError.Detail)

		// Check rate limit status
		err := checkRateLimitStatus(client, accessToken)
		if err != nil {
			return nil, fmt.Errorf("rate limit check failed: %w", err)
		}

		return nil, fmt.Errorf("rate limited: %s - %s", rateLimitError.Title, rateLimitError.Detail)
	}

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error fetching user info, status code: %d, response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var timelineResp models.TimelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&timelineResp); err != nil {
		return nil, fmt.Errorf("error decoding user response: %w", err)
	}

	// Pretty-print the timelineResp struct as JSON
	timelineRespBytes, err := json.MarshalIndent(timelineResp, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error formatting timelineResp JSON: %w", err)
	}
	fmt.Println("Parsed Timeline Response:")
	fmt.Println(string(timelineRespBytes))

	return &timelineResp, nil
}

func checkRateLimitStatus(client *http.Client, accessToken string) error {
	req, err := http.NewRequest("GET", "https://api.twitter.com/2/usage/rate_limit_status", nil)
	if err != nil {
		return fmt.Errorf("error creating rate limit status request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending rate limit status request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code when checking rate limit: %d, response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	// Read and log the rate limit status response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading rate limit status response: %w", err)
	}

	// Parse the response to get more details
	var rateLimitStatus map[string]interface{}
	err = json.Unmarshal(bodyBytes, &rateLimitStatus)
	if err != nil {
		return fmt.Errorf("error parsing rate limit status JSON: %w, raw response: %s",
			err, string(bodyBytes))
	}

	// Log detailed rate limit information
	fmt.Println("Detailed Rate Limit Status:")
	fmt.Printf("Full Response: %+v\n", rateLimitStatus)

	// Optional: Extract and print specific rate limit details
	if resources, ok := rateLimitStatus["resources"].(map[string]interface{}); ok {
		for resourceType, endpoints := range resources {
			fmt.Printf("Resource Type: %s\n", resourceType)
			if endpointMap, ok := endpoints.(map[string]interface{}); ok {
				for endpoint, limits := range endpointMap {
					fmt.Printf("  Endpoint: %s\n", endpoint)
					fmt.Printf("  Limits: %+v\n", limits)
				}
			}
		}
	}

	return nil
}
