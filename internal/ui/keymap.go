package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"

	"github.com/seanmayer/citadel/internal/config"
)

type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Create       key.Binding
	Delete       key.Binding
	Enter        key.Binding
	Refresh      key.Binding
	FetchRefresh key.Binding
	Back         key.Binding
	Help         key.Binding
	Quit         key.Binding
}

func NewKeyMap(cfg config.Keybindings) KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "move down"),
		),
		Create: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "create branch"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete worktree"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open or run"),
		),
		Refresh: key.NewBinding(
			key.WithKeys(cfg.Refresh),
			key.WithHelp(cfg.Refresh, "refresh"),
		),
		FetchRefresh: key.NewBinding(
			key.WithKeys(cfg.FetchRefresh),
			key.WithHelp(cfg.FetchRefresh, "fetch + refresh"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Help: key.NewBinding(
			key.WithKeys(cfg.Help),
			key.WithHelp(cfg.Help, "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys(cfg.Quit, "ctrl+c"),
			key.WithHelp(cfg.Quit+"/ctrl+c", "quit"),
		),
	}
}

func (k KeyMap) ShortHelp() string {
	items := []key.Binding{k.Up, k.Down, k.Create, k.Delete, k.Enter, k.Refresh, k.FetchRefresh, k.Back, k.Help, k.Quit}
	parts := make([]string, 0, len(items))
	for _, binding := range items {
		help := binding.Help()
		parts = append(parts, help.Key+" "+help.Desc)
	}
	return strings.Join(parts, "  ")
}
