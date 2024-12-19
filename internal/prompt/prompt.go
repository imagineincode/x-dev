package prompt

import (
	"context"
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
	fmt.Printf("Authenticated as %v (@%v)", userResponse.Data.Name, userResponse.Data.Username)
	fmt.Println()
	fmt.Println()

	latestPost := &models.LatestPost{
		PostID: "",
		Text:   "",
	}

	for {
		userSelection, err := runMainPrompt(latestPost)
		if err != nil {
			return fmt.Errorf("main prompt failed: %w", err)
		}

		switch userSelection {
		case "Start new post":
			content, err := editor.OpenEditor(ctx)
			if err != nil {
				fmt.Println(Failed("[ERROR] "), err)
				continue
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
					fmt.Println(Failed("[ERROR] "), err)
				} else {
					postID := postResponse.Data.ID
					fmt.Println("\U00002705 Post Successful! Post ID: ", postID)
					latestPost.PostID = postID
					latestPost.Text = content
				}
				rateLimitStatus := rateLimitStatus(rateLimit)

				if rateLimitStatus != "" {
					fmt.Println(rateLimitStatus)
				}

			case 1:
				fmt.Println("\U0000274C Post discarded.")

			default:
				fmt.Println("\U0000274C Post discarded.")

			}
		case "Add post to latest thread":
			content, err := editor.OpenEditor(ctx)
			if err != nil {
				fmt.Println(Failed("[ERROR] "), err)
			}

			if strings.TrimSpace(content) == "" {
				fmt.Println(Warn("[WARN] "), "No content entered. Returning to main prompt.")
			}

			if len(content) > maxPostLength {
				fmt.Println(Failed("[ERROR] "), "post exceeds maximum length of", maxPostLength, "characters.")
			}

			previewResponse, err := showPreviewPrompt(content)
			if err != nil {
				fmt.Println(Failed("[ERROR] "), err)
			}

			switch previewResponse {
			case 0:
				replyToPostID := &models.Reply{ReplyID: latestPost.PostID}
				threadPost := &models.ReplyPost{
					Text:  content,
					Reply: replyToPostID,
				}

				var postResponse *models.PostResponse
				var rateLimit *models.RateLimitInfo
				postResponse, rateLimit, err = api.SendReplyPost(ctx, threadPost, tokenResp.AccessToken)
				if err != nil {
					fmt.Println(Failed("[ERROR] "), err)
				} else {
					postID := postResponse.Data.ID
					fmt.Println("\U00002705 Posting to Thread Successful! Post ID: ", postID)
					latestPost.PostID = postID
					latestPost.Text = content
				}

				rateLimitStatus := rateLimitStatus(rateLimit)

				if rateLimitStatus != "" {
					fmt.Println(rateLimitStatus)
				}

			case 1:
				fmt.Println("\U0000274C Post discarded.")

			default:
				fmt.Println(Warn("[WARN]"), "unable to determine selection, returning to main menu.")

			}

		case "Show timeline":
			var timelineResponse *models.TimelineResponse
			var rateLimit *models.RateLimitInfo
			timelineResponse, rateLimit, err = api.GetHomeTimeline(ctx, userResponse.Data.ID, tokenResp.AccessToken)
			if err != nil {
				fmt.Println(Failed("[ERROR]"), err)
			} else {
				err = paginatePosts(timelineResponse)
				if err != nil {
					fmt.Println(Failed("[ERROR] "), err)
				}
			}

			rateLimitStatus := rateLimitStatus(rateLimit)

			if rateLimitStatus != "" {
				fmt.Println(rateLimitStatus)
			}

		case "Exit":
			fmt.Println(Success("[OK] "), "exiting x-yapper...")
			return nil

		default:
			fmt.Println(Warn("[WARN]"), "unable to determine selection, returning to main menu.")
		}
	} // end, return to main menu
}

