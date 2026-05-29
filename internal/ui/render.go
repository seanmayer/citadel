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
	ModeSplash  Mode = "splash"
	ModeList    Mode = "list"
	ModeCommand Mode = "command"
	ModeCreate  Mode = "create-branch"
	ModeCommit  Mode = "commit"
	ModeDelete  Mode = "delete-worktree"
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
	BranchInput   string
	CommitInput   string
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
	case ModeSplash:
		return r.styles.Root.Render(r.renderSplash(vm))
	case ModeOutput:
		return r.styles.Root.Render(r.renderOutput(vm))
	case ModeHelp:
		return r.styles.Root.Render(r.renderHelp(vm))
	default:
		return r.styles.Root.Render(r.renderMain(vm))
	}
}

func (r Renderer) renderMain(vm ViewModel) string {
	header := r.styles.Header.Render("citadel") + "\n" + r.styles.Subtle.Render(vm.RepoRoot)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, r.renderListPane(vm), r.renderDetailPane(vm))
	footer := r.renderFooter(vm)
	return lipgloss.JoinVertical(lipgloss.Left, header, panes, footer)
}

func (r Renderer) renderSplash(vm ViewModel) string {
	width := max(vm.Width-4, 60)
	height := max(vm.Height-2, 16)

	lines := []string{
		r.styles.SplashMark.Render("citadel"),
		r.styles.SplashTag.Render("minimal git worktree command center"),
	}

	if vm.RepoRoot != "" {
		lines = append(lines, "", r.styles.Subtle.Render(compactPath(vm.RepoRoot)))
	}

	status := r.styles.Subtle.Render("loading worktrees")
	if vm.ErrorMessage != "" {
		status = r.styles.Error.Render(vm.ErrorMessage)
	}

	lines = append(lines, "", status, r.styles.Footer.Render("press any key to continue"))

	content := r.styles.SplashBox.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
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
			parts := make([]string, 0, 4)
			if vm.Config.UI.ShowBranch {
				parts = append(parts, worktree.BranchDisplay())
			}
			parts = append(parts, displayPath(vm.RepoRoot, worktree.Path))
			if vm.Config.UI.ShowCommitHash && worktree.CommitHash != "" {
				parts = append(parts, git.ShortHash(worktree.CommitHash))
			}

			badges := r.statusBadges(worktree, vm.Config)
			if len(badges) > 0 {
				parts = append(parts, strings.Join(badges, " "))
			}

			row := strings.Join(parts, "  ")
			if worktree.IsCurrent {
				row += " " + r.styles.CurrentBadge.Render("current")
			}

			if i == vm.Selected {
				row = r.styles.Selected.Render("> " + row)
			} else {
				row = "  " + row
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
		upstream := "local-only"
		if worktree.Status.HasUpstream {
			upstream = worktree.Status.Upstream
		}

		ahead := "n/a"
		behind := "n/a"
		if worktree.Status.HasUpstream {
			if worktree.Status.RemoteExists && worktree.Status.AheadBehindKnown() {
				ahead = fmt.Sprintf("%d", worktree.Status.Ahead)
				behind = fmt.Sprintf("%d", worktree.Status.Behind)
			} else {
				ahead = "unknown"
				behind = "unknown"
			}
		}

		merged := "unknown"
		if worktree.Status.MergeKnown() {
			merged = yesNo(worktree.Status.MergedIntoBase)
		}

		dirty := "unknown"
		if worktree.Status.DirtyKnown() {
			dirty = yesNo(worktree.Status.IsDirty)
		}

		lines := []string{
			r.detailLine("Path", worktree.Path),
			r.detailLine("Branch", worktree.BranchDisplay()),
			r.detailLine("Commit", fallbackValue(worktree.CommitHash, "unknown")),
			r.detailLine("Current worktree", yesNo(worktree.IsCurrent)),
			r.detailLine("Upstream", upstream),
			r.detailLine("Ahead", ahead),
			r.detailLine("Behind", behind),
			r.detailLine("Merged into base", merged),
			r.detailLine("Dirty", dirty),
			r.detailLine("Status error", fallbackValue(worktree.Status.Error, "none")),
		}
		content = strings.Join(lines, "\n")
	}

	if vm.Mode == ModeList && len(vm.Worktrees) > 0 && vm.Selected >= 0 && vm.Selected < len(vm.Worktrees) {
		actionPanel := r.styles.CommandBox.Render(r.renderActionList(vm, vm.Worktrees[vm.Selected]))
		content = content + "\n\n" + actionPanel
	}

	if vm.Mode == ModeCommand {
		commandPanel := r.styles.CommandBox.Render(
			r.styles.Label.Render("Command Mode") + "\n" +
				r.styles.Subtle.Render("Run a git command or the built-in hot push workflow in the selected worktree.") + "\n\n" +
				vm.InputView,
		)
		content = content + "\n\n" + commandPanel
	}
	if vm.Mode == ModeCreate {
		branchPanel := r.styles.CommandBox.Render(
			r.styles.Label.Render("Create Branch") + "\n" +
				r.styles.Subtle.Render("Type a branch name for the selected worktree.") + "\n\n" +
				vm.BranchInput,
		)
		content = content + "\n\n" + branchPanel
	}
	if vm.Mode == ModeCommit {
		commitPanel := r.styles.CommandBox.Render(
			r.styles.Label.Render("Commit Changes") + "\n" +
				r.styles.Subtle.Render("Type a commit message for the selected worktree.") + "\n\n" +
				vm.CommitInput,
		)
		content = content + "\n\n" + commitPanel
	}
	if vm.Mode == ModeDelete && len(vm.Worktrees) > 0 && vm.Selected >= 0 && vm.Selected < len(vm.Worktrees) {
		deletePanel := r.styles.CommandBox.Render(r.renderDeleteConfirmation(vm, vm.Worktrees[vm.Selected]))
		content = content + "\n\n" + deletePanel
	}

	return r.styles.Panel.Width(panelWidths(vm.Width)[1]).Render(title + "\n\n" + content)
}

