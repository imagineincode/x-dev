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

type User struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Username         string `json:"username"`
	MostRecentPostID string `json:"most_recent_tweet_id"`
	Verified         bool   `json:"verified"`
	VerifiedType     string `json:"verified_type"`
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
	} `json:"includes"`
	Meta struct {
		NewestID    string `json:"newest_id"`
		NextToken   string `json:"next_token"`
		OldestID    string `json:"oldest_id"`
		ResultCount int    `json:"result_count"`
	} `json:"meta"`
}

type Tweet struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	AuthorID  string `json:"author_id"`
	CreatedAt string `json:"created_at"`

	Attachments *struct {
		MediaKeys []string `json:"media_keys,omitempty"`
	} `json:"attachments,omitempty"`

	PublicMetrics struct {
		BookmarkCount   int `json:"bookmark_count"`
		ImpressionCount int `json:"impression_count"`
		LikeCount       int `json:"like_count"`
		QuoteCount      int `json:"quote_count"`
		ReplyCount      int `json:"reply_count"`
		RetweetCount    int `json:"retweet_count"`
	} `json:"public_metrics"`
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
