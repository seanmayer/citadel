package ui

import (
	"strings"
	"testing"

	"github.com/seanmayer/citadel/internal/config"
	"github.com/seanmayer/citadel/internal/git"
)

func TestRenderOutputShowsContinueCallToAction(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(config.Defaults())
	view := renderer.Render(ViewModel{
		Mode:        ModeOutput,
		Width:       100,
		Height:      30,
		LastCommand: "git status",
		OutputView:  "clean",
	})

	if !strings.Contains(view, "SPACE TO CONTINUE") {
		t.Fatalf("rendered output missing continue title:\n%s", view)
	}
	if !strings.Contains(view, "Return to the worktree list") {
		t.Fatalf("rendered output missing continue hint:\n%s", view)
	}
}

func TestRenderDetailsShowsWorktreeActionList(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(config.Defaults())
	view := renderer.Render(ViewModel{
		Mode:     ModeList,
		Width:    120,
		Height:   30,
		RepoRoot: "/repo",
		Worktrees: []git.Worktree{{
			Path:   "/repo/feature",
			Branch: "feature/demo",
		}},
	})

	for _, snippet := range []string{
		"Actions",
		"open terminal command mode",
		"git add .",
		"commit files with a message",
	} {
		if !strings.Contains(view, snippet) {
			t.Fatalf("rendered view missing %q:\n%s", snippet, view)
		}
	}
}
