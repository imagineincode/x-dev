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
		fmt.Println("1. Create Post")
		fmt.Println("2. Exit")
		fmt.Print("Enter choice: ")

		var choice int
		_, err := fmt.Scan(&choice)
		if err != nil {
			fmt.Println("Invalid input. Please enter 1 or 2.")
			continue
		}

		switch choice {
		case 1:
			handleCreatePost()
		case 2:
			fmt.Println("Exiting. Goodbye!")
			os.Exit(0)
		default:
			fmt.Println("Invalid choice. Please enter 1 or 2.")
		}
	}
}

func handleCreatePost() {
	var editor string
	if isEditorAvailable("nvim") {
		editor = "nvim"
	} else if isEditorAvailable("vim") {
		editor = "vim"
	} else {
		editor = ""
	}

	var content string
	prompt := &survey.Editor{
		Message:       "Compose your post:",
		FileName:      "*.txt",
		Editor:        editor,
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
