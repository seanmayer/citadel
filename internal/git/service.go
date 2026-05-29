package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path       string
	Branch     string
	CommitHash string
	IsCurrent  bool
	IsDetached bool
	IsBare     bool
	Locked     bool
	Prunable   bool
	Status     BranchStatus
}

func (w Worktree) HasNamedBranch() bool {
	return w.Branch != "" && !w.IsDetached
}

func (w Worktree) CanCreateBranch() bool {
	return !w.IsBare && !w.HasNamedBranch()
}

func (w Worktree) BranchDisplay() string {
	switch {
	case w.HasNamedBranch():
		return w.Branch
	case w.IsDetached:
		return "detached"
	default:
		return "none"
	}
}

type RefreshOptions struct {
	BaseBranch string
	Fetch      bool
}

type DeleteOptions struct {
	ForceRemove bool
	ForceBranch bool
}

type CommandRunner interface {
	Run(dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error)
}

type execRunner struct{}

func (execRunner) Run(dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err == nil {
		return stdout, stderr, 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), nil
	}

	return stdout, stderr, -1, err
}

type Service struct {
	runner CommandRunner
}

func NewService(runner CommandRunner) *Service {
	if runner == nil {
		runner = execRunner{}
	}

	return &Service{runner: runner}
}

func (s *Service) DetectRepoRoot(ctx context.Context, cwd string) (string, error) {
	stdout, stderr, exitCode, err := s.run(ctx, cwd, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("detect repository root: %w", err)
	}
	if exitCode != 0 {
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if message == "" {
			message = "git rev-parse returned a non-zero exit status"
		}
		return "", fmt.Errorf("not inside a Git repository: %s", message)
	}

	root := strings.TrimSpace(stdout)
	if root == "" {
		return "", errors.New("git rev-parse returned an empty repository root")
	}

	return normalizePath(root), nil
}

func (s *Service) ListWorktrees(ctx context.Context, repoRoot string, currentWorktreePath string, options RefreshOptions) ([]Worktree, error) {
	if options.BaseBranch == "" {
		options.BaseBranch = "origin/main"
	}

	if options.Fetch {
		if err := s.FetchRemoteState(ctx, repoRoot); err != nil {
			return nil, err
		}
	}

	stdout, stderr, exitCode, err := s.run(ctx, repoRoot, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("load git worktrees: %w", err)
	}
	if exitCode != 0 {
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if message == "" {
			message = "git worktree list returned a non-zero exit status"
		}
		return nil, fmt.Errorf("load git worktrees: %s", message)
	}

	worktrees, err := ParseWorktreesPorcelain(stdout)
	if err != nil {
		return nil, fmt.Errorf("parse git worktrees: %w", err)
	}

	for i := range worktrees {
		worktrees[i].Path = normalizePath(worktrees[i].Path)
		worktrees[i].IsCurrent = samePath(worktrees[i].Path, currentWorktreePath)
	}

	return s.EnrichWorktrees(ctx, worktrees, options.BaseBranch), nil
}

func (s *Service) EnrichWorktrees(ctx context.Context, worktrees []Worktree, baseBranch string) []Worktree {
	if baseBranch == "" {
		baseBranch = "origin/main"
	}

	enriched := make([]Worktree, len(worktrees))
	copy(enriched, worktrees)

	for i := range enriched {
		if enriched[i].IsBare {
			continue
		}

		status, err := s.GetBranchStatus(ctx, enriched[i].Path, baseBranch)
		enriched[i].Status = status
		if err != nil && enriched[i].Status.Error == "" {
			enriched[i].Status.Error = err.Error()
		}
	}

	return enriched
}

func (s *Service) FetchRemoteState(ctx context.Context, repoRoot string) error {
	stdout, stderr, exitCode, err := s.run(ctx, repoRoot, "git", "fetch", "--prune")
	if err != nil {
		return fmt.Errorf("fetch remote state: %w", err)
	}
	if exitCode != 0 {
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if message == "" {
			message = "git fetch returned a non-zero exit status"
		}
		return fmt.Errorf("fetch remote state: %s", message)
	}
	return nil
}

func (s *Service) OpenPullRequest(ctx context.Context, worktreePath string, branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "branch name is empty", errors.New("branch name is empty")
	}

	stdout, stderr, exitCode, err := s.run(ctx, worktreePath, "gh", "pr", "view", branch, "--web")
	text := strings.TrimRight(combinedText(stdout, stderr), "\n")
	if text == "" && err == nil && exitCode == 0 {
		text = "(no output)"
	}

	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("open pull request for branch %q in %q: %w", branch, worktreePath, err)
	}
	if exitCode != 0 {
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if message == "" {
			message = fmt.Sprintf("gh pr view exited with status %d", exitCode)
		}
		if text == "" {
			text = message
		}
		return text, fmt.Errorf("open pull request for branch %q in %q: %s", branch, worktreePath, message)
	}

	return text, nil
}

