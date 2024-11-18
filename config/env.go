// config/env.go
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv" //go get github.com/joho/godotenv
)

// Method 1: Using godotenv library
func LoadEnvConfig() (string, error) {
	if err := godotenv.Load(); err != nil {
		return "", fmt.Errorf("error loading .env file: %v", err)
	}

	token := os.Getenv("TWITTER_BEARER_TOKEN")
	if token == "" {
		return "", fmt.Errorf("TWITTER_BEARER_TOKEN is not set in .env file")
	}

	return token, nil
}

// Method 2: Direct environment variable
func LoadDirectEnvConfig() (string, error) {
	token := os.Getenv("TWITTER_BEARER_TOKEN")
	if token == "" {
		return "", fmt.Errorf("TWITTER_BEARER_TOKEN environment variable is not set")
	}

	return token, nil
}

// Method 3: Environment with fallback to file
func LoadEnvWithFallback(configPath string) (string, error) {
	// Try environment first
	token := os.Getenv("TWITTER_BEARER_TOKEN")
	if token != "" {
		return token, nil
	}

	// Fallback to .env file
	if err := godotenv.Load(configPath); err != nil {
		return "", fmt.Errorf("error loading .env file: %v", err)
	}

	token = os.Getenv("TWITTER_BEARER_TOKEN")
	if token == "" {
		return "", fmt.Errorf("TWITTER_BEARER_TOKEN is not set in environment or .env file")
	}

	return token, nil
}
