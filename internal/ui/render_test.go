package ui

import (
	"strings"
	"testing"

	"github.com/seanmayer/citadel/internal/config"
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
