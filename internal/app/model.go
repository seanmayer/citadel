package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanmayer/citadel/internal/commands"
	"github.com/seanmayer/citadel/internal/config"
	"github.com/seanmayer/citadel/internal/git"
	"github.com/seanmayer/citadel/internal/ui"
)

type GitService interface {
	ListWorktrees(ctx context.Context, repoRoot string, currentWorktreePath string, options git.RefreshOptions) ([]git.Worktree, error)
	DeleteWorktree(ctx context.Context, repoRoot string, worktree git.Worktree, options git.DeleteOptions) (string, error)
}

type CommandService interface {
	Execute(ctx context.Context, worktreePath string, raw string) (commands.Result, error)
}

type EditorService interface {
	Open(ctx context.Context, worktreePath string) (string, error)
}

type worktreesLoadedMsg struct {
	worktrees []git.Worktree
	err       error
	silent    bool
}

type commandFinishedMsg struct {
	result         commands.Result
	err            error
	refresh        bool
	successMessage string
}

type autoRefreshTickMsg struct {
	token int
}

type Model struct {
	config               config.Config
	gitService           GitService
	commandService       CommandService
	editorService        EditorService
	repoRoot             string
	currentWorktreePath  string
	worktrees            []git.Worktree
	selected             int
	top                  int
	width                int
	height               int
	state                ui.Mode
	previousState        ui.Mode
	renderer             ui.Renderer
	keys                 ui.KeyMap
	commandInput         *commands.InputModel
	branchInput          *commands.InputModel
	commitInput          *commands.InputModel
	outputViewport       viewport.Model
	deleteTarget         git.Worktree
	deleteOptions        git.DeleteOptions
	lastCommand          string
	statusMessage        string
	errorMessage         string
	autoRefreshToken     int
	pendingWorktreeLoads int
}

func New(cfg config.Config, gitService GitService, commandService CommandService, editorService EditorService, currentWorktreePath string) *Model {
	keys := ui.NewKeyMap(cfg.Keybindings)
	return &Model{
		config:              cfg,
		gitService:          gitService,
		commandService:      commandService,
		editorService:       editorService,
		repoRoot:            currentWorktreePath,
		currentWorktreePath: currentWorktreePath,
		state:               ui.ModeList,
		renderer:            ui.NewRenderer(cfg),
		keys:                keys,
		commandInput:        commands.NewInputModel(cfg.DefaultCommand),
		branchInput:         commands.NewPromptInputModel("branch> ", "feature/my-branch", ""),
		commitInput:         commands.NewPromptInputModel("message> ", "feat: describe changes", ""),
		outputViewport:      viewport.New(80, 20),
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.beginWorktreeLoad(false, false),
		m.scheduleAutoRefreshCmd(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeComponents()
		if m.state == ui.ModeOutput {
			m.outputViewport.GotoTop()
		}
		return m, nil
	case worktreesLoadedMsg:
		m.finishWorktreeLoad()
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			return m, m.scheduleAutoRefreshCmd()
		}
		m.worktrees = msg.worktrees
		if len(m.worktrees) == 0 {
			m.selected = 0
			m.top = 0
		} else if m.selected >= len(m.worktrees) {
			m.selected = len(m.worktrees) - 1
		}
		m.ensureSelectionVisible()
		if !msg.silent && m.state != ui.ModeOutput {
			m.statusMessage = fmt.Sprintf("Loaded %d worktree(s)", len(m.worktrees))
		}
		m.errorMessage = ""
		return m, m.scheduleAutoRefreshCmd()
	case autoRefreshTickMsg:
		if msg.token != m.autoRefreshToken {
			return m, nil
		}
		if m.pendingWorktreeLoads > 0 || !m.shouldAutoRefresh() {
			return m, m.scheduleAutoRefreshCmd()
		}
		return m, m.beginWorktreeLoad(false, true)
	case commandFinishedMsg:
		var validationErr *commands.ValidationError
		if errors.As(msg.err, &validationErr) {
			m.errorMessage = validationErr.Error()
			m.statusMessage = "Command was not executed."
			return m, nil
		}

		m.lastCommand = msg.result.Parsed.Raw
		m.clearDeleteState()
		m.errorMessage = ""
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
		}
		m.statusMessage = "Command finished."
		if msg.err == nil && msg.successMessage != "" {
			m.statusMessage = msg.successMessage
		}
		m.state = ui.ModeOutput
		m.outputViewport.SetContent(msg.result.Output)
		m.outputViewport.GotoTop()
		if msg.refresh {
			return m, m.beginWorktreeLoad(false, false)
		}
		return m, nil
	}

	switch m.state {
	case ui.ModeCommand:
		return m.updateCommand(msg)
	case ui.ModeCreate:
		return m.updateCreateBranch(msg)
	case ui.ModeCommit:
		return m.updateCommit(msg)
	case ui.ModeDelete:
		return m.updateDelete(msg)
	case ui.ModeOutput:
		return m.updateOutput(msg)
	case ui.ModeHelp:
		return m.updateHelp(msg)
	default:
		return m.updateList(msg)
	}
}

