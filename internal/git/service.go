package git

import (
	"bufio"
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
	Head       string
	IsCurrent  bool
	IsDetached bool
	IsBare     bool
	IsDirty    bool
	DirtyKnown bool
	Locked     bool
	Prunable   bool
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

type Runner interface {
	CombinedOutput(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

type osRunner struct{}

func (osRunner) CombinedOutput(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

type Service struct {
	runner Runner
}

func NewService(runner Runner) *Service {
	if runner == nil {
		runner = osRunner{}
	}

	return &Service{runner: runner}
}

func (s *Service) DetectRepoRoot(ctx context.Context, cwd string) (string, error) {
	output, err := s.runner.CombinedOutput(ctx, cwd, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("not inside a Git repository: %s", message)
	}

	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", errors.New("git rev-parse returned an empty repository root")
	}

	return normalizePath(root), nil
}

func (s *Service) ListWorktrees(ctx context.Context, repoRoot string, currentWorktreePath string, showDirty bool) ([]Worktree, error) {
	output, err := s.runner.CombinedOutput(ctx, repoRoot, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("load git worktrees: %w", err)
	}

	worktrees, err := ParseWorktreesPorcelain(string(output))
	if err != nil {
		return nil, fmt.Errorf("parse git worktrees: %w", err)
	}

	for i := range worktrees {
		worktrees[i].Path = normalizePath(worktrees[i].Path)
		worktrees[i].IsCurrent = samePath(worktrees[i].Path, currentWorktreePath)

		if !showDirty || worktrees[i].IsBare {
			continue
		}

		dirty, err := s.isDirty(ctx, worktrees[i].Path)
		if err != nil {
			return nil, err
		}

		worktrees[i].IsDirty = dirty
		worktrees[i].DirtyKnown = true
	}

	return worktrees, nil
}

func (s *Service) ExecuteGitCommand(ctx context.Context, worktreePath string, args []string) (string, error) {
	output, err := s.runner.CombinedOutput(ctx, worktreePath, "git", args...)
	text := strings.TrimRight(string(output), "\n")
	if text == "" && err == nil {
		text = "(no output)"
	}

	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("%s failed: %w", displayCommand(args), err)
	}

	return text, nil
}

func (s *Service) isDirty(ctx context.Context, worktreePath string) (bool, error) {
	output, err := s.runner.CombinedOutput(ctx, worktreePath, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("load dirty status for %q: %w", worktreePath, err)
	}

	return strings.TrimSpace(string(output)) != "", nil
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
			current.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
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

func displayCommand(args []string) string {
	if len(args) == 0 {
		return "git"
	}

	return "git " + strings.Join(args, " ")
}
