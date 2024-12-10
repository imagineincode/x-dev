package models

import (
	"fmt"
	"time"
)

const (
	AuthEndpoint     = "https://twitter.com/i/oauth2/authorize"
	CallbackPort     = "8080"
	CallbackEndpoint = "/callback"
)

var (
	AuthTokenChan = make(chan string, 1)
	Scopes        = "tweet.read tweet.write users.read offline.access"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

type UserResponse struct {
	Data struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Username         string `json:"username"`
		MostRecentPostID string `json:"most_recent_tweet_id"`
		Verified         bool   `json:"verified"`
		VerifiedType     string `json:"verified_type"`
	} `json:"data"`
}

type PostResponse struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

type LastPostID struct {
	InReplyToPostID string `json:"in_reply_to_tweet_id"`
}

type ThreadPost struct {
	Text  string      `json:"text"`
	Reply *LastPostID `json:"reply,omitempty"`
}

type TimelineResponse struct {
	PostData []Tweet `json:"data"`
	Includes struct {
		Users []User  `json:"users,omitempty"`
		Media []Media `json:"media,omitempty"`
	} `json:"includes,omitempty"`
	Meta map[string]interface{} `json:"meta"`
}

type Tweet struct {
	ID            string              `json:"id"`
	Text          string              `json:"text"`
	AuthorID      string              `json:"author_id"`
	CreatedAt     string              `json:"created_at"`
	Attachments   map[string][]string `json:"attachments,omitempty"`
	PublicMetrics map[string]int      `json:"public_metrics,omitempty"`
}

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

type Media struct {
	MediaKey string `json:"media_key"`
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
}

type RateLimitError struct {
	Info           *RateLimitInfo
	ResponseBody   string
	RetryAfterSecs int
}

type RateLimitInfo struct {
	Remaining int
	Limit     int
	ResetTime time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("Rate limit exceeded. Retry after %d seconds. Details: %s",
		e.RetryAfterSecs, e.ResponseBody)
}

func SendAuthToken(code string) bool {
	select {
	case AuthTokenChan <- code:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}
