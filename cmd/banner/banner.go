package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/AlecAivazis/survey/v2"
)

func main() {
	fmt.Println(`      
  \\ / / 
   \  /  
   / / 
  / /\\  
 / /  \\
  yapper`)
	fmt.Println("")
	for {
		// Display menu options
		fmt.Println("Select an option:")
		fmt.Println("1. Create Post")
		fmt.Println("2. Exit")
		fmt.Print("Enter choice: ")

		// Capture user input
		var choice int
		_, err := fmt.Scan(&choice)
		if err != nil {
			fmt.Println("Invalid input. Please enter 1 or 2.")
			continue
		}

		// Handle choices using a switch statement
		switch choice {
		case 1:
			// Open the editor for "Create Post"
			handleCreatePost()
		case 2:
			// Exit the program
			fmt.Println("Exiting. Goodbye!")
			os.Exit(0)
		default:
			fmt.Println("Invalid choice. Please enter 1 or 2.")
		}
	}
}

// handleCreatePost opens the editor for the user to create a post
func handleCreatePost() {
	// Check for the availability of editors
	var editor string
	if isEditorAvailable("nvim") {
		editor = "nvim"
	} else if isEditorAvailable("vim") {
		editor = "vim"
	} else {
		editor = "" // Use the default editor
	}

	// Prompt the user to input content using the selected editor
	var content string
	prompt := &survey.Editor{
		Message:       "Compose your post:",
		FileName:      "*.txt", // Temporary file with .txt extension
		Editor:        editor,  // Specify the editor
		HideDefault:   false,
		AppendDefault: true,
	}
	err := survey.AskOne(prompt, &content)
	if err != nil {
		log.Fatalf("Error during input: %v", err)
	}

	fmt.Println("Post preview:")
	fmt.Println(content)
}

func isEditorAvailable(editor string) bool {
	_, err := exec.LookPath(editor)
	return err == nil
}
