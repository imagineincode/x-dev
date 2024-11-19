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

	"x-dev/config"

	"github.com/dghubble/oauth1"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(consumerKey, consumerSecret, accessToken, accessTokenSecret string) *Client {
	config := oauth1.NewConfig(consumerKey, consumerSecret)
	token := oauth1.NewToken(accessToken, accessTokenSecret)

	httpClient := config.Client(oauth1.NoContext, token)

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
	oauthCredentials, err := config.LoadOAuthConfig()
	if err != nil {
		log.Fatalf("Error loading OAuth config: %v", err)
	}

	consumerKey := oauthCredentials.ConsumerKey
	consumerSecret := oauthCredentials.ConsumerSecret
	accessToken := oauthCredentials.AccessToken
	accessTokenSecret := oauthCredentials.AccessTokenSecret

	client := NewClient(consumerKey, consumerSecret, accessToken, accessTokenSecret)

	for {
		fmt.Println("\nX Yapper")
		fmt.Println("1. Post Tweet")
		fmt.Println("2. Exit")

		var choice string
		fmt.Print("Enter your choice: ")
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			userID, err := client.getUserID()
			if err != nil {
				log.Printf("Error getting user ID: %v", err)
				continue
			}

			// Prompt for tweet text
			fmt.Print("Enter your tweet (max 280 characters): ")
			var tweetText string
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				tweetText = scanner.Text()
			}

			// Validate tweet length
			if len(tweetText) > 280 {
				fmt.Println("Tweet is too long. Maximum 280 characters.")
				continue
			}

			// Preview tweet
			fmt.Println("\nTweet Preview:")
			fmt.Println(tweetText)
			fmt.Print("\nConfirm tweet? (y/n): ")
			var confirm string
			fmt.Scanln(&confirm)

			if strings.ToLower(confirm) == "y" {
				err = client.postTweet(userID, tweetText)
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

func (c *Client) getUserID() (string, error) {
	req, err := c.createRequest("GET", c.baseURL+"/users/me", nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Data.ID, nil
}

func (c *Client) postTweet(userID, text string) error {
	// Prepare tweet payload
	payload := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}

	// Convert payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Create request
	req, err := c.createRequest(
		"POST",
		fmt.Sprintf("%s/tweets", c.baseURL),
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to post tweet, status: %d", resp.StatusCode)
	}

	return nil
}
