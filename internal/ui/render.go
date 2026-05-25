package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanmayer/citadel/internal/config"
	"github.com/seanmayer/citadel/internal/git"
)

type Mode string

const (
	ModeList    Mode = "list"
	ModeCommand Mode = "command"
	ModeOutput  Mode = "output"
	ModeHelp    Mode = "help"
)

type ViewModel struct {
	Mode          Mode
	Width         int
	Height        int
	RepoRoot      string
	Worktrees     []git.Worktree
	Selected      int
	Top           int
	Config        config.Config
	InputView     string
	OutputView    string
	LastCommand   string
	StatusMessage string
	ErrorMessage  string
}

type Renderer struct {
	styles Styles
	keys   KeyMap
}

func NewRenderer(cfg config.Config) Renderer {
	return Renderer{
		styles: DefaultStyles(),
		keys:   NewKeyMap(cfg.Keybindings),
	}
}

func (r Renderer) Render(vm ViewModel) string {
	switch vm.Mode {
	case ModeOutput:
		return r.styles.Root.Render(r.renderOutput(vm))
	case ModeHelp:
		return r.styles.Root.Render(r.renderHelp(vm))
	default:
		return r.styles.Root.Render(r.renderMain(vm))
	}
}

func (r Renderer) renderMain(vm ViewModel) string {
	header := r.styles.Header.Render("gwtui") + "\n" + r.styles.Subtle.Render(vm.RepoRoot)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, r.renderListPane(vm), r.renderDetailPane(vm))
	footer := r.renderFooter(vm)
	return lipgloss.JoinVertical(lipgloss.Left, header, panes, footer)
}

func (r Renderer) renderListPane(vm ViewModel) string {
	rows := make([]string, 0)
	if len(vm.Worktrees) == 0 {
		rows = append(rows, r.styles.Subtle.Render("No worktrees found."))
	} else {
		height := listHeight(vm.Height)
		end := min(len(vm.Worktrees), vm.Top+height)
		for i := vm.Top; i < end; i++ {
			worktree := vm.Worktrees[i]
			row := compactPath(worktree.Path)

			meta := make([]string, 0, 3)
			if vm.Config.UI.ShowBranch && worktree.Branch != "" {
				meta = append(meta, worktree.Branch)
			}
			if vm.Config.UI.ShowCommitHash && worktree.Head != "" {
				meta = append(meta, git.ShortHash(worktree.Head))
			}
			if vm.Config.UI.ShowDirtyStatus && worktree.DirtyKnown && worktree.IsDirty {
				meta = append(meta, "dirty")
			}

			if len(meta) > 0 {
				row = fmt.Sprintf("%s  %s", row, r.styles.Subtle.Render("["+strings.Join(meta, " | ")+"]"))
			}
			if worktree.IsCurrent {
				row += " " + r.styles.CurrentBadge.Render("current")
			}

			if i == vm.Selected {
				row = r.styles.Selected.Render(row)
			}

			rows = append(rows, row)
		}
	}

	title := r.styles.Label.Render("Worktrees")
	content := strings.Join(rows, "\n")
	return r.styles.Panel.Width(panelWidths(vm.Width)[0]).Render(title + "\n\n" + content)
}

func (r Renderer) renderDetailPane(vm ViewModel) string {
	title := r.styles.Label.Render("Details")
	content := r.styles.Subtle.Render("Select a worktree to inspect.")

	if len(vm.Worktrees) > 0 && vm.Selected >= 0 && vm.Selected < len(vm.Worktrees) {
		worktree := vm.Worktrees[vm.Selected]
		lines := []string{
			r.detailLine("path", worktree.Path),
			r.detailLine("branch", fallbackValue(worktree.Branch, "detached")),
			r.detailLine("commit", fallbackValue(worktree.Head, "unknown")),
			r.detailLine("current", yesNo(worktree.IsCurrent)),
		}
		if vm.Config.UI.ShowDirtyStatus {
			dirty := "unknown"
			if worktree.DirtyKnown {
				dirty = yesNo(worktree.IsDirty)
			}
			lines = append(lines, r.detailLine("dirty", dirty))
		}
		content = strings.Join(lines, "\n")
	}

	if vm.Mode == ModeCommand {
		commandPanel := r.styles.CommandBox.Render(
			r.styles.Label.Render("Command Mode") + "\n" +
				r.styles.Subtle.Render("Run a git command in the selected worktree.") + "\n\n" +
				vm.InputView,
		)
		content = content + "\n\n" + commandPanel
	}

	return r.styles.Panel.Width(panelWidths(vm.Width)[1]).Render(title + "\n\n" + content)
}

func (r Renderer) renderOutput(vm ViewModel) string {
	title := r.styles.Header.Render("Command Output")
	meta := r.styles.Subtle.Render(vm.LastCommand)
	body := r.styles.OutputBox.Render(vm.OutputView)
	footer := r.renderFooter(vm)
	return lipgloss.JoinVertical(lipgloss.Left, title, meta, body, footer)
}

func (r Renderer) renderHelp(vm ViewModel) string {
	lines := []string{
		r.styles.Header.Render("Help"),
		"",
		r.helpLine(r.keys.Up),
		r.helpLine(r.keys.Down),
		r.helpLine(r.keys.Enter),
		r.helpLine(r.keys.Refresh),
		r.helpLine(r.keys.Back),
		r.helpLine(r.keys.Help),
		r.helpLine(r.keys.Quit),
	}
	if vm.StatusMessage != "" {
		lines = append(lines, "", r.styles.Subtle.Render(vm.StatusMessage))
	}
	return r.styles.HelpBox.Render(strings.Join(lines, "\n"))
}

func (r Renderer) renderFooter(vm ViewModel) string {
	lines := []string{r.styles.Footer.Render(r.keys.ShortHelp())}
	if vm.StatusMessage != "" {
		lines = append(lines, r.styles.Subtle.Render(vm.StatusMessage))
	}
	if vm.ErrorMessage != "" {
		lines = append(lines, r.styles.Error.Render(vm.ErrorMessage))
	}
	return strings.Join(lines, "\n")
}

func (r Renderer) detailLine(label string, value string) string {
	return r.styles.Label.Render(label+":") + " " + r.styles.Value.Render(value)
}

func (r Renderer) helpLine(binding key.Binding) string {
	help := binding.Help()
	return fmt.Sprintf("%-12s %s", help.Key, help.Desc)
}

func panelWidths(width int) [2]int {
	if width <= 80 {
		return [2]int{36, 44}
	}
	left := width * 38 / 100
	right := width - left - 6
	if left < 30 {
		left = 30
	}
	if right < 40 {
		right = 40
	}
	return [2]int{left, right}
}

func listHeight(height int) int {
	if height <= 0 {
		return 10
	}
	value := height - 10
	if value < 6 {
		return 6
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func fallbackValue(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func compactPath(path string) string {
	path = filepath.Clean(path)

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if relative, relErr := filepath.Rel(home, path); relErr == nil && !strings.HasPrefix(relative, "..") {
			path = filepath.Join("~", relative)
		}
	}

	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) <= 4 {
		return path
	}

	return filepath.Join("...", filepath.Join(parts[len(parts)-4:]...))
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