func (r Renderer) renderOutput(vm ViewModel) string {
	title := r.styles.Header.Render("Command Output")
	meta := r.styles.Subtle.Render(vm.LastCommand)
	body := r.styles.OutputBox.Render(vm.OutputView)
	cta := r.renderOutputContinue(vm)
	footer := r.renderFooter(vm)
	return lipgloss.JoinVertical(lipgloss.Left, title, meta, body, cta, footer)
}

func (r Renderer) renderHelp(vm ViewModel) string {
	lines := []string{
		r.styles.Header.Render("Help"),
		"",
		r.helpLine(r.keys.Up),
		r.helpLine(r.keys.Down),
		r.helpLine(r.keys.OpenEditor),
		r.helpLine(r.keys.OpenPullRequest),
		r.helpLine(r.keys.StageAll),
		r.helpLine(r.keys.HotPush),
		r.helpLine(r.keys.Commit),
		r.helpLine(r.keys.Create),
		r.helpLine(r.keys.Delete),
		r.helpLine(r.keys.Enter),
		r.helpLine(r.keys.Continue),
		r.helpLine(r.keys.Refresh),
		r.helpLine(r.keys.FetchRefresh),
		r.helpLine(r.keys.Back),
		r.helpLine(r.keys.Help),
		r.helpLine(r.keys.Quit),
	}
	if vm.Mode == ModeDelete {
		lines = append(lines, fmt.Sprintf("%-12s %s", "y", "confirm delete"))
	}
	if vm.StatusMessage != "" {
		lines = append(lines, "", r.styles.Subtle.Render(vm.StatusMessage))
	}
	return r.styles.HelpBox.Render(strings.Join(lines, "\n"))
}

func (r Renderer) renderFooter(vm ViewModel) string {
	help := r.keys.ShortHelp()
	if vm.Mode == ModeOutput {
		help = r.keys.OutputHelp()
	}
	if vm.Mode == ModeDelete {
		help = help + "  y confirm delete"
	}
	lines := []string{r.styles.Footer.Render(help)}
	if vm.StatusMessage != "" {
		lines = append(lines, r.styles.Subtle.Render(vm.StatusMessage))
	}
	if vm.ErrorMessage != "" {
		lines = append(lines, r.styles.Error.Render(vm.ErrorMessage))
	}
	return strings.Join(lines, "\n")
}

func (r Renderer) renderActionList(vm ViewModel, worktree git.Worktree) string {
	lines := []string{
		r.styles.Label.Render("Actions"),
		r.styles.Subtle.Render(fmt.Sprintf("Press %s to open terminal command mode.", r.keys.Enter.Help().Key)),
	}

	if worktree.HasNamedBranch() {
		lines = append(lines, r.styles.Subtle.Render(fmt.Sprintf("Press %s to open the pull request for this branch, if one exists.", r.keys.OpenPullRequest.Help().Key)))
	}

	if worktree.IsBare {
		lines = append(lines, r.styles.Subtle.Render("Bare worktrees cannot open an editor, stage files, or create commits."))
		return strings.Join(lines, "\n")
	}

	lines = append(lines,
		r.styles.Subtle.Render(fmt.Sprintf("Press %s to open this worktree in the editor.", r.keys.OpenEditor.Help().Key)),
		r.styles.Subtle.Render(fmt.Sprintf("Press %s to stage all files with git add .", r.keys.StageAll.Help().Key)),
		r.styles.Subtle.Render(fmt.Sprintf("Press %s to fetch, pull, add, commit, and push with hot push.", r.keys.HotPush.Help().Key)),
		r.styles.Subtle.Render(fmt.Sprintf("Press %s to commit files with a message.", r.keys.Commit.Help().Key)),
	)

	if worktree.CanCreateBranch() {
		lines = append(lines, r.styles.Subtle.Render(fmt.Sprintf("Press %s to create a branch for this worktree.", r.keys.Create.Help().Key)))
	}

	switch {
	case worktree.IsCurrent:
		lines = append(lines, r.styles.Subtle.Render("Current worktree cannot be deleted while citadel is running in it."))
	default:
		lines = append(lines, r.styles.Subtle.Render(fmt.Sprintf("Press %s to delete this worktree.", r.keys.Delete.Help().Key)))
	}

	return strings.Join(lines, "\n")
}