func (m *Model) View() string {
	vm := ui.ViewModel{
		Mode:          m.state,
		Width:         m.width,
		Height:        m.height,
		RepoRoot:      m.repoRoot,
		Worktrees:     m.worktrees,
		Selected:      m.selected,
		Top:           m.top,
		Config:        m.config,
		InputView:     m.commandInput.View(),
		BranchInput:   m.branchInput.View(),
		CommitInput:   m.commitInput.View(),
		OutputView:    m.outputViewport.View(),
		LastCommand:   m.lastCommand,
		StatusMessage: m.statusMessage,
		ErrorMessage:  m.errorMessage,
	}
	return m.renderer.Render(vm)
}

func (m *Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(keyMsg, m.keys.Help):
		m.previousState = m.state
		m.state = ui.ModeHelp
		return m, nil
	case key.Matches(keyMsg, m.keys.Refresh):
		m.statusMessage = "Refreshing worktrees..."
		m.errorMessage = ""
		return m, m.beginWorktreeLoad(false, false)
	case key.Matches(keyMsg, m.keys.FetchRefresh):
		m.statusMessage = "Fetching remote state and refreshing worktrees..."
		m.errorMessage = ""
		return m, m.beginWorktreeLoad(true, false)
	case key.Matches(keyMsg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
			m.ensureSelectionVisible()
		}
		return m, nil
	case key.Matches(keyMsg, m.keys.Down):
		if m.selected < len(m.worktrees)-1 {
			m.selected++
			m.ensureSelectionVisible()
		}
		return m, nil
	case key.Matches(keyMsg, m.keys.OpenEditor):
		worktree, ok := m.selectedWorktree()
		if !ok {
			return m, nil
		}
		if worktree.IsBare {
			m.statusMessage = "Editor was not opened."
			m.errorMessage = "Cannot open a bare worktree in an editor."
			return m, nil
		}
		m.statusMessage = "Opening worktree in editor..."
		m.errorMessage = ""
		return m, m.openEditorCmd(worktree.Path)
	case key.Matches(keyMsg, m.keys.StageAll):
		worktree, ok := m.selectedWorktree()
		if !ok {
			return m, nil
		}
		if worktree.IsBare {
			m.statusMessage = "Files were not staged."
			m.errorMessage = "Cannot stage files in a bare worktree."
			return m, nil
		}
		m.statusMessage = "Staging all changes..."
		m.errorMessage = ""
		return m, m.executeCommandCmd(worktree.Path, "git add .", true, "Staged all changes.")
	case key.Matches(keyMsg, m.keys.Commit):
		worktree, ok := m.selectedWorktree()
		if !ok {
			return m, nil
		}
		if worktree.IsBare {
			m.statusMessage = "Commit was not created."
			m.errorMessage = "Cannot commit files in a bare worktree."
			return m, nil
		}
		m.commitInput.Reset()
		m.commitInput.SetWidth(m.commandInputWidth())
		m.state = ui.ModeCommit
		m.statusMessage = "Commit mode."
		m.errorMessage = ""
		return m, m.commitInput.Focus()
	case key.Matches(keyMsg, m.keys.Create):
		worktree, ok := m.selectedWorktree()
		if !ok {
			return m, nil
		}
		if worktree.IsBare {
			m.statusMessage = "Branch was not created."
			m.errorMessage = "Cannot create a branch for a bare worktree."
			return m, nil
		}
		if worktree.HasNamedBranch() {
			m.statusMessage = "Branch was not created."
			m.errorMessage = fmt.Sprintf("Selected worktree already has branch %q.", worktree.Branch)
			return m, nil
		}
		m.branchInput.Reset()
		m.branchInput.SetWidth(m.commandInputWidth())
		m.state = ui.ModeCreate
		m.statusMessage = "Create branch mode."
		m.errorMessage = ""
		return m, m.branchInput.Focus()
	case key.Matches(keyMsg, m.keys.Delete):
		worktree, ok := m.selectedWorktree()
		if !ok {
			return m, nil
		}
		if worktree.IsCurrent {
			m.statusMessage = "Worktree was not deleted."
			m.errorMessage = "Cannot delete the current worktree."
			return m, nil
		}
		if worktree.IsBare {
			m.statusMessage = "Worktree was not deleted."
			m.errorMessage = "Cannot delete a bare worktree."
			return m, nil
		}
		m.deleteTarget = worktree
		m.deleteOptions = m.deleteOptionsFor(worktree)
		m.state = ui.ModeDelete
		m.statusMessage = "Delete mode. Press y to confirm or esc to cancel."
		m.errorMessage = ""
		return m, nil
	case key.Matches(keyMsg, m.keys.Enter):
		if len(m.worktrees) == 0 {
			return m, nil
		}
		m.commandInput.Reset()
		m.commandInput.SetWidth(m.commandInputWidth())
		m.state = ui.ModeCommand
		m.statusMessage = "Command mode."
		m.errorMessage = ""
		return m, m.commandInput.Focus()
	}

	return m, nil
}