func runMainPrompt(latestPost *models.LatestPost) (string, error) {
	type PromptOption struct {
		Name    string
		Details string
	}

	mainPromptOptions := []PromptOption{
		{
			Name:    "Start new post",
			Details: "  Create new post",
		},
		{
			Name: "Show timeline",
			Details: fmt.Sprintf(
				"  View recent posts and interactions\n  "+
					"%s x-developer free tier has a limit of 1 request every 15 minutes for the timeline endpoint.\n  "+
					"%s x-developer free tier has a limit of 100 post pulls a month for the timeline endpoint.",
				Warn("[WARN]"), Warn("[WARN]")),
		},
		{
			Name:    "Exit",
			Details: "  Close the application",
		},
	}

	if latestPost.Text != "" {
		wrappedText := wrapText(latestPost.Text, 60)
		mainPromptOptions = append(mainPromptOptions[:1], append([]PromptOption{
			{
				Name: "Add post to latest thread",
				Details: fmt.Sprintf(
					"  Reply to the most recently created thread\n"+
						"  Post ID: %s\n"+
						"------------------------------------------------------------\n"+
						"%s\n"+
						"------------------------------------------------------------",
					latestPost.PostID, wrappedText),
			},
		}, mainPromptOptions[1:]...)...)
	}

	prompt := promptui.Select{
		Label: "Choose an action",
		Items: mainPromptOptions,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ .Name }}?",
			Active:   "-> {{ .Name | cyan }}",
			Inactive: "   {{ .Name | white }}",
			Selected: "\U0001F44D {{ .Name | green }}",
			Details:  "\n{{ .Details | faint }}",
		},
	}

	selectedIndex, _, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("prompt failed: %w", err)
	}

	return mainPromptOptions[selectedIndex].Name, nil
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
	if err := keyboard.Open(); err != nil {
		return fmt.Errorf("could not open keyboard: %w", err)
	}
	defer keyboard.Close()

	userMap := mapUsersFromTimelineResponse(timelineResponse.Includes.Users)

	var postContents []string
	for _, tweet := range timelineResponse.Data {
		postContents = append(postContents, formatTweetContent(tweet, userMap))
	}

	availableHeight := calculateAvailablePageHeight()
	pages := paginateTweetContents(postContents, availableHeight)

	return displayPages(pages)
}

func mapUsersFromTimelineResponse(users []models.User) map[string]*models.User {
	userMap := make(map[string]*models.User)
	if len(users) == 0 {
		return userMap
	}

	for i := range users {
		user := users[i]
		userMap[user.ID] = &user
	}
	return userMap
}

func displayPages(pages []string) error {
	for pageIndex := range pages {
		fmt.Print("\033[H\033[2J")

		fmt.Printf("𝕏 Timeline - Page %d of %d (Space: Next, Q: Quit)\n\n", pageIndex+1, len(pages))

		fmt.Print(pages[pageIndex])

		char, key, err := keyboard.GetSingleKey()
		if err != nil {
			return fmt.Errorf("error reading keyboard: %w", err)
		}

		if key == keyboard.KeyCtrlC || char == 'q' || char == 'Q' {
			break
		}

		if key != keyboard.KeySpace && pageIndex < len(pages)-1 {
			break
		}
	}
	return nil
}

func formatTweetContent(tweet models.Tweet, userMap map[string]*models.User) string {
	var content strings.Builder

	content.WriteString(formatAuthorInfo(tweet, userMap))

	content.WriteString(formatReferenceTweetType(tweet))

	content.WriteString("------------------------------------------------------------\n")
	content.WriteString(wrapText(tweet.Text, 60))

	content.WriteString(formatAttachments(tweet))

	content.WriteString(formatURLs(tweet))

	content.WriteString(formatPublicMetrics(tweet))

	return content.String()
}

func formatAuthorInfo(tweet models.Tweet, userMap map[string]*models.User) string {
	var authorInfo strings.Builder

	author, found := userMap[tweet.AuthorID]
	var createdTime string

	createdTimeRaw, err := time.Parse(time.RFC3339, tweet.CreatedAt)
	if err != nil {
		createdTime = fmt.Sprintf("%s (error parsing time: %v)", tweet.CreatedAt, err)
	} else {
		createdTime = humanize.Time(createdTimeRaw)
	}

	if found {
		authorInfo.WriteString(fmt.Sprintf("%s (@%s) | %s\n", author.Name, author.Username, createdTime))
	} else {
		authorInfo.WriteString(fmt.Sprintf("Author ID: %s | Post ID: %s\n", tweet.AuthorID, tweet.ID))
	}

	return authorInfo.String()
}