func (r Renderer) renderOutputContinue(vm ViewModel) string {
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		r.styles.ContinueTitle.Render("SPACE TO CONTINUE"),
		r.styles.ContinueHint.Render("Return to the worktree list"),
	)
	box := r.styles.ContinueBox.Render(content)

	availableWidth := vm.Width - 4
	if availableWidth <= lipgloss.Width(box) {
		return box
	}

	return lipgloss.PlaceHorizontal(availableWidth, lipgloss.Center, box)
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

func displayPath(repoRoot string, worktreePath string) string {
	if repoRoot != "" {
		if relative, err := filepath.Rel(repoRoot, worktreePath); err == nil && relative != "" {
			return relative
		}
	}

	return compactPath(worktreePath)
}

func (r Renderer) renderDeleteConfirmation(vm ViewModel, worktree git.Worktree) string {
	lines := []string{
		r.styles.Label.Render("Delete Worktree"),
		r.styles.Subtle.Render("Remove the selected worktree and delete its local branch if it has one."),
		"",
		r.detailLine("Path", worktree.Path),
		r.detailLine("Branch", worktree.BranchDisplay()),
	}

	warnings := deleteWarnings(worktree, vm.Config.Git.BaseBranch)
	if len(warnings) > 0 {
		lines = append(lines, "")
		for _, warning := range warnings {
			lines = append(lines, r.styles.Error.Render("Warning: "+warning))
		}
	} else {
		lines = append(lines, "", r.styles.Subtle.Render("This worktree is ready to delete."))
	}

	lines = append(lines, "", r.styles.Subtle.Render("Press y to delete or esc to cancel."))
	return strings.Join(lines, "\n")
}

func deleteWarnings(worktree git.Worktree, baseBranch string) []string {
	warnings := make([]string, 0, 3)
	if baseBranch == "" {
		baseBranch = "origin/main"
	}

	switch {
	case worktree.HasNamedBranch() && worktree.Status.MergeKnown() && !worktree.Status.MergedIntoBase:
		warnings = append(warnings, fmt.Sprintf("This worktree branch is not merged into %s.", baseBranch))
	case !worktree.HasNamedBranch() && worktree.Status.MergeKnown() && !worktree.Status.MergedIntoBase:
		warnings = append(warnings, fmt.Sprintf("This worktree is not merged into %s.", baseBranch))
	case worktree.HasNamedBranch() && !worktree.Status.MergeKnown():
		warnings = append(warnings, fmt.Sprintf("Merge status for this worktree branch is unknown against %s.", baseBranch))
	case !worktree.HasNamedBranch() && !worktree.Status.MergeKnown():
		warnings = append(warnings, fmt.Sprintf("Merge status for this worktree is unknown against %s.", baseBranch))
	}

	if worktree.Status.DirtyKnown() && worktree.Status.IsDirty {
		warnings = append(warnings, "Uncommitted changes in this worktree will be lost.")
	}
	if worktree.Locked {
		warnings = append(warnings, "This worktree is locked and will be force removed.")
	}

	return warnings
}

func (r Renderer) statusBadges(worktree git.Worktree, cfg config.Config) []string {
	badges := make([]string, 0, 5)

	if cfg.Git.ShowRemoteStatus {
		switch {
		case !worktree.Status.HasUpstream:
			badges = append(badges, "local")
		case worktree.Status.RemoteExists && worktree.Status.AheadBehindKnown():
			badges = append(badges, fmt.Sprintf("↑%d", worktree.Status.Ahead))
			badges = append(badges, fmt.Sprintf("↓%d", worktree.Status.Behind))
		}
	}

	if cfg.Git.ShowDirtyStatus && worktree.Status.DirtyKnown() {
		if worktree.Status.IsDirty {
			badges = append(badges, "dirty")
		} else {
			badges = append(badges, "clean")
		}
	}

	if cfg.Git.ShowMergeStatus && worktree.Status.MergeKnown() {
		if worktree.Status.MergedIntoBase {
			badges = append(badges, "merged")
		} else {
			badges = append(badges, "not merged")
		}
	}

	if worktree.Status.Error != "" {
		badges = append(badges, "error")
	}

	return badges
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
