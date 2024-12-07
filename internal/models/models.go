package models

import "time"

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

func SendAuthToken(code string) bool {
	select {
	case AuthTokenChan <- code:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}
