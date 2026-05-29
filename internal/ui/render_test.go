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

func TestRenderSplashShowsCitadelBranding(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(config.Defaults())
	view := renderer.Render(ViewModel{
		Mode:     ModeSplash,
		Width:    100,
		Height:   30,
		RepoRoot: "/repo/project",
	})

	for _, snippet := range []string{
		"citadel",
		"minimal git worktree command center",
		"loading worktrees",
		"press any key to continue",
	} {
		if !strings.Contains(view, snippet) {
			t.Fatalf("rendered splash missing %q:\n%s", snippet, view)
		}
	}
}

func TestRenderListShowsWorktreeActionList(t *testing.T) {
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
		Selected: 0,
		Config:   config.Defaults(),
	})

	for _, snippet := range []string{
		"Actions",
		"open terminal command mode",
		"open this worktree in a new terminal",
		"open the pull request for this branch",
		"git add .",
		"hot push",
		"commit files with a message",
	} {
		if !strings.Contains(view, snippet) {
			t.Fatalf("rendered view missing %q:\n%s", snippet, view)
		}
	}
}

func TestRenderListMarksSelectedWorktree(t *testing.T) {
	t.Parallel()

	renderer := NewRenderer(config.Defaults())
	view := renderer.Render(ViewModel{
		Mode:     ModeList,
		Width:    100,
		Height:   30,
		RepoRoot: "/repo",
		Worktrees: []git.Worktree{
			{Path: "/repo", Branch: "main"},
			{Path: "/repo/feature", Branch: "feature/demo"},
		},
		Selected: 1,
		Config:   config.Defaults(),
	})

	if !strings.Contains(view, "> feature/demo") {
		t.Fatalf("rendered view missing selected marker:\n%s", view)
	}
}
