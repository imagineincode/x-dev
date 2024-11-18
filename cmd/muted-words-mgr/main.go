package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"x-dev/config"
)

type MutedWord struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient creates a new Twitter API client
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{},
		baseURL:    "https://api.twitter.com/2",
		token:      token,
	}
}

// createRequest creates an HTTP request with proper headers
func (c *Client) createRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	req.Header.Add("Content-Type", "application/json")
	return req, nil
}

func main() {
	// Load configuration from .env
	token, err := config.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	client := NewClient(token)

	// Simple command-line interface
	for {
		fmt.Println("\nTwitter Muted Words Manager")
		fmt.Println("1. List muted words")
		fmt.Println("2. Add muted word")
		fmt.Println("3. Remove muted word")
		fmt.Println("4. Exit")

		var choice string
		fmt.Print("Enter your choice (1-4): ")
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			// Get user ID first (required for v2 API)
			userID, err := client.getUserID()
			if err != nil {
				log.Printf("Error getting user ID: %v", err)
				continue
			}

			// List muted words
			mutedWords, err := client.listMutedWords(userID)
			if err != nil {
				log.Printf("Error listing muted words: %v", err)
				continue
			}

			fmt.Println("\nCurrent muted words:")
			for _, word := range mutedWords {
				fmt.Printf("- %s (ID: %s)\n", word.Text, word.ID)
			}

		case "2":
			// Get user ID
			userID, err := client.getUserID()
			if err != nil {
				log.Printf("Error getting user ID: %v", err)
				continue
			}

			var word string
			fmt.Print("Enter word to mute: ")
			fmt.Scanln(&word)

			err = client.addMutedWord(userID, strings.TrimSpace(word))
			if err != nil {
				log.Printf("Error adding muted word: %v", err)
				continue
			}
			fmt.Printf("Successfully muted word: %s\n", word)

		case "3":
			// Get user ID
			userID, err := client.getUserID()
			if err != nil {
				log.Printf("Error getting user ID: %v", err)
				continue
			}

			var wordID string
			fmt.Print("Enter muted word ID to remove: ")
			fmt.Scanln(&wordID)

			err = client.removeMutedWord(userID, wordID)
			if err != nil {
				log.Printf("Error removing muted word: %v", err)
				continue
			}
			fmt.Println("Successfully removed muted word")

		case "4":
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

func (c *Client) listMutedWords(userID string) ([]MutedWord, error) {
	req, err := c.createRequest("GET", fmt.Sprintf("%s/users/%s/muted_words", c.baseURL, userID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []MutedWord `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *Client) addMutedWord(userID, word string) error {
	payload := struct {
		Text string `json:"text"`
	}{
		Text: word,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := c.createRequest(
		"POST",
		fmt.Sprintf("%s/users/%s/muted_words", c.baseURL, userID),
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("failed to add muted word, status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) removeMutedWord(userID, wordID string) error {
	req, err := c.createRequest(
		"DELETE",
		fmt.Sprintf("%s/users/%s/muted_words/%s", c.baseURL, userID, wordID),
		nil,
	)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to remove muted word, status: %d", resp.StatusCode)
	}

	return nil
}
