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
	worktrees    []git.Worktree
	deleteCalls  []deleteCall
	deleteOutput string
	deleteErr    error
}

func (s *stubGitService) ListWorktrees(_ context.Context, _ string, _ string, _ git.RefreshOptions) ([]git.Worktree, error) {
	return s.worktrees, nil
}

type deleteCall struct {
	repoRoot string
	worktree git.Worktree
	options  git.DeleteOptions
}

func (s *stubGitService) DeleteWorktree(_ context.Context, repoRoot string, worktree git.Worktree, options git.DeleteOptions) (string, error) {
	s.deleteCalls = append(s.deleteCalls, deleteCall{
		repoRoot: repoRoot,
		worktree: worktree,
		options:  options,
	})

	if s.deleteOutput == "" {
		return "deleted", s.deleteErr
	}

	return s.deleteOutput, s.deleteErr
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

type stubEditorService struct {
	paths  []string
	output string
	err    error
}

func (s *stubEditorService) Open(_ context.Context, worktreePath string) (string, error) {
	s.paths = append(s.paths, worktreePath)
	if s.output == "" {
		return "(no output)", s.err
	}
	return s.output, s.err
}

type stubTerminalService struct {
	paths  []string
	output string
	err    error
}

func (s *stubTerminalService) Open(_ context.Context, worktreePath string) (string, error) {
	s.paths = append(s.paths, worktreePath)
	if s.output == "" {
		return "(no output)", s.err
	}
	return s.output, s.err
}

func TestCreateBranchKeyEntersCreateModeForBranchlessWorktree(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
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

func TestStageAllRunsGitAddAndRefreshes(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{
		worktrees: []git.Worktree{{
			Path:   "/repo/feature",
			Branch: "feature/demo",
		}},
	}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.worktrees = gitService.worktrees

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("stage all command = nil, want command")
	}
	if updated.statusMessage != "Staging all changes..." {
		t.Fatalf("statusMessage = %q, want staging message", updated.statusMessage)
	}

	msg := cmd()
	finished, ok := msg.(commandFinishedMsg)
	if !ok {
		t.Fatalf("message type = %T, want commandFinishedMsg", msg)
	}
	if len(commandService.calls) != 1 {
		t.Fatalf("Execute() call count = %d, want 1", len(commandService.calls))
	}
	if commandService.calls[0].raw != "git add ." {
		t.Fatalf("raw command = %q, want %q", commandService.calls[0].raw, "git add .")
	}

	next, refreshCmd := updated.Update(finished)
	updated = next.(*Model)
	if updated.state != ui.ModeOutput {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeOutput)
	}
	if updated.statusMessage != "Staged all changes." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Staged all changes.")
	}
	if refreshCmd == nil {
		t.Fatal("refresh command = nil, want reload command")
	}
}

func TestCommitKeyEntersCommitMode(t *testing.T) {
	t.Parallel()

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.width = 120
	model.height = 40
	model.worktrees = []git.Worktree{{
		Path:   "/repo/feature",
		Branch: "feature/demo",
	}}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	updated := next.(*Model)

	if updated.state != ui.ModeCommit {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeCommit)
	}
	if updated.statusMessage != "Commit mode." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Commit mode.")
	}
}

func TestCommitModeRunsGitCommitAndRefreshes(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{
		worktrees: []git.Worktree{{
			Path:   "/repo/feature",
			Branch: "feature/demo",
		}},
	}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.width = 120
	model.height = 40
	model.worktrees = gitService.worktrees
	model.state = ui.ModeCommit
	model.commitInput.SetValue("feat: save worktree changes")

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("commit command = nil, want command")
	}

	msg := cmd()
	finished, ok := msg.(commandFinishedMsg)
	if !ok {
		t.Fatalf("message type = %T, want commandFinishedMsg", msg)
	}
	if len(commandService.calls) != 1 {
		t.Fatalf("Execute() call count = %d, want 1", len(commandService.calls))
	}
	if commandService.calls[0].raw != `git commit -m "feat: save worktree changes"` {
		t.Fatalf("raw command = %q, want commit command", commandService.calls[0].raw)
	}

	next, refreshCmd := updated.Update(finished)
	updated = next.(*Model)
	if updated.state != ui.ModeOutput {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeOutput)
	}
	if updated.statusMessage != "Committed changes." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Committed changes.")
	}
	if refreshCmd == nil {
		t.Fatal("refresh command = nil, want reload command")
	}
}

