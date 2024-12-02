package prompt

import (
	"context"
	"fmt"
	"log"
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
	fmt.Println()
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
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		if idx == 1 { // Exit option
			fmt.Println(Success("[OK] "), "exiting editor...")

			return nil
		}

		content, err := editor.OpenEditor(ctx)
		if err != nil {
			log.Printf("error: %v", err)

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

		shouldSend, err := showPreviewPrompt(content)
		if err != nil {
			log.Printf("preview failed: %v", err)

			continue
		}

		if shouldSend {
			err = api.PostTweet(ctx, content, tokenResp.AccessToken)
			if err != nil {
				fmt.Printf(Failed("[ERROR] "), "error posting tweet: %v\n", err)
			} else {
				fmt.Println("\U00002705 Post Successful!")
			}

		} else {
			fmt.Println("\U0000274C Post discarded.")
		}
	}
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
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return false, fmt.Errorf("preview selection failed: %w", err)
	}

	return idx == 0, nil // index 0 is "Send Post"
}
