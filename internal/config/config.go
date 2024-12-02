package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type EditorConfig struct {
	Editors   []string
	EnvEditor string
}

type Editor struct {
	Path string
	Name string
}

func NewEditorConfig() *EditorConfig {
	return &EditorConfig{
		Editors:   []string{"nvim", "vim", "nano", "emacs", "notepad"},
		EnvEditor: os.Getenv("EDITOR"),
	}
}

func (ec *EditorConfig) ChooseEditor() (*Editor, error) {
	if ec.EnvEditor != "" {
		if path, err := exec.LookPath(ec.EnvEditor); err == nil {
			return &Editor{Path: path, Name: ec.EnvEditor}, nil
		}
	}

	for _, editor := range ec.Editors {
		if path, err := exec.LookPath(editor); err == nil {
			return &Editor{Path: path, Name: editor}, nil
		}
	}

	return nil, errors.New("no suitable editor found")
}

func (e Editor) OpenEditor(ctx context.Context) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	tmpfile, err := os.CreateTemp("", fmt.Sprintf("posteditor_%s_*.txt", timestamp))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpfileName := tmpfile.Name()
	defer os.Remove(tmpfileName)
	defer tmpfile.Close()

	editorPath, err := exec.LookPath(e.Path)
	if err != nil {
		return "", fmt.Errorf("editor not found: %s", e.Path)
	}

	cmd := exec.CommandContext(ctx, editorPath, tmpfileName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run editor %s: %w", e.Name, err)
	}

	content, err := os.ReadFile(tmpfileName)
	if err != nil {
		return "", fmt.Errorf("failed to read temp file: %w", err)
	}

	return strings.TrimRight(string(content), "\n\r\t "), nil
}

func LoadConfig() (string, string, error) {
	clientID := os.Getenv("TWITTER_CLIENT_ID")
	clientSecret := os.Getenv("TWITTER_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", "", errors.New("missing required environment variables")
	}

	return clientID, clientSecret, nil
}