func (m *Model) updateCommand(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case keyMsg.Type == tea.KeyCtrlC:
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Back):
			m.commandInput.Blur()
			m.state = ui.ModeList
			m.errorMessage = ""
			m.statusMessage = "Returned to worktree list."
			return m, nil
		case key.Matches(keyMsg, m.keys.Help):
			m.previousState = m.state
			m.state = ui.ModeHelp
			return m, nil
		case key.Matches(keyMsg, m.keys.Enter):
			worktree, ok := m.selectedWorktree()
			if !ok {
				return m, nil
			}
			raw := m.commandInput.ResolvedValue()
			m.statusMessage = "Running command..."
			m.errorMessage = ""
			return m, m.executeCommandCmd(worktree.Path, raw, false, "")
		}
	}

	cmd := m.commandInput.Update(msg)
	return m, cmd
}

func (m *Model) updateCreateBranch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case keyMsg.Type == tea.KeyCtrlC:
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Back):
			m.branchInput.Blur()
			m.state = ui.ModeList
			m.errorMessage = ""
			m.statusMessage = "Returned to worktree list."
			return m, nil
		case key.Matches(keyMsg, m.keys.Help):
			m.previousState = m.state
			m.state = ui.ModeHelp
			return m, nil
		case key.Matches(keyMsg, m.keys.Enter):
			worktree, ok := m.selectedWorktree()
			if !ok {
				return m, nil
			}

			branchName := m.branchInput.Value()
			if branchName == "" {
				m.statusMessage = "Branch was not created."
				m.errorMessage = "Branch name is empty."
				return m, nil
			}

			raw := fmt.Sprintf("git switch -c %q", branchName)
			m.statusMessage = "Creating branch..."
			m.errorMessage = ""
			return m, m.executeCommandCmd(worktree.Path, raw, true, "Branch created.")
		}
	}

	cmd := m.branchInput.Update(msg)
	return m, cmd
}

