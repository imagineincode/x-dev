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

type ThreadPost struct {
	Text  string      `json:"text"`
	Reply *LastPostID `json:"reply,omitempty"`
}

type TimelineResponse struct {
	Data     []Tweet `json:"data"`
	Includes struct {
		Users []User  `json:"users,omitempty"`
		Media []Media `json:"media,omitempty"`
	} `json:"includes,omitempty"`
	Meta struct {
		NewestID    string `json:"newest_id"`
		NextToken   string `json:"next_token"`
		OldestID    string `json:"oldest_id"`
		ResultCount int    `json:"result_count"`
	} `json:"meta"`
}

type User struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Username         string `json:"username"`
	MostRecentPostID string `json:"most_recent_tweet_id"`
	Verified         bool   `json:"verified"`
	VerifiedType     string `json:"verified_type,omitempty"`
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

type LastPostID struct {
	InReplyToPostID string `json:"in_reply_to_tweet_id"`
}

type Tweet struct {
	ID                  string            `json:"id"`
	Text                string            `json:"text"`
	EditHistoryTweetIDs []string          `json:"edit_history_tweet_ids,omitempty"`
	AuthorID            string            `json:"author_id"`
	CreatedAt           string            `json:"created_at"`
	InReplyToUserID     string            `json:"in_reply_to_user_id,omitempty"`
	ReferencedTweets    []ReferencedTweet `json:"referenced_tweets,omitempty"`
	Entities            *Entities         `json:"entities,omitempty"`
	Attachments         *Attachments      `json:"attachments,omitempty"`
	PublicMetrics       PublicMetrics     `json:"public_metrics"`
	Lang                string            `json:"lang,omitempty"`
}

type ReferencedTweet struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type Entities struct {
	URLs []URL `json:"urls,omitempty"`
}

type URL struct {
	Start       int    `json:"start"`
	End         int    `json:"end"`
	URL         string `json:"url"`
	ExpandedURL string `json:"expanded_url"`
	DisplayURL  string `json:"display_url"`
	MediaKey    string `json:"media_key,omitempty"`
}

type Attachments struct {
	MediaKeys []string `json:"media_keys,omitempty"`
}

type PublicMetrics struct {
	RetweetCount    int `json:"retweet_count"`
	ReplyCount      int `json:"reply_count"`
	LikeCount       int `json:"like_count"`
	QuoteCount      int `json:"quote_count"`
	BookmarkCount   int `json:"bookmark_count,omitempty"`
	ImpressionCount int `json:"impression_count,omitempty"`
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
	Remaining int       `json:"Remaining"`
	Limit     int       `json:"Limit"`
	ResetTime time.Time `json:"ResetTime"`
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
