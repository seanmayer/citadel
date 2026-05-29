package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"

	"github.com/seanmayer/citadel/internal/config"
)

type KeyMap struct {
	Up              key.Binding
	Down            key.Binding
	OpenEditor      key.Binding
	OpenPullRequest key.Binding
	StageAll        key.Binding
	HotPush         key.Binding
	Commit          key.Binding
	Create          key.Binding
	Delete          key.Binding
	Enter           key.Binding
	Continue        key.Binding
	Refresh         key.Binding
	FetchRefresh    key.Binding
	Back            key.Binding
	Help            key.Binding
	Quit            key.Binding
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
		OpenEditor: key.NewBinding(
			key.WithKeys(cfg.OpenEditor),
			key.WithHelp(cfg.OpenEditor, "open editor"),
		),
		OpenPullRequest: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "open PR"),
		),
		StageAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "git add ."),
		),
		HotPush: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "hot push"),
		),
		Commit: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "commit changes"),
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
			key.WithHelp("enter", "command/run"),
		),
		Continue: key.NewBinding(
			key.WithKeys(" ", "enter"),
			key.WithHelp("space/enter", "continue"),
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
	items := []key.Binding{k.Up, k.Down, k.OpenEditor, k.OpenPullRequest, k.StageAll, k.HotPush, k.Commit, k.Create, k.Delete, k.Enter, k.Refresh, k.FetchRefresh, k.Back, k.Help, k.Quit}
	return helpString(items)
}

func (k KeyMap) OutputHelp() string {
	items := []key.Binding{k.Up, k.Down, k.Continue, k.Back, k.Help, k.Quit}
	return helpString(items)
}

func helpString(items []key.Binding) string {
	parts := make([]string, 0, len(items))
	for _, binding := range items {
		help := binding.Help()
		parts = append(parts, help.Key+" "+help.Desc)
	}
	return strings.Join(parts, "  ")
}