func (m *Model) updateCommit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case keyMsg.Type == tea.KeyCtrlC:
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Back):
			m.commitInput.Blur()
			m.state = ui.ModeList
			m.errorMessage = ""
			m.statusMessage = "Returned to worktree list."
			return m, nil
		case key.Matches(keyMsg, m.keys.Help):
			m.previousState = m.state
			m.state = ui.ModeHelp
			return m, nil
		case key.Matches(keyMsg, m.keys.Enter):
			worktree, ok := m.selectedWorktree()
			if !ok {
				return m, nil
			}

			message := m.commitInput.Value()
			if message == "" {
				m.statusMessage = "Commit was not created."
				m.errorMessage = "Commit message is empty."
				return m, nil
			}

			raw := fmt.Sprintf("git commit -m %q", message)
			m.statusMessage = "Committing changes..."
			m.errorMessage = ""
			return m, m.executeCommandCmd(worktree.Path, raw, true, "Committed changes.")
		}
	}

	cmd := m.commitInput.Update(msg)
	return m, cmd
}

func (m *Model) updateDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case keyMsg.Type == tea.KeyCtrlC:
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Back):
			m.clearDeleteState()
			m.state = ui.ModeList
			m.errorMessage = ""
			m.statusMessage = "Returned to worktree list."
			return m, nil
		case key.Matches(keyMsg, m.keys.Help):
			m.previousState = m.state
			m.state = ui.ModeHelp
			return m, nil
		case isConfirmDeleteKey(keyMsg):
			worktree := m.deleteTarget
			if worktree.Path == "" {
				selected, ok := m.selectedWorktree()
				if !ok {
					return m, nil
				}
				worktree = selected
			}
			m.statusMessage = "Deleting worktree..."
			m.errorMessage = ""
			return m, m.executeDeleteCmd(worktree, m.deleteOptions)
		}
	}

	return m, nil
}

func (m *Model) updateOutput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Continue), key.Matches(keyMsg, m.keys.Back):
			m.state = ui.ModeList
			m.errorMessage = ""
			m.statusMessage = "Returned to worktree list."
			return m, nil
		case key.Matches(keyMsg, m.keys.Help):
			m.previousState = m.state
			m.state = ui.ModeHelp
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.outputViewport, cmd = m.outputViewport.Update(msg)
	return m, cmd
}

func (m *Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(keyMsg, m.keys.Back), key.Matches(keyMsg, m.keys.Help):
		m.state = m.previousState
		return m, nil
	}

	return m, nil
}

func (m *Model) beginWorktreeLoad(forceFetch bool, silent bool) tea.Cmd {
	m.pendingWorktreeLoads++
	return m.loadWorktreesCmd(forceFetch, silent)
}

func (m *Model) finishWorktreeLoad() {
	if m.pendingWorktreeLoads > 0 {
		m.pendingWorktreeLoads--
	}
}

func (m *Model) loadWorktreesCmd(forceFetch bool, silent bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		worktrees, err := m.gitService.ListWorktrees(ctx, m.repoRoot, m.currentWorktreePath, git.RefreshOptions{
			BaseBranch: m.config.Git.BaseBranch,
			Fetch:      forceFetch || m.config.Git.FetchOnRefresh,
		})
		return worktreesLoadedMsg{
			worktrees: worktrees,
			err:       err,
			silent:    silent,
		}
	}
}

func (m *Model) scheduleAutoRefreshCmd() tea.Cmd {
	interval := m.config.Git.AutoRefreshInterval.Duration
	if interval <= 0 {
		return nil
	}

	m.autoRefreshToken++
	token := m.autoRefreshToken
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return autoRefreshTickMsg{token: token}
	})
}

func (m *Model) shouldAutoRefresh() bool {
	return m.state == ui.ModeList
}

func (m *Model) executeCommandCmd(worktreePath string, raw string, refresh bool, successMessage string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		result, err := m.commandService.Execute(ctx, worktreePath, raw)
		return commandFinishedMsg{
			result:         result,
			err:            err,
			refresh:        refresh,
			successMessage: successMessage,
		}
	}
}

