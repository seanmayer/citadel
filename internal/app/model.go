package app

import (
	"context"
	"errors"
	"fmt"
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
	ListWorktrees(ctx context.Context, repoRoot string, currentWorktreePath string, showDirty bool) ([]git.Worktree, error)
}

type CommandService interface {
	Execute(ctx context.Context, worktreePath string, raw string) (commands.Result, error)
}

type worktreesLoadedMsg struct {
	worktrees []git.Worktree
	err       error
}

type commandFinishedMsg struct {
	result commands.Result
	err    error
}

type Model struct {
	config              config.Config
	gitService          GitService
	commandService      CommandService
	repoRoot            string
	currentWorktreePath string
	worktrees           []git.Worktree
	selected            int
	top                 int
	width               int
	height              int
	state               ui.Mode
	previousState       ui.Mode
	renderer            ui.Renderer
	keys                ui.KeyMap
	commandInput        *commands.InputModel
	outputViewport      viewport.Model
	lastCommand         string
	statusMessage       string
	errorMessage        string
}

func New(cfg config.Config, gitService GitService, commandService CommandService, currentWorktreePath string) *Model {
	keys := ui.NewKeyMap(cfg.Keybindings)
	return &Model{
		config:              cfg,
		gitService:          gitService,
		commandService:      commandService,
		repoRoot:            currentWorktreePath,
		currentWorktreePath: currentWorktreePath,
		state:               ui.ModeList,
		renderer:            ui.NewRenderer(cfg),
		keys:                keys,
		commandInput:        commands.NewInputModel(cfg.DefaultCommand),
		outputViewport:      viewport.New(80, 20),
	}
}

func (m *Model) Init() tea.Cmd {
	return m.loadWorktreesCmd()
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
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			return m, nil
		}
		m.worktrees = msg.worktrees
		if len(m.worktrees) == 0 {
			m.selected = 0
			m.top = 0
		} else if m.selected >= len(m.worktrees) {
			m.selected = len(m.worktrees) - 1
		}
		m.ensureSelectionVisible()
		m.statusMessage = fmt.Sprintf("Loaded %d worktree(s)", len(m.worktrees))
		m.errorMessage = ""
		return m, nil
	case commandFinishedMsg:
		var validationErr *commands.ValidationError
		if errors.As(msg.err, &validationErr) {
			m.errorMessage = validationErr.Error()
			m.statusMessage = "Command was not executed."
			return m, nil
		}

		m.lastCommand = msg.result.Parsed.Raw
		m.errorMessage = ""
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
		}
		m.statusMessage = "Command finished."
		m.state = ui.ModeOutput
		m.outputViewport.SetContent(msg.result.Output)
		m.outputViewport.GotoTop()
		return m, nil
	}

	switch m.state {
	case ui.ModeCommand:
		return m.updateCommand(msg)
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
		return m, m.loadWorktreesCmd()
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
			return m, m.executeCommandCmd(worktree.Path, raw)
		}
	}

	cmd := m.commandInput.Update(msg)
	return m, cmd
}

func (m *Model) updateOutput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Back):
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

func (m *Model) loadWorktreesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		worktrees, err := m.gitService.ListWorktrees(ctx, m.repoRoot, m.currentWorktreePath, m.config.UI.ShowDirtyStatus)
		return worktreesLoadedMsg{
			worktrees: worktrees,
			err:       err,
		}
	}
}

func (m *Model) executeCommandCmd(worktreePath string, raw string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		result, err := m.commandService.Execute(ctx, worktreePath, raw)
		return commandFinishedMsg{
			result: result,
			err:    err,
		}
	}
}

func (m *Model) resizeComponents() {
	m.commandInput.SetWidth(m.commandInputWidth())

	outputWidth := m.width - 8
	outputHeight := m.height - 8
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

func (m *Model) selectedWorktree() (git.Worktree, bool) {
	if len(m.worktrees) == 0 || m.selected < 0 || m.selected >= len(m.worktrees) {
		return git.Worktree{}, false
	}
	return m.worktrees[m.selected], true
}
