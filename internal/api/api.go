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
	"strconv"
	"strings"
	"sync"
	"time"

	"x-dev/internal/models"
	"x-dev/internal/prompt"
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
			fmt.Println(prompt.Failed("[ERROR]"), "HTTP server error: ", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := callbackServer.Shutdown(shutdownCtx); err != nil {
			fmt.Println(prompt.Failed("[ERROR]"), "server shutdown error: ", err)
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
		fmt.Println(prompt.Failed("[ERROR] "), "error writing response: ", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if !models.SendAuthToken(code) {
		fmt.Println(prompt.Warn("[WARN] "), "Timeout sending auth code")
	}

	go func(ctx context.Context) {
		if err := callbackServer.Shutdown(ctx); err != nil {
			fmt.Println(prompt.Failed("[ERROR]"), "error shutting down server: ", err)
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

func GetHomeTimeline(ctx context.Context, userID string, accessToken string) (*models.TimelineResponse, *models.RateLimitInfo, error) {
	timelineURL := fmt.Sprintf("https://api.twitter.com/2/users/%s/timelines/reverse_chronological", userID)
	maxResults := 5
	tweetFields := []string{"attachments", "author_id", "created_at", "id", "public_metrics", "text"}
	userFields := []string{"id", "name", "username", "verified"}

	query := url.Values{}
	query.Set("max_results", fmt.Sprintf("%d", maxResults))
	query.Set("tweet.fields", strings.Join(tweetFields, ","))
	query.Set("user.fields", strings.Join(userFields, ","))

	fullURL := fmt.Sprintf("%s?%s", timelineURL, query.Encode())

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating user request: %w", err)
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
		return nil, nil, fmt.Errorf("error sending user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		rateLimitError, err := handle429Response(resp)
		if err != nil {
			return nil, nil, fmt.Errorf("error processing rate limit error: %w", err)
		}

		fmt.Printf("Rate Limit Exceeded:\n")
		fmt.Printf("Retry After: %d seconds\n", rateLimitError.RetryAfterSecs)
		fmt.Printf("Response Body: %s\n", rateLimitError.ResponseBody)

		return nil, rateLimitError.Info, rateLimitError
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("error fetching user info, status code: %d, response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	rateLimitInfo, err := extractRateLimitInfo(resp)
	if err != nil {
		return nil, nil, fmt.Errorf("error extracting rate limit info: %w", err)
	}

	var timelineResp models.TimelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&timelineResp); err != nil {
		return nil, nil, fmt.Errorf("error decoding user response: %w", err)
	}

	return &timelineResp, rateLimitInfo, nil
}

func extractRateLimitInfo(resp *http.Response) (*models.RateLimitInfo, error) {
	remainingStr := resp.Header.Get("X-Rate-Limit-Remaining")
	limitStr := resp.Header.Get("X-Rate-Limit-Limit")
	resetStr := resp.Header.Get("X-Rate-Limit-Reset")

	if remainingStr == "" || limitStr == "" || resetStr == "" {
		return nil, nil
	}

	remaining, err := strconv.Atoi(remainingStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing remaining rate limit: %w", err)
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing rate limit: %w", err)
	}

	resetTimestamp, err := strconv.ParseInt(resetStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing rate limit reset timestamp: %w", err)
	}

	resetTime := time.Unix(resetTimestamp, 0)

	return &models.RateLimitInfo{
		Remaining: remaining,
		Limit:     limit,
		ResetTime: resetTime,
	}, nil
}

func handle429Response(resp *http.Response) (*models.RateLimitError, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading 429 response body: %w", err)
	}
	bodyStr := string(bodyBytes)

	rateLimitInfo, err := extractRateLimitInfo(resp)
	if err != nil {
		fmt.Printf("Warning: Could not extract rate limit info: %v\n", err)
	}

	retryAfterStr := resp.Header.Get("Retry-After")
	retryAfter := 0
	if retryAfterStr != "" {
		retryAfter, _ = strconv.Atoi(retryAfterStr)
	}

	rateLimitError := &models.RateLimitError{
		Info:           rateLimitInfo,
		ResponseBody:   bodyStr,
		RetryAfterSecs: retryAfter,
	}

	if rateLimitInfo != nil {
		fmt.Printf("Rate Limit (429) - Remaining: %d/%d, Reset Time: %s\n",
			rateLimitInfo.Remaining,
			rateLimitInfo.Limit,
			rateLimitInfo.ResetTime.Format(time.RFC1123))
	}

	return rateLimitError, nil
}
