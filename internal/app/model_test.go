package app

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanmayer/citadel/internal/commands"
	"github.com/seanmayer/citadel/internal/config"
	"github.com/seanmayer/citadel/internal/git"
	"github.com/seanmayer/citadel/internal/ui"
)

type stubGitService struct {
	worktrees             []git.Worktree
	listCalls             []listCall
	listErr               error
	deleteCalls           []deleteCall
	deleteOutput          string
	deleteErr             error
	openPullRequestCalls  []openPullRequestCall
	openPullRequestOutput string
	openPullRequestError  error
}

type listCall struct {
	repoRoot            string
	currentWorktreePath string
	options             git.RefreshOptions
}

func (s *stubGitService) ListWorktrees(_ context.Context, repoRoot string, currentWorktreePath string, options git.RefreshOptions) ([]git.Worktree, error) {
	s.listCalls = append(s.listCalls, listCall{
		repoRoot:            repoRoot,
		currentWorktreePath: currentWorktreePath,
		options:             options,
	})
	return s.worktrees, s.listErr
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

type openPullRequestCall struct {
	worktreePath string
	branch       string
}

func (s *stubGitService) OpenPullRequest(_ context.Context, worktreePath string, branch string) (string, error) {
	s.openPullRequestCalls = append(s.openPullRequestCalls, openPullRequestCall{
		worktreePath: worktreePath,
		branch:       branch,
	})

	if s.openPullRequestOutput == "" {
		return "(no output)", s.openPullRequestError
	}

	return s.openPullRequestOutput, s.openPullRequestError
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

func TestNewStartsInSplashMode(t *testing.T) {
	t.Parallel()

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, "/repo")

	if model.state != ui.ModeSplash {
		t.Fatalf("state = %q, want %q", model.state, ui.ModeSplash)
	}
}

func TestSplashFinishedTransitionsToList(t *testing.T) {
	t.Parallel()

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, "/repo")

	next, _ := model.Update(splashFinishedMsg{})
	updated := next.(*Model)

	if updated.state != ui.ModeList {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeList)
	}
}

func TestAutoRefreshTickReloadsWorktreesSilentlyInListMode(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Git.AutoRefreshInterval = config.Duration{Duration: 5 * time.Second}

	gitService := &stubGitService{
		worktrees: []git.Worktree{{
			Path:   "/repo",
			Branch: "main",
		}},
	}
	model := New(cfg, gitService, &stubCommandService{}, &stubEditorService{}, "/repo")
	model.state = ui.ModeList
	model.statusMessage = "Watching worktrees."
	model.autoRefreshToken = 1

	next, cmd := model.Update(autoRefreshTickMsg{token: 1})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("auto refresh command = nil, want load command")
	}
	if updated.pendingWorktreeLoads != 1 {
		t.Fatalf("pendingWorktreeLoads = %d, want 1", updated.pendingWorktreeLoads)
	}

	msg := cmd()
	loaded, ok := msg.(worktreesLoadedMsg)
	if !ok {
		t.Fatalf("message type = %T, want worktreesLoadedMsg", msg)
	}
	if !loaded.silent {
		t.Fatal("silent = false, want true")
	}
	if len(gitService.listCalls) != 1 {
		t.Fatalf("ListWorktrees() call count = %d, want 1", len(gitService.listCalls))
	}
	if gitService.listCalls[0].options.Fetch {
		t.Fatal("Fetch = true, want false")
	}

	next, rescheduleCmd := updated.Update(loaded)
	updated = next.(*Model)
	if updated.pendingWorktreeLoads != 0 {
		t.Fatalf("pendingWorktreeLoads = %d, want 0", updated.pendingWorktreeLoads)
	}
	if updated.statusMessage != "Watching worktrees." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Watching worktrees.")
	}
	if len(updated.worktrees) != 1 || updated.worktrees[0].Branch != "main" {
		t.Fatalf("worktrees = %#v, want reloaded main worktree", updated.worktrees)
	}
	if rescheduleCmd == nil {
		t.Fatal("reschedule command = nil, want next auto-refresh timer")
	}
}

func TestAutoRefreshTickSkipsReloadOutsideListMode(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Git.AutoRefreshInterval = config.Duration{Duration: 5 * time.Second}

	gitService := &stubGitService{}
	model := New(cfg, gitService, &stubCommandService{}, &stubEditorService{}, "/repo")
	model.state = ui.ModeCommand
	model.autoRefreshToken = 2

	next, cmd := model.Update(autoRefreshTickMsg{token: 2})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("auto refresh timer = nil, want rescheduled timer")
	}
	if updated.pendingWorktreeLoads != 0 {
		t.Fatalf("pendingWorktreeLoads = %d, want 0", updated.pendingWorktreeLoads)
	}
	if len(gitService.listCalls) != 0 {
		t.Fatalf("ListWorktrees() call count = %d, want 0", len(gitService.listCalls))
	}
}

