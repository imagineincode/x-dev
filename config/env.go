package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// OAuthCredentials stores the OAuth 1.0a credentials
type OAuthCredentials struct {
	ConsumerKey       string
	ConsumerSecret    string
	AccessToken       string
	AccessTokenSecret string
}

// LoadOAuthConfig loads OAuth 1.0a credentials from the .env file
func LoadOAuthConfig() (*OAuthCredentials, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	consumerKey := os.Getenv("TWITTER_CONSUMER_KEY")
	consumerSecret := os.Getenv("TWITTER_CONSUMER_SECRET")
	accessToken := os.Getenv("TWITTER_ACCESS_TOKEN")
	accessTokenSecret := os.Getenv("TWITTER_ACCESS_TOKEN_SECRET")

	// Ensure all required credentials are set
	if consumerKey == "" || consumerSecret == "" || accessToken == "" || accessTokenSecret == "" {
		return nil, fmt.Errorf("one or more required environment variables are missing: TWITTER_CONSUMER_KEY, TWITTER_CONSUMER_SECRET, TWITTER_ACCESS_TOKEN, TWITTER_ACCESS_TOKEN_SECRET")
	}

	return &OAuthCredentials{
		ConsumerKey:       consumerKey,
		ConsumerSecret:    consumerSecret,
		AccessToken:       accessToken,
		AccessTokenSecret: accessTokenSecret,
	}, nil
}
