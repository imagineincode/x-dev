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
	createSpace()
	for {
		userSelection, err := runMainPrompt()
		if err != nil {
			return fmt.Errorf("main prompt failed: %w", err)
		}

		switch userSelection {
		case 0:
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
				fmt.Println(Failed("[ERROR] "), "tweet exceeds maximum length of", maxPostLength, "characters.")
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
				postResponse, err = api.PostTweet(ctx, content, tokenResp.AccessToken)
				if err != nil {
					fmt.Printf(Failed("[ERROR] "), "error in post response: %v\n", err)
				} else {
					postID := postResponse.Data.ID
					fmt.Println("\U00002705 Post Successful! Post ID: ", postID)
				}
			case 1:
				fmt.Println("\U0000274C Post discarded.")
			default:
				continue
			}
		case 2:
			fmt.Println(Success("[OK] "), "exiting editor...")
			return nil
		default:
			return nil
		}
	} // end, return to main menu
}

func runMainPrompt() (int, error) {
	prompt := promptui.Select{
		Label: "Choose an action",
		Items: []string{"Start New Post", "Add Post to Latest Thead", "Exit"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}?",
			Active:   "-> {{ . | cyan }}",
			Inactive: "  {{ . | white }}",
			Selected: "\U0001F44D {{ . | green }}",
			Details:  "",
			Help:     "",
			FuncMap:  nil,
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return 2, fmt.Errorf("prompt failed: %w", err)
	}
	return idx, nil
}

func showPreviewPrompt(content string) (int, error) {
	wrappedContent := wrapText(content, 100)

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
			Active:   "-> {{ . | cyan }}",
			Inactive: "  {{ . | white }}",
			Selected: "\U0001F680 {{ . | green }}",
			Details:  "",
			Help:     "",
			FuncMap:  nil,
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
	paragraphs := strings.Split(text, "\n\n")
	wrappedParagraphs := []string{}

	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) == "" {
			wrappedParagraphs = append(wrappedParagraphs, "")
			continue
		}

		originalLines := strings.Split(paragraph, "\n")
		wrappedLines := []string{}

		for _, line := range originalLines {
			words := strings.Fields(line)

			if len(words) == 0 {
				wrappedLines = append(wrappedLines, "")
				continue
			}

			currentLine := words[0]

			for _, word := range words[1:] {
				if len(currentLine)+len(word)+1 > lineWidth {
					wrappedLines = append(wrappedLines, currentLine)
					currentLine = word
				} else {
					currentLine += " " + word
				}
			}

			wrappedLines = append(wrappedLines, currentLine)
		}

		wrappedParagraphs = append(wrappedParagraphs, strings.Join(wrappedLines, "\n"))
	}
	return strings.Join(wrappedParagraphs, "\n\n")
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

func createSpace() {
	fmt.Println()
	fmt.Println()
}