func formatReferenceTweetType(tweet models.Tweet) string {
	var refTweetType string
	if len(tweet.ReferencedTweets) > 0 {
		refTweetType = tweet.ReferencedTweets[0].Type
	} else {
		refTweetType = "new post"
	}

	return fmt.Sprintf("Type: %s | PostID: %s\n", refTweetType, tweet.ID)
}

func formatAttachments(tweet models.Tweet) string {
	if tweet.Attachments == nil || len(tweet.Attachments.MediaKeys) == 0 {
		return ""
	}

	var attachments strings.Builder
	attachments.WriteString("\nAttachments:\n")
	for key, value := range tweet.Attachments.MediaKeys {
		attachmentLine := fmt.Sprintf("  Media key %v: %v\n", key, value)
		attachments.WriteString(attachmentLine)
	}

	return attachments.String()
}

func formatURLs(tweet models.Tweet) string {
	if tweet.Entities == nil || len(tweet.Entities.URLs) == 0 {
		return ""
	}

	var urls strings.Builder
	urls.WriteString("\nURLs :\n")
	for _, url := range tweet.Entities.URLs {
		urlLine := fmt.Sprintf("  URL: %s\n  Expanded URL: %s\n", url.URL, url.ExpandedURL)
		urls.WriteString(urlLine)
	}

	return urls.String()
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

func formatPublicMetrics(tweet models.Tweet) string {
	if reflect.DeepEqual(tweet.PublicMetrics, struct{}{}) {
		return ""
	}

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

	var content strings.Builder
	content.WriteString("\n------------------------------------------------------------\n")

	var metricLine string
	metricValues := reflect.ValueOf(tweet.PublicMetrics)
	metricType := metricValues.Type()

	for _, metricName := range metricOrder {
		for i := range metricValues.NumField() {
			if strings.ToLower(metricType.Field(i).Name) == strings.ReplaceAll(metricName, "_", "") {
				metricValue, ok := metricValues.Field(i).Interface().(int)
				if !ok {
					continue
				}

				if emoji, emojiExists := emojiMap[metricName]; emojiExists {
					metricLine += fmt.Sprintf("  %s %d    ", emoji, metricValue)
				} else {
					metricLine += fmt.Sprintf("  %s: %d ", metricName, metricValue)
				}
				break
			}
		}
	}

	content.WriteString(metricLine)
	content.WriteString("\n------------------------------------------------------------\n\n")

	return content.String()
}

func calculateAvailablePageHeight() int {
	_, terminalHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		terminalHeight = 24
	}

	availableHeight := terminalHeight - 5
	if availableHeight < 10 {
		availableHeight = 10
	}

	return availableHeight
}

func paginateTweetContents(postContents []string, availableHeight int) []string {
	var pages []string
	var currentPage strings.Builder
	var currentPageLineCount int

	for _, postContent := range postContents {
		postLines := strings.Split(postContent, "\n")

		if currentPageLineCount+len(postLines) > availableHeight {
			pages = append(pages, currentPage.String())
			currentPage.Reset()
			currentPageLineCount = 0
		}

		currentPage.WriteString(postContent)
		currentPageLineCount += len(postLines)
	}

	if currentPage.Len() > 0 {
		pages = append(pages, currentPage.String())
	}

	return pages
}

func rateLimitStatus(rateLimit *models.RateLimitInfo) string {
	resetTime := rateLimit.ResetTime.Format("Jan 2 at 3:04 PM")

	if rateLimit.Remaining < 5 && rateLimit.Remaining != 0 {
		return fmt.Sprintf("%s %d requests remaining before rate limit reached for this endpoint. Resets on %s.",
			Warn("[WARN]"),
			rateLimit.Remaining,
			resetTime)
	} else if rateLimit.Remaining == 0 {
		return fmt.Sprintf("%s Rate limit exceeded for this endpoint. Resets on %s.",
			Warn("[WARN]"),
			resetTime)
	}
	return ""
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
