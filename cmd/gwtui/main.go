package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanmayer/citadel/internal/app"
	"github.com/seanmayer/citadel/internal/commands"
	"github.com/seanmayer/citadel/internal/config"
	"github.com/seanmayer/citadel/internal/editor"
	"github.com/seanmayer/citadel/internal/git"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "citadel: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("detect working directory: %w", err)
	}

	gitService := git.NewService(nil)
	repoRoot, err := gitService.DetectRepoRoot(context.Background(), cwd)
	if err != nil {
		return fmt.Errorf("citadel requires running inside a Git repository: %w", err)
	}

	commandService := commands.NewService(gitService)
	editorService := editor.NewService(cfg.Editor, nil)
	model := app.New(cfg, gitService, commandService, editorService, repoRoot)

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}

	return nil
}
