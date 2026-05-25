package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanmayer/citadel/internal/commands"
	"github.com/seanmayer/citadel/internal/config"
	"github.com/seanmayer/citadel/internal/git"
	"github.com/seanmayer/citadel/internal/ui"
)

type stubGitService struct {
	worktrees []git.Worktree
}

func (s *stubGitService) ListWorktrees(_ context.Context, _ string, _ string, _ git.RefreshOptions) ([]git.Worktree, error) {
	return s.worktrees, nil
}

type commandCall struct {
	worktreePath string
	raw          string
}

type stubCommandService struct {
	calls  []commandCall
	result commands.Result
	err    error
}

func (s *stubCommandService) Execute(_ context.Context, worktreePath string, raw string) (commands.Result, error) {
	s.calls = append(s.calls, commandCall{
		worktreePath: worktreePath,
		raw:          raw,
	})

	if s.result.Parsed.Raw == "" && s.result.Output == "" {
		return commands.Result{
			Parsed: commands.ParsedCommand{Raw: raw},
			Output: "ok",
		}, s.err
	}

	return s.result, s.err
}

func TestCreateBranchKeyEntersCreateModeForBranchlessWorktree(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, "/repo")
	model.width = 120
	model.height = 40
	model.worktrees = []git.Worktree{{
		Path:       "/repo/detached",
		Branch:     "detached",
		IsDetached: true,
	}}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	updated := next.(*Model)

	if updated.state != ui.ModeCreate {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeCreate)
	}
	if updated.statusMessage != "Create branch mode." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Create branch mode.")
	}
}

func TestCreateBranchKeyRejectsWorktreeWithExistingBranch(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, "/repo")
	model.worktrees = []git.Worktree{{
		Path:   "/repo/main",
		Branch: "main",
	}}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	updated := next.(*Model)

	if updated.state != ui.ModeList {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeList)
	}
	if updated.errorMessage == "" {
		t.Fatal("errorMessage = empty, want branch creation error")
	}
}

func TestCreateBranchModeRunsGitSwitchCreateAndRefreshes(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{
		worktrees: []git.Worktree{{
			Path:       "/repo/detached",
			Branch:     "detached",
			IsDetached: true,
		}},
	}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, "/repo")
	model.width = 120
	model.height = 40
	model.worktrees = gitService.worktrees
	model.state = ui.ModeCreate
	model.branchInput.SetValue("feature/demo")

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("create branch command = nil, want command")
	}

	msg := cmd()
	finished, ok := msg.(commandFinishedMsg)
	if !ok {
		t.Fatalf("message type = %T, want commandFinishedMsg", msg)
	}

	if len(commandService.calls) != 1 {
		t.Fatalf("Execute() call count = %d, want 1", len(commandService.calls))
	}

	call := commandService.calls[0]
	if call.worktreePath != "/repo/detached" {
		t.Fatalf("worktreePath = %q, want %q", call.worktreePath, "/repo/detached")
	}
	if call.raw != `git switch -c "feature/demo"` {
		t.Fatalf("raw command = %q, want %q", call.raw, `git switch -c "feature/demo"`)
	}

	gitService.worktrees = []git.Worktree{{
		Path:   "/repo/detached",
		Branch: "feature/demo",
	}}

	next, refreshCmd := updated.Update(finished)
	updated = next.(*Model)
	if updated.state != ui.ModeOutput {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeOutput)
	}
	if updated.statusMessage != "Branch created." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Branch created.")
	}
	if refreshCmd == nil {
		t.Fatal("refresh command = nil, want reload command")
	}

	reloadMsg := refreshCmd()
	worktreesMsg, ok := reloadMsg.(worktreesLoadedMsg)
	if !ok {
		t.Fatalf("reload message type = %T, want worktreesLoadedMsg", reloadMsg)
	}
	if len(worktreesMsg.worktrees) != 1 || worktreesMsg.worktrees[0].Branch != "feature/demo" {
		t.Fatalf("reloaded worktrees = %#v, want branch feature/demo", worktreesMsg.worktrees)
	}
}
