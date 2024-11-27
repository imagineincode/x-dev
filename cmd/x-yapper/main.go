package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

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

type UserResponse struct {
	Data struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Verified bool   `json:"verified"`
	} `json:"data"`
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
	maxPostLength  int
	authTokenChan  = make(chan string)
	callbackServer *http.Server
	scopes         = "tweet.read tweet.write users.read offline.access"
	success        = promptui.Styler(promptui.FGGreen)
	info           = promptui.Styler(promptui.FGCyan)
	warn           = promptui.Styler(promptui.FGYellow)
	failed         = promptui.Styler(promptui.FGRed)
)

const (
	authEndpoint       = "https://twitter.com/i/oauth2/authorize"
	tknEndpoint        = "https://api.twitter.com/2/oauth2/token"
	callbackPort       = "8080"
	callbackEndpoint   = "/callback"
	codeVerifierLength = 128
)

func loadConfig() (string, string, error) {
	clientID := os.Getenv("TWITTER_CLIENT_ID")
	clientSecret := os.Getenv("TWITTER_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", "", errors.New("missing required environment variables")
	}

	return clientID, clientSecret, nil
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)

	for i := range b {
		randomByte := make([]byte, 1)

		if _, err := rand.Read(randomByte); err != nil {
			log.Fatalf("failed to generate random byte: %v", err)
		}

		b[i] = charset[int(randomByte[0])%len(charset)]
	}

	return string(b)
}

func generateCodeVerifier() string {
	return generateRandomString(codeVerifierLength)
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])

	return strings.ReplaceAll(strings.ReplaceAll(challenge, "+", "-"), "/", "_")
}

func startCallbackServer(ctx context.Context, wGroup *sync.WaitGroup) {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackEndpoint, func(w http.ResponseWriter, r *http.Request) {
		handleCallback(ctx, w, r)
	})

	callbackServer = &http.Server{
		Addr:              ":" + callbackPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          log.New(os.Stderr, "http: ", log.LstdFlags),
	}

	wGroup.Add(1)

	go func() {
		defer wGroup.Done()

		if err := callbackServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := callbackServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()
}

func handleCallback(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	state := queryParams.Get("state")
	code := queryParams.Get("code")

	if state != authState {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)

		return
	}

	_, err := w.Write([]byte("Authorization successful! You can close this window."))
	if err != nil {
		log.Printf("error writing response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	authTokenChan <- code

	go func(ctx context.Context) {
		if err := callbackServer.Shutdown(ctx); err != nil {
			log.Printf("error shutting down server: %v", err)
		}
	}(ctx)
}

func exchangeCodeForToken(ctx context.Context, clientID, clientSecret, code string) (*TokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", fmt.Sprintf("http://localhost:%s%s", callbackPort, callbackEndpoint))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tknEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creating token request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:           100,
			MaxIdleConnsPerHost:    10,
			IdleConnTimeout:        90 * time.Second,
			Proxy:                  nil,
			OnProxyConnectResponse: nil,
			DialContext:            nil,
			Dial:                   nil,
			DialTLSContext:         nil,
			DialTLS:                nil,
			TLSClientConfig:        nil,
			TLSHandshakeTimeout:    0,
			DisableKeepAlives:      false,
			DisableCompression:     false,
			MaxConnsPerHost:        0,
			ResponseHeaderTimeout:  0,
			ExpectContinueTimeout:  0,
			TLSNextProto:           nil,
			ProxyConnectHeader:     nil,
			GetProxyConnectHeader:  nil,
			MaxResponseHeaderBytes: 0,
			WriteBufferSize:        0,
			ReadBufferSize:         0,
			ForceAttemptHTTP2:      false,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
		Jar: nil,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending token request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting token, status code: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("error decoding token response: %w", err)
	}

	return &tokenResp, nil
}

func checkAccountType(ctx context.Context, accessToken string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	userURL := "https://api.twitter.com/2/users/me?user.fields=verified"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return 0, fmt.Errorf("error creating user request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:           100,
			MaxIdleConnsPerHost:    10,
			IdleConnTimeout:        90 * time.Second,
			Proxy:                  nil,
			OnProxyConnectResponse: nil,
			DialContext:            nil,
			Dial:                   nil,
			DialTLSContext:         nil,
			DialTLS:                nil,
			TLSClientConfig:        nil,
			TLSHandshakeTimeout:    0,
			DisableKeepAlives:      false,
			DisableCompression:     false,
			MaxConnsPerHost:        0,
			ResponseHeaderTimeout:  0,
			ExpectContinueTimeout:  0,
			TLSNextProto:           nil,
			ProxyConnectHeader:     nil,
			GetProxyConnectHeader:  nil,
			MaxResponseHeaderBytes: 0,
			WriteBufferSize:        0,
			ReadBufferSize:         0,
			ForceAttemptHTTP2:      false,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
		Jar: nil,
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error sending user request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)

		return 0, fmt.Errorf("error fetching user info, status code: %d, response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var userResp UserResponse

	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return 0, fmt.Errorf("error decoding user response: %w", err)
	}

	if userResp.Data.Verified {
		maxPostLength = 4000

		fmt.Println(success("[OK] "), "verified account detected. extended post length enabled.")
	} else {
		maxPostLength = 280

		fmt.Println(success("[OK] "), "basic account detected. standard post length requirements set.")
	}

	return maxPostLength, nil
}

