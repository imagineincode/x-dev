package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"x-dev/internal/api"
	"x-dev/internal/config"
	"x-dev/internal/models"
	"x-dev/internal/prompt"
	"x-dev/internal/xauth"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println(prompt.Info("[INFO] "), "getting environment variables")

	clientID, clientSecret, err := config.LoadClientConfig()
	if err != nil {
		fmt.Println(prompt.Failed("[ERROR]"), "failed to load configuration:", err)

		cancel()
		os.Exit(1)
	}

	fmt.Println(prompt.Success("[OK] "), "environment variables set")
	fmt.Println(prompt.Success("[OK] "), "starting authentication service")

	codeVerifier := xauth.GenerateCodeVerifier()
	codeChallenge := xauth.GenerateCodeChallenge(codeVerifier)
	authState := xauth.GenerateRandomString(32)

	fmt.Println(prompt.Success("[OK] "), "starting callback server")

	var wGroup sync.WaitGroup

	api.StartCallbackServer(ctx, &wGroup, authState)

	fmt.Println(prompt.Success("[OK] "), "creating unique authentication URL")

	u, err := url.Parse(models.AuthEndpoint)
	if err != nil {
		log.Fatalf("failed to parse auth endpoint: %v", err)
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", fmt.Sprintf("http://localhost:%s%s", models.CallbackPort, models.CallbackEndpoint))
	q.Set("scope", models.Scopes)
	q.Set("state", authState)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")

	u.RawQuery = q.Encode()
	authURL := u.String()

	fmt.Printf("\nPlease open this URL in your browser to authorize the application:\n\n%s\n\n", authURL)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case code := <-models.AuthTokenChan:
		tokenResponse, err := xauth.ExchangeCodeForToken(ctx, clientID, clientSecret, codeVerifier, code)
		if err != nil {
			log.Fatalf("error exchanging code for token: %v", err)
		}

		fmt.Println(prompt.Success("[OK] "), "authentication successful, starting x-yapper prompt")

		var maxPostLength int

		maxPostLength, userResponse, err := api.CheckAccountType(ctx, tokenResponse.AccessToken)
		if err != nil {
			maxPostLength = 280

			fmt.Printf("could not determine tweet length limit: %v", err)
			fmt.Println(prompt.Info("[INFO] "), "standard post length requirements set")
		}

		if err := prompt.RunPrompts(ctx, tokenResponse, maxPostLength, userResponse); err != nil {
			log.Fatalf("error: %v", err)
		}

	case <-sigChan:
		fmt.Println(prompt.Warn("[WARN] "), "received interrupt, shutting down...")

		cancel()
	}

	wGroup.Wait()
}
