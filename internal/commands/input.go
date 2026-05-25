package commands

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type InputModel struct {
	model          textinput.Model
	defaultCommand string
}

func NewInputModel(defaultCommand string) *InputModel {
	model := textinput.New()
	model.Prompt = "> "
	model.Placeholder = defaultCommand
	model.SetValue(defaultCommand)
	model.CursorEnd()
	model.CharLimit = 512

	return &InputModel{
		model:          model,
		defaultCommand: defaultCommand,
	}
}

func (m *InputModel) Focus() tea.Cmd {
	return m.model.Focus()
}

func (m *InputModel) Blur() {
	m.model.Blur()
}

func (m *InputModel) Reset() {
	m.model.SetValue(m.defaultCommand)
	m.model.CursorEnd()
}

func (m *InputModel) SetWidth(width int) {
	if width < 10 {
		width = 10
	}
	m.model.Width = width
}

func (m *InputModel) Value() string {
	return strings.TrimSpace(m.model.Value())
}

func (m *InputModel) ResolvedValue() string {
	value := m.Value()
	if value == "" {
		return m.defaultCommand
	}
	return value
}

func (m *InputModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.model, cmd = m.model.Update(msg)
	return cmd
}

func (m *InputModel) View() string {
	return m.model.View()
}
