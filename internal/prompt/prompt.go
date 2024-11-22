package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

type EditorConfig struct {
	editors   []string
	envEditor string
}

type Editor struct {
	path string
	name string
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

	return nil, fmt.Errorf("no suitable editor found")
}

func (e *Editor) openEditor() (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	tmpfile, err := os.CreateTemp("", fmt.Sprintf("posteditor_%s_*.txt", timestamp))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpfileName := tmpfile.Name()
	defer os.Remove(tmpfileName)
	tmpfile.Close()

	cmd := exec.Command(e.path, tmpfileName)
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

func showPreviewPrompt(content string) (bool, error) {
	fmt.Println("\nPost Preview")
	fmt.Println("------------------------------------------")
	fmt.Println(content)
	fmt.Println("------------------------------------------")

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
		return false, fmt.Errorf("preview selection failed: %w", err)
	}

	return idx == 0, nil // index 0 is "Send Post"
}

func runPrompts() error {
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
		prompt := promptui.Prompt{
			Label:     "Press Enter to start new Post",
			AllowEdit: true,
		}

		result, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		if strings.ToLower(strings.TrimSpace(result)) == "exit" {
			fmt.Println("Exiting editor...")
			return nil
		}

		content, err := editor.openEditor()
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		if strings.TrimSpace(content) == "" {
			fmt.Println("No content entered. Returning to main prompt.")
			continue
		}

		shouldSend, err := showPreviewPrompt(content)
		if err != nil {
			log.Printf("Preview failed: %v", err)
			continue
		}

		if shouldSend {
			fmt.Println("\U00002705 Successfully sent post!")
		} else {
			fmt.Println("Post discarded.")
		}
	}
}

func main() {
	if err := runPrompts(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