func TestDeleteKeyEntersDeleteModeForSelectableWorktree(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.worktrees = []git.Worktree{{
		Path:       "/repo/detached",
		Branch:     "detached",
		IsDetached: true,
	}}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated := next.(*Model)

	if updated.state != ui.ModeDelete {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeDelete)
	}
	if updated.statusMessage != "Delete mode. Press y to confirm or esc to cancel." {
		t.Fatalf("statusMessage = %q, want delete confirmation message", updated.statusMessage)
	}
}

func TestDeleteKeyRejectsCurrentWorktree(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.worktrees = []git.Worktree{{
		Path:      "/repo",
		Branch:    "main",
		IsCurrent: true,
	}}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated := next.(*Model)

	if updated.state != ui.ModeList {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeList)
	}
	if updated.errorMessage != "Cannot delete the current worktree." {
		t.Fatalf("errorMessage = %q, want current-worktree error", updated.errorMessage)
	}
}

func TestDeleteModeRunsWorktreeDeletionAndRefreshes(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{
		worktrees: []git.Worktree{{
			Path:       "/repo/detached",
			Branch:     "detached",
			IsDetached: true,
		}},
	}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.worktrees = gitService.worktrees
	model.state = ui.ModeDelete
	model.deleteTarget = gitService.worktrees[0]

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("delete command = nil, want command")
	}

	msg := cmd()
	finished, ok := msg.(commandFinishedMsg)
	if !ok {
		t.Fatalf("message type = %T, want commandFinishedMsg", msg)
	}

	if len(gitService.deleteCalls) != 1 {
		t.Fatalf("DeleteWorktree() call count = %d, want 1", len(gitService.deleteCalls))
	}

	call := gitService.deleteCalls[0]
	if call.repoRoot != "/repo" {
		t.Fatalf("repoRoot = %q, want %q", call.repoRoot, "/repo")
	}
	if call.worktree.Path != "/repo/detached" {
		t.Fatalf("worktree.Path = %q, want %q", call.worktree.Path, "/repo/detached")
	}
	if call.options.ForceRemove {
		t.Fatalf("ForceRemove = true, want false")
	}
	if call.options.ForceBranch {
		t.Fatalf("ForceBranch = true, want false")
	}

	gitService.worktrees = []git.Worktree{{
		Path:   "/repo",
		Branch: "main",
	}}

	next, refreshCmd := updated.Update(finished)
	updated = next.(*Model)
	if updated.state != ui.ModeOutput {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeOutput)
	}
	if updated.statusMessage != "Worktree deleted." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Worktree deleted.")
	}
	if refreshCmd == nil {
		t.Fatal("refresh command = nil, want reload command")
	}

	reloadMsg := refreshCmd()
	worktreesMsg, ok := reloadMsg.(worktreesLoadedMsg)
	if !ok {
		t.Fatalf("reload message type = %T, want worktreesLoadedMsg", reloadMsg)
	}
	if len(worktreesMsg.worktrees) != 1 || worktreesMsg.worktrees[0].Branch != "main" {
		t.Fatalf("reloaded worktrees = %#v, want remaining main worktree", worktreesMsg.worktrees)
	}
}

func TestDeleteModeForcesBranchDeletionWhenMergeStatusIsUnknown(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{
		worktrees: []git.Worktree{{
			Path:   "/repo/feature",
			Branch: "feature/demo",
			Status: git.BranchStatus{
				MergedIntoBase: false,
			},
		}},
	}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.worktrees = gitService.worktrees

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated := next.(*Model)
	if updated.state != ui.ModeDelete {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeDelete)
	}

	_, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("delete command = nil, want command")
	}
	cmd()

	if len(gitService.deleteCalls) != 1 {
		t.Fatalf("DeleteWorktree() call count = %d, want 1", len(gitService.deleteCalls))
	}
	if !gitService.deleteCalls[0].options.ForceBranch {
		t.Fatalf("ForceBranch = false, want true")
	}
}

