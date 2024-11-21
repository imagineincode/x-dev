package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/manifoldco/promptui"
)

func chooseEditor() string {
	editors := []string{"nvim", "vim", "nano", "emacs", "notepad"}
	for _, editor := range editors {
		path, err := exec.LookPath(editor)
		if err == nil {
			return path
		}
	}
	return "notepad" // Fallback
}

func openEditor(editor string) (string, error) {
	tmpfile, err := os.CreateTemp("", "posteditor*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func main() {
	for {
		// Start prompt
		prompt := promptui.Prompt{
			Label: "Press Enter to start post editor (or type 'exit' to quit)",
		}

		result, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		if strings.ToLower(result) == "exit" {
			break
		}

		// Choose and open editor
		editor := chooseEditor()
		content, err := openEditor(editor)
		if err != nil {
			log.Printf("Error opening editor: %v", err)
			continue
		}

		// Confirmation prompt
		confirmPrompt := promptui.Select{
			Label: fmt.Sprintf("Content:\n%s\nConfirm?", content),
			Items: []string{"Yes", "No"},
		}

		_, result, err = confirmPrompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			continue
		}

		// Placeholder for future function call based on confirmation
		// You can add specific actions here when needed
		if result == "Yes" {
			fmt.Println("Content confirmed. Placeholder for further action.")
		} else {
			fmt.Println("Content discarded.")
		}
	}
}
