package prompt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"x-dev/internal/api"
	"x-dev/internal/config"
	"x-dev/internal/models"

	"github.com/dustin/go-humanize"
	"github.com/eiannone/keyboard"
	"github.com/manifoldco/promptui"
	"golang.org/x/term"
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
				var rateLimit *models.RateLimitInfo
				postResponse, rateLimit, err = api.SendPost(ctx, content, tokenResp.AccessToken)
				if err != nil {
					fmt.Println(Failed("[ERROR] "), "error in post response: ", err)
				} else {
					postID := postResponse.Data.ID
					fmt.Println("\U00002705 Post Successful! Post ID: ", postID)
					lastPostID.InReplyToPostID = postID

				}
				if rateLimit != nil {
					rateLimitJSON, err := json.MarshalIndent(rateLimit, "", "  ")
					if err != nil {
						fmt.Println("Error marshaling rate limit info:", err)
					} else {
						fmt.Println(string(rateLimitJSON))
					}
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
				var rateLimit *models.RateLimitInfo
				postResponse, rateLimit, err = api.SendReplyPost(ctx, threadPost, tokenResp.AccessToken)
				if err != nil {
					fmt.Println(Failed("[ERROR] "), "error in post response: ", err)
				} else {
					postID := postResponse.Data.ID
					fmt.Println("\U00002705 Posting to Thread Successful! Post ID: ", postID)
				}

				if rateLimit != nil {
					rateLimitJSON, err := json.MarshalIndent(rateLimit, "", "  ")
					if err != nil {
						fmt.Println("Error marshaling rate limit info.", err)
					} else {
						fmt.Println(string(rateLimitJSON))
					}
				}

			case 1:
				fmt.Println("\U0000274C Post discarded.")

			default:
				continue
			}

		case "Show Timeline":
			var timelineResponse *models.TimelineResponse
			var rateLimit *models.RateLimitInfo
			timelineResponse, rateLimit, err = api.GetHomeTimeline(ctx, userResponse.Data.ID, tokenResp.AccessToken)
			if err != nil {
				fmt.Println(Failed("[ERROR] "), "error in timeline response.", err)
			} else {
				err = paginatePosts(timelineResponse)
				if err != nil {
					fmt.Println(Failed("[ERROR] "), "error in timeline response.", err)
				}
			}

			if rateLimit != nil {
				rateLimitJSON, err := json.MarshalIndent(rateLimit, "", "  ")
				if err != nil {
					fmt.Println("Error marshaling rate limit info.", err)
				} else {
					fmt.Println(string(rateLimitJSON))
				}
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
	mainPromptOptions := []string{"Start New Post", "Show Timeline", "Exit"}

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

func paginatePosts(timelineResponse *models.TimelineResponse) error {
	// Get terminal height
	_, terminalHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		terminalHeight = 24 // fallback to a standard terminal height
	}

	availableHeight := terminalHeight - 5
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Open keyboard for reading
	if err := keyboard.Open(); err != nil {
		return fmt.Errorf("could not open keyboard: %v", err)
	}
	defer keyboard.Close()

	// Prepare the posts content
	var postContents []string
	for _, tweet := range timelineResponse.PostData {
		var content strings.Builder

		// Post header
		content.WriteString(fmt.Sprintf("Post ID: %s Author ID: %s\n", tweet.ID, tweet.AuthorID))

		// Created time
		createdTime, err := time.Parse(time.RFC3339, tweet.CreatedAt)
		if err != nil {
			content.WriteString(fmt.Sprintf("Created: %s (error parsing time: %v)", tweet.CreatedAt, err))
		} else {
			content.WriteString(fmt.Sprintf("Created: (%s) %s",
				humanize.Time(createdTime),
				createdTime.Local().Format("Mon, Jan. 2, 2006 at 3:04 PM MST")))
		}

		// Post text
		wrappedPostContent := wrapText(tweet.Text, 60)
		content.WriteString("\n------------------------------------------------------------\n")
		content.WriteString(wrappedPostContent)

		// Attachments
		content.WriteString("Attachments:\n")
		for key, values := range tweet.Attachments.MediaKeys {
			attachmentLine := fmt.Sprintf("  %d:", key)
			for _, value := range values {
				attachmentLine += fmt.Sprintf(" %v", value)
			}
			content.WriteString(attachmentLine)
		}

		// Public Metrics
		if !reflect.DeepEqual(tweet.PublicMetrics, struct{}{}) {
			emojiMap := map[string]string{
				"like_count":       "♡",
				"retweet_count":    "🔁",
				"reply_count":      "💬",
				"bookmark_count":   "⛉",
				"impression_count": "👀",
			}

			metricOrder := []string{
				"reply_count",
				"retweet_count",
				"like_count",
				"impression_count",
				"bookmark_count",
			}

			content.WriteString("\n------------------------------------------------------------\n")
			var metricLine string
			metricValues := reflect.ValueOf(tweet.PublicMetrics)
			metricType := metricValues.Type()

			for _, metricName := range metricOrder {
				// Find the field by name
				for i := 0; i < metricValues.NumField(); i++ {
					if strings.ToLower(metricType.Field(i).Name) == strings.ReplaceAll(metricName, "_", "") {
						metricValue := metricValues.Field(i).Interface().(int)

						// Only add to metricLine if the value is non-zero
						if metricValue > 0 {
							if emoji, emojiExists := emojiMap[metricName]; emojiExists {
								metricLine += fmt.Sprintf("  %s %d    ", emoji, metricValue)
							} else {
								metricLine += fmt.Sprintf("  %s: %d ", metricName, metricValue)
							}
						}
						break
					}
				}
			}
			content.WriteString(metricLine)
			content.WriteString("\n------------------------------------------------------------\n\n")

			postContents = append(postContents, content.String())
		}
	}

	// Prepare pages with multiple tweets
	var pages []string
	var currentPage strings.Builder
	var currentPageLineCount int

	for _, postContent := range postContents {
		// Count lines in this tweet
		postLines := strings.Split(postContent, "\n")

		// Check if adding this tweet would exceed page height
		if currentPageLineCount+len(postLines) > availableHeight {
			// Current page is full, save it and start a new page
			pages = append(pages, currentPage.String())
			currentPage.Reset()
			currentPageLineCount = 0
		}

		// Add tweet to current page
		currentPage.WriteString(postContent)
		currentPageLineCount += len(postLines)
	}

	// Add last page if not empty
	if currentPage.Len() > 0 {
		pages = append(pages, currentPage.String())
	}

	// Navigate through pages
	for pageIndex := 0; pageIndex < len(pages); pageIndex++ {
		// Clear screen
		fmt.Print("\033[H\033[2J")

		// Print header
		fmt.Println("𝕏 Timeline - Page %d of %d "+Info("(Space: Next, Q: Quit)")+"\n\n",
			pageIndex+1, len(pages))

		// Print current page
		fmt.Print(pages[pageIndex])

		// Wait for user input
		char, key, err := keyboard.GetSingleKey()
		if err != nil {
			return fmt.Errorf("error reading keyboard: %v", err)
		}

		// Check for quit
		if key == keyboard.KeyCtrlC || char == 'q' || char == 'Q' {
			break
		}

		// If not space or last page, break
		if key != keyboard.KeySpace && pageIndex < len(pages)-1 {
			break
		}
	}

	return nil
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