func TestOpenEditorRunsForSelectedWorktree(t *testing.T) {
	t.Parallel()

	editorService := &stubEditorService{output: "editor launched"}
	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, editorService, &stubTerminalService{}, "/repo")
	model.worktrees = []git.Worktree{{
		Path:   "/repo/feature",
		Branch: "feature/demo",
	}}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("open editor command = nil, want command")
	}
	if updated.statusMessage != "Opening worktree in editor..." {
		t.Fatalf("statusMessage = %q, want opening message", updated.statusMessage)
	}

	msg := cmd()
	finished, ok := msg.(commandFinishedMsg)
	if !ok {
		t.Fatalf("message type = %T, want commandFinishedMsg", msg)
	}
	if finished.err != nil {
		t.Fatalf("editor open error = %v, want nil", finished.err)
	}
	if finished.result.Output != "editor launched" {
		t.Fatalf("output = %q, want %q", finished.result.Output, "editor launched")
	}
	if len(editorService.paths) != 1 || editorService.paths[0] != "/repo/feature" {
		t.Fatalf("opened paths = %#v, want /repo/feature", editorService.paths)
	}

	next, _ = updated.Update(finished)
	updated = next.(*Model)
	if updated.statusMessage != "Opened worktree in editor." {
		t.Fatalf("statusMessage = %q, want success message", updated.statusMessage)
	}
	if updated.errorMessage != "" {
		t.Fatalf("errorMessage = %q, want empty", updated.errorMessage)
	}
	if updated.state != ui.ModeOutput {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeOutput)
	}
	if updated.lastCommand != "code ." {
		t.Fatalf("lastCommand = %q, want %q", updated.lastCommand, "code .")
	}
}

func TestOpenEditorRejectsBareWorktree(t *testing.T) {
	t.Parallel()

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.worktrees = []git.Worktree{{
		Path:   "/repo.git",
		Branch: "main",
		IsBare: true,
	}}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	updated := next.(*Model)
	if cmd != nil {
		t.Fatal("open editor command != nil, want nil")
	}
	if updated.errorMessage != "Cannot open a bare worktree in an editor." {
		t.Fatalf("errorMessage = %q, want bare-worktree error", updated.errorMessage)
	}
}

func TestOpenTerminalRunsForSelectedWorktree(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Terminal.Command = "wezterm"
	cfg.Terminal.Args = []string{"start", "--cwd", "{path}"}

	terminalService := &stubTerminalService{output: "terminal launched"}
	model := New(cfg, &stubGitService{}, &stubCommandService{}, &stubEditorService{}, terminalService, "/repo")
	model.worktrees = []git.Worktree{{
		Path:   "/repo/feature",
		Branch: "feature/demo",
	}}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("open terminal command = nil, want command")
	}
	if updated.statusMessage != "Opening worktree in terminal..." {
		t.Fatalf("statusMessage = %q, want opening message", updated.statusMessage)
	}

	msg := cmd()
	finished, ok := msg.(commandFinishedMsg)
	if !ok {
		t.Fatalf("message type = %T, want commandFinishedMsg", msg)
	}
	if finished.err != nil {
		t.Fatalf("terminal open error = %v, want nil", finished.err)
	}
	if finished.result.Output != "terminal launched" {
		t.Fatalf("output = %q, want %q", finished.result.Output, "terminal launched")
	}
	if len(terminalService.paths) != 1 || terminalService.paths[0] != "/repo/feature" {
		t.Fatalf("opened paths = %#v, want /repo/feature", terminalService.paths)
	}

	next, _ = updated.Update(finished)
	updated = next.(*Model)
	if updated.statusMessage != "Opened worktree in terminal." {
		t.Fatalf("statusMessage = %q, want success message", updated.statusMessage)
	}
	if updated.errorMessage != "" {
		t.Fatalf("errorMessage = %q, want empty", updated.errorMessage)
	}
	if updated.state != ui.ModeOutput {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeOutput)
	}
	if updated.lastCommand != "wezterm start --cwd /repo/feature" {
		t.Fatalf("lastCommand = %q, want %q", updated.lastCommand, "wezterm start --cwd /repo/feature")
	}
}

func TestOutputModeSpaceReturnsToList(t *testing.T) {
	t.Parallel()

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, &stubTerminalService{}, "/repo")
	model.state = ui.ModeOutput
	model.statusMessage = "Command finished."
	model.errorMessage = "stale"

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	updated := next.(*Model)

	if updated.state != ui.ModeList {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeList)
	}
	if updated.statusMessage != "Returned to worktree list." {
		t.Fatalf("statusMessage = %q, want return message", updated.statusMessage)
	}
	if updated.errorMessage != "" {
		t.Fatalf("errorMessage = %q, want empty", updated.errorMessage)
	}
}
