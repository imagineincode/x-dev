package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
)

type TweetRequest struct {
	Text string `json:"text"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

type EditorConfig struct {
	editors   []string
	envEditor string
}

type Editor struct {
	path string
	name string
}

var (
	authState      string
	codeVerifier   string
	codeChallenge  string
	authTokenChan  = make(chan string)
	callbackServer *http.Server
	scopes         = "tweet.read tweet.write users.read offline.access"
	cyan           = color.New(color.FgCyan).SprintFunc()
	red            = color.New(color.FgRed).SprintFunc()
	green          = color.New(color.FgGreen).SprintFunc()
)

const (
	authEndpoint     = "https://twitter.com/i/oauth2/authorize"
	tokenEndpoint    = "https://api.twitter.com/2/oauth2/token"
	callbackPort     = "8080"
	callbackEndpoint = "/callback"
	maxTweetLength   = 280
)

func loadConfig() (clientID, clientSecret string, err error) {
	clientID = os.Getenv("TWITTER_CLIENT_ID")
	clientSecret = os.Getenv("TWITTER_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", "", fmt.Errorf("missing required environment variables")
	}

	return clientID, clientSecret, nil
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	r := rand.New(rand.NewPCG(
		uint64(time.Now().UnixNano()),
		uint64(time.Now().UnixNano()+1),
	))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.IntN(len(charset))]
	}
	return string(b)
}

func generateCodeVerifier() string {
	return generateRandomString(128)
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
	return strings.ReplaceAll(strings.ReplaceAll(challenge, "+", "-"), "/", "_")
}

func startCallbackServer(ctx context.Context, wg *sync.WaitGroup) {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackEndpoint, handleCallback)

	callbackServer = &http.Server{
		Addr:    ":" + callbackPort,
		Handler: mux,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := callbackServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf(red("[ERROR] "), "HTTP server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := callbackServer.Shutdown(shutdownCtx); err != nil {
			log.Printf(red("[ERROR] "), "server shutdown error: %v", err)
		}
	}()
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	state := queryParams.Get("state")
	code := queryParams.Get("code")

	if state != authState {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	_, err := w.Write([]byte("Authorization successful! You can close this window."))
	if err != nil {
		log.Printf("error writing response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	authTokenChan <- code

	go func() {
		if err := callbackServer.Shutdown(context.Background()); err != nil {
			log.Printf(red("[ERROR] "), "error shutting down server: %v", err)
		}
	}()
}

func exchangeCodeForToken(clientID, clientSecret, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", fmt.Sprintf("http://localhost:%s%s", callbackPort, callbackEndpoint))

	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf(red("[ERROR] "), "error creating token request: %v", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(red("[ERROR] "), "error sending token request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(red("[ERROR] "), "error getting token, status code: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf(red("[ERROR] "), "error decoding token response: %v", err)
	}

	return &tokenResp, nil
}

func postTweet(text string, accessToken string) error {
	tweetURL := "https://api.twitter.com/2/tweets"
	tweetReq := TweetRequest{Text: text}

	jsonData, err := json.Marshal(tweetReq)
	if err != nil {
		return fmt.Errorf(red("[ERROR] "), "error marshaling tweet request: %v", err)
	}

	req, err := http.NewRequest("POST", tweetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf(red("[ERROR] "), "error creating request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(red("[ERROR] "), "error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf(red("[ERROR] "), "error posting tweet, status code: %d, response: %s",
			resp.StatusCode, string(body))
	}

	return nil
}

func validateTweetLength(text string) error {
	if len(text) > maxTweetLength {
		return fmt.Errorf(red("[ERROR] "), "tweet exceeds maximum length of %d characters", maxTweetLength)
	}
	return nil
}

func newEditorConfig() *EditorConfig {
	return &EditorConfig{
		editors:   []string{"nvim", "vim", "nano", "emacs", "notepad"},
		envEditor: os.Getenv("EDITOR"),
	}
}

func (ec *EditorConfig) chooseEditor() (*Editor, error) {
	if ec.envEditor != "" {
		if path, err := exec.LookPath(ec.envEditor); err == nil {
			return &Editor{path: path, name: ec.envEditor}, nil
		}
	}

	for _, editor := range ec.editors {
		if path, err := exec.LookPath(editor); err == nil {
			return &Editor{path: path, name: editor}, nil
		}
	}

	return nil, fmt.Errorf(red("[ERROR] "), "no suitable editor found")
}

func (e *Editor) openEditor() (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	tmpfile, err := os.CreateTemp("", fmt.Sprintf("posteditor_%s_*.txt", timestamp))
	if err != nil {
		return "", fmt.Errorf(red("[ERROR] "), "failed to create temp file: %w", err)
	}
	tmpfileName := tmpfile.Name()
	defer os.Remove(tmpfileName)
	tmpfile.Close()

	cmd := exec.Command(e.path, tmpfileName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(red("[ERROR] "), "failed to run editor %s: %w", e.name, err)
	}

	content, err := os.ReadFile(tmpfileName)
	if err != nil {
		return "", fmt.Errorf(red("[ERROR] "), "failed to read temp file: %w", err)
	}

	return strings.TrimRight(string(content), "\n\r\t "), nil
}

func wrapText(text string, lineWidth int) string {
	text = strings.TrimSpace(text)

	paragraphs := strings.Split(text, "\n")
	wrappedParagraphs := []string{}

	for _, paragraph := range paragraphs {
		if paragraph == "" {
			continue
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}

		lines := []string{}
		currentLine := words[0]

		for _, word := range words[1:] {
			if len(currentLine)+len(word)+1 > lineWidth {
				lines = append(lines, currentLine)
				currentLine = word
			} else {
				currentLine += " " + word
			}
		}

		lines = append(lines, currentLine)

		wrappedParagraphs = append(wrappedParagraphs, strings.Join(lines, "\n"))
	}

	return strings.Join(wrappedParagraphs, "\n\n")
}

func showPreviewPrompt(content string) (bool, error) {
	wrappedContent := wrapText(content, 60)

	fmt.Println("\nPost Preview:")
	fmt.Println("--------------------------------------------------")
	fmt.Println(wrappedContent)
	fmt.Println("--------------------------------------------------")
	fmt.Println("")

	prompt := promptui.Select{
		Label: "Choose an action",
		Items: []string{"Send Post", "Discard"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}?",
			Active:   "\U0001F449 {{ . | cyan }}",
			Inactive: "  {{ . | white }}",
			Selected: "\U0001F680 {{ . | green }}",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return false, fmt.Errorf(red("[ERROR] "), "preview selection failed: %w", err)
	}

	return idx == 0, nil // index 0 is "Send Post"
}

func runPrompts(tokenResp *TokenResponse) error {
	config := newEditorConfig()
	editor, err := config.chooseEditor()
	if err != nil {
		return fmt.Errorf(red("[ERROR] "), "editor initialization failed: %w", err)
	}

	fmt.Println(`      
    \ \  //  
     \ \//  
      \ \ 
     //\ \
    //  \ \
     yapper`)
	fmt.Println("")

	for {
		prompt := promptui.Select{
			Label: "Choose an action",
			Items: []string{"Start New Post", "Exit"},
			Templates: &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   "-> {{ . | cyan }}",
				Inactive: "  {{ . | white }}",
				Selected: "\U0001F44D {{ . | green }}",
			},
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return fmt.Errorf(red("[ERROR] "), "prompt failed: %w", err)
		}

		if idx == 1 { // Exit option
			fmt.Println("Exiting editor...")
			return nil
		}

		content, err := editor.openEditor()
		if err != nil {
			log.Printf(red("[ERROR] "), "error: %v", err)
			return nil
		}

		if strings.TrimSpace(content) == "" {
			fmt.Println("No content entered. Returning to main prompt.")
			return nil
		}

		if err := validateTweetLength(content); err != nil {
			fmt.Println(err)
			return nil
		}

		shouldSend, err := showPreviewPrompt(content)
		if err != nil {
			log.Printf(red("[ERROR] "), "preview failed: %v", err)
			continue
		}

		if shouldSend {
			err = postTweet(content, tokenResp.AccessToken)
			if err != nil {
				fmt.Printf(red("[ERROR] "), "error posting tweet: %v\n", err)
			} else {
				fmt.Println("\U00002705 Successfully sent post!")
			}

		} else {
			fmt.Println("\U0000274C Post discarded.")
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println(cyan("[INFO] "), "getting environment variables.")

	clientID, clientSecret, err := loadConfig()
	if err != nil {
		log.Printf(red("[ERROR] "), "failed to load configuration: %v", err)
		log.Printf("Please ensure TWITTER_CLIENT_ID and TWITTER_CLIENT_SECRET are set")
		os.Exit(1)
	}

	fmt.Println(green("[OK]   "), "environment variables set.")
	fmt.Println(cyan("[INFO] "), "starting authentication service.")

	codeVerifier = generateCodeVerifier()
	codeChallenge = generateCodeChallenge(codeVerifier)
	authState = generateRandomString(32)

	var wg sync.WaitGroup
	startCallbackServer(ctx, &wg)

	authURL := fmt.Sprintf("\n%s?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		authEndpoint,
		clientID,
		url.QueryEscape(fmt.Sprintf("http://localhost:%s%s", callbackPort, callbackEndpoint)),
		url.QueryEscape(scopes),
		authState,
		codeChallenge,
	)

	fmt.Printf("Please open this URL in your browser to authorize the application:\n%s\n", authURL)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case code := <-authTokenChan:
		tokenResponse, err := exchangeCodeForToken(clientID, clientSecret, code)
		if err != nil {
			log.Fatalf(red("[FATAL] "), "error exchanging code for token: %v", err)
		}

		fmt.Println(green("[OK]  "), "authentication successful, starting x-yapper...")

		if err := runPrompts(tokenResponse); err != nil {
			log.Fatalf(red("[FATAL] "), "error: %v", err)
		}

	case <-sigChan:
		fmt.Println(cyan("[INFO] "), "received interrupt, shutting down...")
		cancel()
	}

	wg.Wait()
}
