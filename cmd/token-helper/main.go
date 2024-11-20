package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

const (
	port         = "8080"
	callbackPath = "/callback"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get OAuth2 credentials from environment
	clientID := os.Getenv("TWITTER_CLIENT_ID")
	clientSecret := os.Getenv("TWITTER_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("TWITTER_CLIENT_ID and TWITTER_CLIENT_SECRET are required in .env file")
	}

	// Configure OAuth2
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://twitter.com/i/oauth2/authorize",
			TokenURL: "https://api.twitter.com/2/oauth2/token",
		},
		RedirectURL: fmt.Sprintf("http://localhost:%s%s", port, callbackPath),
		Scopes:      []string{"tweet.read", "tweet.write", "users.read", "offline.access"},
	}

	// Create a context for the OAuth2 flow
	ctx := context.Background()

	// Generate random state
	state := "random-state-string" // In production, use a secure random string

	// Create a channel to receive the token
	tokenChan := make(chan *oauth2.Token)

	// Set up the callback handler
	http.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		// Verify state
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}

		// Exchange the authorization code for a token
		code := r.URL.Query().Get("code")
		token, err := config.Exchange(ctx, code)
		if err != nil {
			http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
			return
		}

		// Send token through channel
		tokenChan <- token

		// Show success message
		fmt.Fprintf(w, "Authorization successful! You can close this window.")
	})

	// Start the server
	go func() {
		log.Printf("Starting server on port %s\n", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	}()

	// Generate authorization URL with offline access
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	// Print instructions
	fmt.Printf("Please visit this URL to authorize the application:\n\n%s\n\n", authURL)
	fmt.Println("Waiting for authorization...")

	// Wait for token
	token := <-tokenChan

	// Print token information
	fmt.Println("\nAuthorization successful!")
	fmt.Println("\nHere are your tokens (add these to your .env file):")
	fmt.Printf("\nTWITTER_ACCESS_TOKEN=%s", token.AccessToken)
	fmt.Printf("\nTWITTER_REFRESH_TOKEN=%s\n", token.RefreshToken)

	// Save tokens to a file
	tokenFile := "tokens.json"
	tokenData, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling token: %v", err)
	}

	if err := os.WriteFile(tokenFile, tokenData, 0600); err != nil {
		log.Fatalf("Error saving tokens: %v", err)
	}

	fmt.Printf("\nTokens have also been saved to %s\n", tokenFile)
}
