package prompt

import (
	"context"
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
	showHeader()
	fmt.Println(Success("[OK] ") + fmt.Sprintf("authenticated as %v (@%v)", userResponse.Data.Name, userResponse.Data.Username))
	fmt.Println()
	fmt.Println()

	var lastPostID = &models.LastPostID{InReplyToPostID: ""}

	for {
		userSelection, err := runMainPrompt(lastPostID)
		if err != nil {
			return fmt.Errorf("main prompt failed: %w", err)
		}

		switch userSelection {
		case "Start New Post":
			content, err := editor.OpenEditor(ctx)
			if err != nil {
				fmt.Println(Failed("[ERROR] "), err)
				return nil
			}

			if strings.TrimSpace(content) == "" {
				fmt.Println(Warn("[WARN] "), "No content entered. Returning to main prompt.")
				continue
			}

			if len(content) > maxPostLength {
				fmt.Println(Failed("[ERROR] "), "post exceeds maximum length of", maxPostLength, "characters.")
				continue
			}

			previewResponse, err := showPreviewPrompt(content)
			if err != nil {
				fmt.Println(Failed("[ERROR] "), err)
				continue
			}

			switch previewResponse {
			case 0:
				var postResponse *models.PostResponse
				postResponse, err = api.SendPost(ctx, content, tokenResp.AccessToken)
				if err != nil {
					fmt.Printf(Failed("[ERROR] "), "error in post response: %v\n", err)
				} else {
					postID := postResponse.Data.ID
					fmt.Println("\U00002705 Post Successful! Post ID: ", postID)
					lastPostID.InReplyToPostID = postID
				}
			case 1:
				fmt.Println("\U0000274C Post discarded.")
			default:
				continue
			}
		case "Add Post to Latest Thread":
			threadContent, err := editor.OpenEditor(ctx)
			if err != nil {
				fmt.Println(Failed("[ERROR] "), err)
				return nil
			}

			if strings.TrimSpace(threadContent) == "" {
				fmt.Println(Warn("[WARN] "), "No content entered. Returning to main prompt.")
				continue
			}

			if len(threadContent) > maxPostLength {
				fmt.Println(Failed("[ERROR] "), "post exceeds maximum length of", maxPostLength, "characters.")
				continue
			}

			previewResponse, err := showPreviewPrompt(threadContent)
			if err != nil {
				fmt.Println(Failed("[ERROR] "), err)
				continue
			}

			switch previewResponse {
			case 0:
				threadPost := &models.ThreadPost{
					Text:  threadContent,
					Reply: lastPostID,
				}
				var postResponse *models.PostResponse
				postResponse, err = api.SendReplyPost(ctx, threadPost, tokenResp.AccessToken)
				if err != nil {
					fmt.Printf(Failed("[ERROR] "), "error in post response: %v\n", err)
				} else {
					postID := postResponse.Data.ID
					fmt.Println("\U00002705 Posting to Thread Successful! Post ID: ", postID)
				}
			case 1:
				fmt.Println("\U0000274C Post discarded.")
			default:
				continue
			}

		case "Exit":
			fmt.Println(Success("[OK] "), "exiting x-yapper...")
			return nil
		default:
			return nil
		}
	} // end, return to main menu
}

func runMainPrompt(lastPostID *models.LastPostID) (string, error) {
	mainPromptOptions := []string{"Start New Post", "Exit"}

	if lastPostID != nil && lastPostID.InReplyToPostID != "" {
		mainPromptOptions = append(mainPromptOptions[:1], append([]string{"Add Post to Latest Thread"}, mainPromptOptions[1:]...)...)
	}

	prompt := promptui.Select{
		Label: "Choose an action",
		Items: mainPromptOptions,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}?",
			Active:   "-> {{ . | cyan }}",
			Inactive: "  {{ . | white }}",
			Selected: "\U0001F44D {{ . | green }}",
		},
	}

	_, userSelection, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}
	return userSelection, nil
}

func showPreviewPrompt(content string) (int, error) {
	wrappedContent := wrapText(content, 60)

	fmt.Println("\nPost Preview:")
	fmt.Println("------------------------------------------------------------")
	fmt.Println(wrappedContent)
	fmt.Println("------------------------------------------------------------")
	fmt.Println("")

	prompt := promptui.Select{
		Label: "Choose an action",
		Items: []string{"Send Post", "Discard"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}?",
			Active:   "-> {{ . | cyan }}",
			Inactive: "  {{ . | white }}",
			Selected: "\U0001F680 {{ . | green }}",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return 1, fmt.Errorf("preview selection failed: %w", err)
	}

	return idx, nil
}

func wrapText(text string, lineWidth int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	var wrapped strings.Builder
	paragraphs := strings.Split(text, "\n\n")

	for i, paragraph := range paragraphs {
		if i > 0 {
			wrapped.WriteString("\n\n")
		}

		lines := breakParagraphIntoLines(paragraph, lineWidth)
		wrapped.WriteString(strings.Join(lines, "\n"))
	}

	return wrapped.String()
}

func breakParagraphIntoLines(paragraph string, lineWidth int) []string {
	words := strings.Fields(paragraph)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine+" "+word) > lineWidth {
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

func showHeader() {
	fmt.Println(`
    \ \  //
     \ \//
      \ \
     //\ \
    //  \ \
     yapper`)
	fmt.Println()
}