func (s *Service) DeleteWorktree(ctx context.Context, repoRoot string, worktree Worktree, options DeleteOptions) (string, error) {
	transcript := make([]string, 0, 2)

	removeArgs := []string{"worktree", "remove"}
	if options.ForceRemove {
		removeArgs = append(removeArgs, "--force")
	}
	removeArgs = append(removeArgs, worktree.Path)

	removeOutput, err := s.ExecuteGitCommand(ctx, repoRoot, removeArgs)
	transcript = append(transcript, formatCommandTranscript(displayCommand(removeArgs), removeOutput))
	if err != nil {
		return strings.Join(transcript, "\n\n"), err
	}

	if worktree.HasNamedBranch() {
		deleteFlag := "-d"
		if options.ForceBranch {
			deleteFlag = "-D"
		}

		branchArgs := []string{"branch", deleteFlag, worktree.Branch}
		branchOutput, branchErr := s.ExecuteGitCommand(ctx, repoRoot, branchArgs)
		transcript = append(transcript, formatCommandTranscript(displayCommand(branchArgs), branchOutput))
		if branchErr != nil {
			return strings.Join(transcript, "\n\n"), branchErr
		}
	}

	return strings.Join(transcript, "\n\n"), nil
}

func (s *Service) ExecuteGitCommand(ctx context.Context, worktreePath string, args []string) (string, error) {
	stdout, stderr, exitCode, err := s.run(ctx, worktreePath, "git", args...)
	text := strings.TrimRight(combinedText(stdout, stderr), "\n")
	if text == "" && err == nil && exitCode == 0 {
		text = "(no output)"
	}

	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("%s failed: %w", displayCommand(args), err)
	}

	if exitCode != 0 {
		if text == "" {
			text = fmt.Sprintf("%s exited with status %d", displayCommand(args), exitCode)
		}
		return text, fmt.Errorf("%s failed with exit code %d", displayCommand(args), exitCode)
	}

	return text, nil
}

func (s *Service) run(ctx context.Context, dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", "", -1, err
		}
	}

	stdout, stderr, exitCode, err = s.runner.Run(dir, name, args...)
	if err != nil {
		return stdout, stderr, exitCode, err
	}

	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return stdout, stderr, exitCode, err
		}
	}

	return stdout, stderr, exitCode, nil
}

func ParseWorktreesPorcelain(output string) ([]Worktree, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	worktrees := make([]Worktree, 0)
	var current *Worktree

	flush := func() error {
		if current == nil {
			return nil
		}
		if current.Path == "" {
			return errors.New("encountered worktree entry without a path")
		}
		worktrees = append(worktrees, *current)
		current = nil
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "worktree "):
			if err := flush(); err != nil {
				return nil, err
			}
			current = &Worktree{
				Path: strings.TrimSpace(strings.TrimPrefix(line, "worktree ")),
			}
		case current == nil:
			return nil, fmt.Errorf("unexpected line before worktree header: %q", line)
		case strings.HasPrefix(line, "HEAD "):
			current.CommitHash = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			current.Branch = shortBranch(ref)
		case line == "detached":
			current.IsDetached = true
			current.Branch = "detached"
		case line == "bare":
			current.IsBare = true
		case strings.HasPrefix(line, "locked"):
			current.Locked = true
		case strings.HasPrefix(line, "prunable"):
			current.Prunable = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan porcelain output: %w", err)
	}

	if err := flush(); err != nil {
		return nil, err
	}

	return worktrees, nil
}

func ShortHash(hash string) string {
	if len(hash) <= 8 {
		return hash
	}
	return hash[:8]
}

func shortBranch(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}

	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}

	return filepath.Clean(path)
}

func samePath(a string, b string) bool {
	if a == "" || b == "" {
		return false
	}

	return normalizePath(a) == normalizePath(b)
}

func combinedText(stdout string, stderr string) string {
	stdout = strings.TrimRight(stdout, "\n")
	stderr = strings.TrimRight(stderr, "\n")

	switch {
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n" + stderr
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func displayCommand(args []string) string {
	if len(args) == 0 {
		return "git"
	}

	return "git " + strings.Join(args, " ")
}

func formatCommandTranscript(command string, output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		output = "(no output)"
	}

	return command + "\n" + output
}