func postTweet(ctx context.Context, text string, accessToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tweetURL := "https://api.twitter.com/2/tweets"
	tweetReq := TweetRequest{Text: text}

	jsonData, err := json.Marshal(tweetReq)
	if err != nil {
		return fmt.Errorf("error marshaling tweet request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tweetURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:           100,
			MaxIdleConnsPerHost:    10,
			IdleConnTimeout:        90 * time.Second,
			Proxy:                  nil,
			OnProxyConnectResponse: nil,
			DialContext:            nil,
			Dial:                   nil,
			DialTLSContext:         nil,
			DialTLS:                nil,
			TLSClientConfig:        nil,
			TLSHandshakeTimeout:    0,
			DisableKeepAlives:      false,
			DisableCompression:     false,
			MaxConnsPerHost:        0,
			ResponseHeaderTimeout:  0,
			ExpectContinueTimeout:  0,
			TLSNextProto:           nil,
			ProxyConnectHeader:     nil,
			GetProxyConnectHeader:  nil,
			MaxResponseHeaderBytes: 0,
			WriteBufferSize:        0,
			ReadBufferSize:         0,
			ForceAttemptHTTP2:      false,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
		Jar: nil,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error posting tweet, status code: %d, response: %s",
			resp.StatusCode, string(body))
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

	return nil, errors.New("no suitable editor found")
}

func (e *Editor) openEditor(ctx context.Context) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	tmpfile, err := os.CreateTemp("", fmt.Sprintf("posteditor_%s_*.txt", timestamp))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpfileName := tmpfile.Name()
	defer os.Remove(tmpfileName)
	defer tmpfile.Close()

	editorPath, err := exec.LookPath(e.path)
	if err != nil {
		return "", fmt.Errorf("editor not found: %s", e.path)
	}

	cmd := exec.CommandContext(ctx, editorPath, tmpfileName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run editor %s: %w", e.name, err)
	}

	content, err := os.ReadFile(tmpfileName)
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %w", err)
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
			Details:  "",
			Help:     "",
			FuncMap:  nil,
		},
		Size:              0,
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
		CursorPos:         0,
		IsVimMode:         false,
		HideHelp:          false,
		HideSelected:      false,
		Keys:              nil,
		Searcher:          nil,
		StartInSearchMode: false,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return false, fmt.Errorf("preview selection failed: %w", err)
	}

	return idx == 0, nil // index 0 is "Send Post"
}

func runPrompts(ctx context.Context, tokenResp *TokenResponse) error {
	config := newEditorConfig()
	editor, err := config.chooseEditor()
	if err != nil {
		return fmt.Errorf("editor initialization failed: %w", err)
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
				Details:  "",
				Help:     "",
				FuncMap:  nil,
			},
			Size:              0,
			Stdin:             os.Stdin,
			Stdout:            os.Stdout,
			CursorPos:         0,
			IsVimMode:         false,
			HideHelp:          false,
			HideSelected:      false,
			Keys:              nil,
			Searcher:          nil,
			StartInSearchMode: false,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		if idx == 1 { // Exit option
			fmt.Println(success("[OK] "), "exiting editor...")

			return nil
		}

		content, err := editor.openEditor(ctx)
		if err != nil {
			log.Printf("error: %v", err)

			return nil
		}

		if strings.TrimSpace(content) == "" {
			fmt.Println(warn("[WARN] "), "No content entered. Returning to main prompt.")

			return nil
		}

		if len(content) > maxPostLength {
			return fmt.Errorf("tweet exceeds maximum length of %d characters", maxPostLength)
		}

		shouldSend, err := showPreviewPrompt(content)
		if err != nil {
			log.Printf("preview failed: %v", err)

			continue
		}

		if shouldSend {
			err = postTweet(ctx, content, tokenResp.AccessToken)
			if err != nil {
				fmt.Printf(failed("[ERROR] "), "error posting tweet: %v\n", err)
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

	fmt.Println(info("[INFO] "), "getting environment variables.")

	clientID, clientSecret, err := loadConfig()
	if err != nil {
		fmt.Printf("failed to load configuration: %v", err)
		fmt.Println(info("[INFO] "), "configure environment variables.")

		cancel()
		os.Exit(1)
	}

	fmt.Println(success("[OK] "), "environment variables set.")
	fmt.Println(success("[OK] "), "starting authentication service.")

	codeVerifier = generateCodeVerifier()
	codeChallenge = generateCodeChallenge(codeVerifier)
	authState = generateRandomString(32)

	fmt.Println(success("[OK] "), "starting callback server.")

	var wGroup sync.WaitGroup

	startCallbackServer(ctx, &wGroup)

	fmt.Println(success("[OK] "), "creating unique authentication URL.")

	u, err := url.Parse(authEndpoint)
	if err != nil {
		log.Fatalf("failed to parse auth endpoint: %v", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", fmt.Sprintf("http://localhost:%s%s", callbackPort, callbackEndpoint))
	q.Set("scope", scopes)
	q.Set("state", authState)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")

	u.RawQuery = q.Encode()
	authURL := u.String()

	fmt.Printf("\nPlease open this URL in your browser to authorize the application:\n%s\n\n", authURL)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case code := <-authTokenChan:
		tokenResponse, err := exchangeCodeForToken(ctx, clientID, clientSecret, code)
		if err != nil {
			log.Fatalf("error exchanging code for token: %v", err)
		}

		fmt.Println(success("[OK] "), "authentication successful, starting x-yapper...")

		maxPostLength, err = checkAccountType(ctx, tokenResponse.AccessToken)
		if err != nil {
			maxPostLength = 280

			log.Printf("could not determine tweet length limit: %v", err)
			fmt.Println(info("[INFO] "), "standard post length requirements set.")
		}

		if err := runPrompts(ctx, tokenResponse); err != nil {
			log.Fatalf("error: %v", err)
		}

	case <-sigChan:
		fmt.Println(warn("[WARN] "), "received interrupt, shutting down...")

		cancel()
	}

	wGroup.Wait()
}
