package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"x-dev/internal/api"
	"x-dev/internal/config"
	"x-dev/internal/models"

	"github.com/manifoldco/promptui"
)

var (
	Success = promptui.Styler(promptui.FGGreen)
	Info    = promptui.Styler(promptui.FGCyan)
	Warn    = promptui.Styler(promptui.FGYellow)
	Failed  = promptui.Styler(promptui.FGRed)
)

func RunPrompts(ctx context.Context, tokenResp *models.TokenResponse, maxPostLength int, userResponse models.UserResponse) error {
	econfig := config.NewEditorConfig()
	editor, err := econfig.ChooseEditor()
	if err != nil {
		return fmt.Errorf("editor initialization failed: %w", err)
	}

	printHeader(userResponse)

	for {
		action, err := promptMainAction()
		if err != nil {
			return fmt.Errorf("main action prompt failed: %w", err)
		}

		if action == "Exit" {
			fmt.Println(Success("[OK] "), "exiting x-yapper...")
			return nil
		}

		initialPost, err := createPost(ctx, editor, maxPostLength)
		if err != nil {
			if errors.Is(err, errEmptyContent) {
				continue
			}
			return fmt.Errorf("initial post creation failed: %w", err)
		}

		firstTweetResp, err := api.PostTweet(ctx, initialPost, tokenResp.AccessToken)
		if err != nil {
			fmt.Println(Failed("[ERROR] "), fmt.Sprintf("error posting tweet: %v", err))
			continue
		}

		previousTweetID := firstTweetResp.Data.ID
		fmt.Printf("\U00002705 Post Successful! Tweet ID: %v\n", previousTweetID)

		if err := handleThreadCreation(ctx, editor, tokenResp, maxPostLength, previousTweetID); err != nil {
			return err
		}
	}
}

var errEmptyContent = errors.New("no content entered")

func validatePostContent(content string, maxLength int) error {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return errEmptyContent
	}

	if len(content) > maxLength {
		return fmt.Errorf("post exceeds maximum length of %d characters", maxLength)
	}

	return nil
}

func createPost(ctx context.Context, editor interface {
	OpenEditor(ctx context.Context) (string, error)
}, maxPostLength int,
) (string, error) {
	content, err := editor.OpenEditor(ctx)
	if err != nil {
		return "", fmt.Errorf("error opening editor: %w", err)
	}

	if err := validatePostContent(content, maxPostLength); err != nil {
		if errors.Is(err, errEmptyContent) {
			fmt.Println(Warn("[WARN] "), "No content entered. Returning to main prompt.")
		} else {
			fmt.Println(Failed("[ERROR] "), err)
		}
		return "", err
	}

	wrappedContent := wrapText(content, 60)
	shouldSend, err := showPreviewPrompt(wrappedContent)
	if err != nil {
		return "", fmt.Errorf("preview prompt error: %w", err)
	}
	if !shouldSend {
		return "", errEmptyContent
	}
	return content, nil
}

func handleThreadCreation(ctx context.Context, editor interface {
	OpenEditor(ctx context.Context) (string, error)
},
	tokenResp *models.TokenResponse, maxPostLength int, initialTweetID string,
) error {
	var threadTweets []string
	previousTweetID := initialTweetID

	for {
		threadAction, err := promptThreadAction()
		if err != nil {
			return fmt.Errorf("thread action prompt failed: %w", err)
		}

		switch threadAction {
		case "Add Post to Thread":
			threadContent, err := createPost(ctx, editor, maxPostLength)
			if err != nil {
				if errors.Is(err, errEmptyContent) {
					continue
				}
				return err
			}

			threadTweets = append(threadTweets, threadContent)

		case "Preview Thread":
			if len(threadTweets) == 0 {
				fmt.Println(Warn("[WARN] "), "No thread posts to preview.")
				continue
			}

			var formattedTweets []string
			for _, tweet := range threadTweets {
				formattedTweets = append(formattedTweets, wrapText(tweet, 60))
			}

			shouldSendThread, err := showPreviewPrompt(formattedTweets)
			if err != nil {
				return fmt.Errorf("thread preview error: %w", err)
			}
			if !shouldSendThread {
				continue
			}

			var threadTweetIDs []string
			for _, threadContent := range threadTweets {
				threadTweetReq := models.TweetRequest{
					Text: threadContent,
					Reply: &models.ReplyDetails{
						InReplyToTweetID: previousTweetID,
					},
				}

				threadResp, err := api.PostThreadTweet(ctx, threadTweetReq, tokenResp.AccessToken)
				if err != nil {
					fmt.Printf(Failed("[ERROR] "), "error posting thread tweet: %v\n", err)
					continue
				}

				threadTweetIDs = append(threadTweetIDs, threadResp.Data.ID)
				previousTweetID = threadResp.Data.ID
			}

			fmt.Printf("\U00002705 Thread Posts Successful! Tweet IDs: %v\n", threadTweetIDs)
			return nil

		case "Discard":
			return nil
		}
	}
}

func promptMainAction() (string, error) {
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

	_, result, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute prompt: %w", err)
	}

	return result, nil
}

func promptThreadAction() (string, error) {
	prompt := promptui.Select{
		Label: "Thread Options",
		Items: []string{"Add Post to Thread", "Preview Thread", "Discard"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}?",
			Active:   "-> {{ . | cyan }}",
			Inactive: "  {{ . | white }}",
			Selected: "\U0001F44D {{ . | green }}",
		},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute prompt: %w", err)
	}

	return result, nil
}

func printHeader(userResponse models.UserResponse) {
	fmt.Println(`
    \ \  //
     \ \//
      \ \
     //\ \
    //  \ \
     yapper`)
	fmt.Println()
	fmt.Println(Success("[OK] ") + fmt.Sprintf("authenticated as %v (@%v)", userResponse.Data.Name, userResponse.Data.Username))
	fmt.Println()
}

func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	lines := []string{}
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+len(word)+1 > width {
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}

	lines = append(lines, currentLine)
	return strings.Join(lines, "\n")
}

func showPreviewPrompt(content interface{}) (bool, error) {
	var displayText string

	switch v := content.(type) {
	case string:
		displayText = v
	case []string:
		displayText = strings.Join(v, "\n---\n")
	default:
		return false, errors.New("unsupported type: must be string or []string")
	}

	previewPrompt := promptui.Select{
		Label: fmt.Sprintf("Preview Tweet:\n%s\n\nSend this tweet?", displayText),
		Items: []string{"Send", "Discard"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "-> {{ . | cyan }}",
			Inactive: "  {{ . | white }}",
			Selected: "\U0001F44D {{ . | green }}",
		},
	}

	idx, _, err := previewPrompt.Run()
	if err != nil {
		return false, fmt.Errorf("failed to execute prompt: %w", err)
	}

	switch idx {
	case 0: // Send
		return true, nil
	default: // Discard
		return false, nil
	}
}
