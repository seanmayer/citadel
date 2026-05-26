package ui

import "github.com/charmbracelet/lipgloss"

type Styles struct {
	Root          lipgloss.Style
	Header        lipgloss.Style
	Subtle        lipgloss.Style
	Label         lipgloss.Style
	Value         lipgloss.Style
	Panel         lipgloss.Style
	Selected      lipgloss.Style
	CurrentBadge  lipgloss.Style
	DirtyBadge    lipgloss.Style
	Footer        lipgloss.Style
	Error         lipgloss.Style
	CommandBox    lipgloss.Style
	OutputBox     lipgloss.Style
	HelpBox       lipgloss.Style
	ContinueBox   lipgloss.Style
	ContinueTitle lipgloss.Style
	ContinueHint  lipgloss.Style
	Divider       lipgloss.Style
}

func DefaultStyles() Styles {
	return Styles{
		Root: lipgloss.NewStyle().
			Padding(1, 2),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D7AF")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6C7A89")),
		Label: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#5FD7FF")),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EAF7F7")),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3A5568")).
			Padding(1, 2),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0D1B1E")).
			Background(lipgloss.Color("#00D7AF")).
			Padding(0, 1),
		CurrentBadge: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0D1B1E")).
			Background(lipgloss.Color("#7DFFCF")).
			Padding(0, 1),
		DirtyBadge: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#2C1600")).
			Background(lipgloss.Color("#FFB454")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A7B6C2")),
		Error: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B81")),
		CommandBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#00D7AF")).
			Padding(1, 1),
		OutputBox: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#5FD7FF")).
			Padding(0, 1),
		HelpBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#5FD7FF")).
			Padding(1, 2),
		ContinueBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7DFFCF")).
			Background(lipgloss.Color("#10232A")).
			Padding(1, 2).
			Align(lipgloss.Center),
		ContinueTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0D1B1E")).
			Background(lipgloss.Color("#7DFFCF")).
			Padding(0, 1),
		ContinueHint: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D7FFF5")).
			Align(lipgloss.Center),
		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3A5568")),
	}
}