func (m *Model) executeDeleteCmd(worktree git.Worktree, options git.DeleteOptions) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		output, err := m.gitService.DeleteWorktree(ctx, m.repoRoot, worktree, options)
		return commandFinishedMsg{
			result: commands.Result{
				Parsed: commands.ParsedCommand{Raw: deleteCommandLabel(worktree, options)},
				Output: output,
			},
			err:            err,
			refresh:        true,
			successMessage: "Worktree deleted.",
		}
	}
}

func (m *Model) openEditorCmd(worktreePath string) tea.Cmd {
	return func() tea.Msg {
		commandLabel := m.editorCommandLabel(worktreePath)
		if m.editorService == nil {
			return commandFinishedMsg{
				result: commands.Result{
					Parsed: commands.ParsedCommand{Raw: commandLabel},
					Output: "editor service is not configured",
				},
				err: errors.New("editor service is not configured"),
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		output, err := m.editorService.Open(ctx, worktreePath)
		return commandFinishedMsg{
			result: commands.Result{
				Parsed: commands.ParsedCommand{Raw: commandLabel},
				Output: output,
			},
			err:            err,
			successMessage: "Opened worktree in editor.",
		}
	}
}

func (m *Model) resizeComponents() {
	m.commandInput.SetWidth(m.commandInputWidth())
	m.branchInput.SetWidth(m.commandInputWidth())
	m.commitInput.SetWidth(m.commandInputWidth())

	outputWidth := m.width - 8
	outputHeight := m.height - 13
	if outputWidth < 20 {
		outputWidth = 20
	}
	if outputHeight < 5 {
		outputHeight = 5
	}

	m.outputViewport.Width = outputWidth
	m.outputViewport.Height = outputHeight
	m.ensureSelectionVisible()
}

func (m *Model) ensureSelectionVisible() {
	visible := m.listHeight()
	if m.selected < m.top {
		m.top = m.selected
	}
	if m.selected >= m.top+visible {
		m.top = m.selected - visible + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

func (m *Model) listHeight() int {
	if m.height <= 0 {
		return 10
	}
	value := m.height - 10
	if value < 6 {
		return 6
	}
	return value
}

func (m *Model) commandInputWidth() int {
	width := (m.width * 62 / 100) - 16
	if width < 24 {
		return 24
	}
	return width
}

func (m *Model) editorCommandLabel(worktreePath string) string {
	command := strings.TrimSpace(m.config.Editor.Command)
	if command == "" {
		command = "editor"
	}

	parts := []string{command}
	for _, arg := range m.config.Editor.Args {
		parts = append(parts, strings.ReplaceAll(arg, "{path}", worktreePath))
	}

	return strings.Join(parts, " ")
}

func (m *Model) selectedWorktree() (git.Worktree, bool) {
	if len(m.worktrees) == 0 || m.selected < 0 || m.selected >= len(m.worktrees) {
		return git.Worktree{}, false
	}
	return m.worktrees[m.selected], true
}

func (m *Model) clearDeleteState() {
	m.deleteTarget = git.Worktree{}
	m.deleteOptions = git.DeleteOptions{}
}

func (m *Model) deleteOptionsFor(worktree git.Worktree) git.DeleteOptions {
	return git.DeleteOptions{
		ForceRemove: worktree.Locked || (worktree.Status.DirtyKnown() && worktree.Status.IsDirty),
		ForceBranch: worktree.HasNamedBranch() && (!worktree.Status.MergeKnown() || !worktree.Status.MergedIntoBase),
	}
}

func deleteCommandLabel(worktree git.Worktree, options git.DeleteOptions) string {
	parts := make([]string, 0, 2)

	removeArgs := []string{"worktree", "remove"}
	if options.ForceRemove {
		removeArgs = append(removeArgs, "--force")
	}
	removeArgs = append(removeArgs, worktree.Path)
	parts = append(parts, "git "+strings.Join(removeArgs, " "))

	if worktree.HasNamedBranch() {
		deleteFlag := "-d"
		if options.ForceBranch {
			deleteFlag = "-D"
		}
		parts = append(parts, fmt.Sprintf("git branch %s %s", deleteFlag, worktree.Branch))
	}

	return strings.Join(parts, " && ")
}

func isConfirmDeleteKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y')
}
