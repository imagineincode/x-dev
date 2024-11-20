package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(config *oauth2.Config, token *oauth2.Token) *Client {
	// Create an HTTP client with OAuth2 user context
	httpClient := config.Client(oauth2.NoContext, token)

	return &Client{
		httpClient: httpClient,
		baseURL:    "https://api.twitter.com/2",
	}
}

func (c *Client) createRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	return req, nil
}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get OAuth2 credentials from environment
	clientID := os.Getenv("TWITTER_CLIENT_ID")
	clientSecret := os.Getenv("TWITTER_CLIENT_SECRET")
	accessToken := os.Getenv("TWITTER_ACCESS_TOKEN")
	refreshToken := os.Getenv("TWITTER_REFRESH_TOKEN")

	if clientID == "" || clientSecret == "" || accessToken == "" || refreshToken == "" {
		log.Fatal("All OAuth2 credentials are required in .env file")
	}

	// Configure OAuth2
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://twitter.com/i/oauth2/authorize",
			TokenURL: "https://api.twitter.com/2/oauth2/token",
		},
		Scopes: []string{"tweet.read", "tweet.write", "users.read"},
	}

	// Create token from stored credentials
	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	}

	client := NewClient(config, token)

	for {
		fmt.Println("\nX Yapper")
		fmt.Println("1. Post Tweet")
		fmt.Println("2. Exit")

		var choice string
		fmt.Print("Enter your choice: ")
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			fmt.Print("Enter your tweet: ")
			var tweetText string
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				tweetText = scanner.Text()
			}

			if len(tweetText) > 280 {
				fmt.Println("Tweet is too long. Maximum 280 characters.")
				continue
			}

			fmt.Println("\nTweet Preview:")
			fmt.Println(tweetText)
			fmt.Print("\nConfirm tweet? (y/n): ")
			var confirm string
			fmt.Scanln(&confirm)

			if strings.ToLower(confirm) == "y" {
				err := client.postTweet(tweetText)
				if err != nil {
					log.Printf("Error posting tweet: %v", err)
				} else {
					fmt.Println("Tweet posted successfully!")
				}
			} else {
				fmt.Println("Tweet cancelled.")
			}

		case "2":
			fmt.Println("Goodbye!")
			return

		default:
			fmt.Println("Invalid choice. Please try again.")
		}
	}
}

func (c *Client) postTweet(text string) error {
	payload := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling tweet payload: %w", err)
	}

	req, err := c.createRequest(
		"POST",
		fmt.Sprintf("%s/tweets", c.baseURL),
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var twitterErr struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
		}
		if err := json.Unmarshal(body, &twitterErr); err == nil {
			return fmt.Errorf("Twitter API error: %s - %s", twitterErr.Title, twitterErr.Detail)
		}
		return fmt.Errorf("failed to post tweet, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