func TestCreateBranchKeyEntersCreateModeForBranchlessWorktree(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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

func TestHotPushRunsBuiltInWorkflowAndRefreshes(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{
		worktrees: []git.Worktree{{
			Path:   "/repo/feature",
			Branch: "feature/demo",
		}},
	}
	commandService := &stubCommandService{}
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
	model.worktrees = gitService.worktrees

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("hot push command = nil, want command")
	}
	if updated.statusMessage != "Running hot push..." {
		t.Fatalf("statusMessage = %q, want hot push message", updated.statusMessage)
	}

	msg := cmd()
	finished, ok := msg.(commandFinishedMsg)
	if !ok {
		t.Fatalf("message type = %T, want commandFinishedMsg", msg)
	}
	if len(commandService.calls) != 1 {
		t.Fatalf("Execute() call count = %d, want 1", len(commandService.calls))
	}
	if commandService.calls[0].raw != "hot push" {
		t.Fatalf("raw command = %q, want %q", commandService.calls[0].raw, "hot push")
	}

	next, refreshCmd := updated.Update(finished)
	updated = next.(*Model)
	if updated.state != ui.ModeOutput {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeOutput)
	}
	if updated.statusMessage != "Hot push completed." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Hot push completed.")
	}
	if refreshCmd == nil {
		t.Fatal("refresh command = nil, want reload command")
	}
}

func TestCommitKeyEntersCommitMode(t *testing.T) {
	t.Parallel()

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), gitService, commandService, &stubEditorService{}, "/repo")
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
	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, editorService, "/repo")
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

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, "/repo")
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

func TestOpenPullRequestRunsForSelectedBranchWorktree(t *testing.T) {
	t.Parallel()

	gitService := &stubGitService{openPullRequestOutput: "opened browser"}
	model := New(config.Defaults(), gitService, &stubCommandService{}, &stubEditorService{}, "/repo")
	model.worktrees = []git.Worktree{{
		Path:   "/repo/feature",
		Branch: "feature/demo",
	}}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	updated := next.(*Model)
	if cmd == nil {
		t.Fatal("open pull request command = nil, want command")
	}
	if updated.statusMessage != "Opening pull request in browser..." {
		t.Fatalf("statusMessage = %q, want opening message", updated.statusMessage)
	}

	msg := cmd()
	finished, ok := msg.(commandFinishedMsg)
	if !ok {
		t.Fatalf("message type = %T, want commandFinishedMsg", msg)
	}
	if finished.err != nil {
		t.Fatalf("open pull request error = %v, want nil", finished.err)
	}
	if finished.result.Output != "opened browser" {
		t.Fatalf("output = %q, want %q", finished.result.Output, "opened browser")
	}
	if len(gitService.openPullRequestCalls) != 1 {
		t.Fatalf("OpenPullRequest() call count = %d, want 1", len(gitService.openPullRequestCalls))
	}
	if gitService.openPullRequestCalls[0].worktreePath != "/repo/feature" {
		t.Fatalf("worktreePath = %q, want %q", gitService.openPullRequestCalls[0].worktreePath, "/repo/feature")
	}
	if gitService.openPullRequestCalls[0].branch != "feature/demo" {
		t.Fatalf("branch = %q, want %q", gitService.openPullRequestCalls[0].branch, "feature/demo")
	}

	next, _ = updated.Update(finished)
	updated = next.(*Model)
	if updated.statusMessage != "Opened pull request in browser." {
		t.Fatalf("statusMessage = %q, want %q", updated.statusMessage, "Opened pull request in browser.")
	}
	if updated.errorMessage != "" {
		t.Fatalf("errorMessage = %q, want empty", updated.errorMessage)
	}
	if updated.state != ui.ModeOutput {
		t.Fatalf("state = %q, want %q", updated.state, ui.ModeOutput)
	}
	if updated.lastCommand != "gh pr view feature/demo --web" {
		t.Fatalf("lastCommand = %q, want %q", updated.lastCommand, "gh pr view feature/demo --web")
	}
}

func TestOpenPullRequestRejectsDetachedWorktree(t *testing.T) {
	t.Parallel()

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, "/repo")
	model.worktrees = []git.Worktree{{
		Path:       "/repo/detached",
		Branch:     "detached",
		IsDetached: true,
	}}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	updated := next.(*Model)
	if cmd != nil {
		t.Fatal("open pull request command != nil, want nil")
	}
	if updated.errorMessage != "Selected worktree does not have a named branch." {
		t.Fatalf("errorMessage = %q, want detached-branch error", updated.errorMessage)
	}
}

func TestOutputModeSpaceReturnsToList(t *testing.T) {
	t.Parallel()

	model := New(config.Defaults(), &stubGitService{}, &stubCommandService{}, &stubEditorService{}, "/repo")
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
