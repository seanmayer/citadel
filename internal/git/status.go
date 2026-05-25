package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errRemoteReferenceMissing = errors.New("upstream reference is unavailable")

type BranchStatus struct {
	Upstream       string
	HasUpstream    bool
	Ahead          int
	Behind         int
	MergedIntoBase bool
	RemoteExists   bool
	IsDirty        bool
	Error          string

	dirtyKnown       bool
	mergeKnown       bool
	aheadBehindKnown bool
}

func (s BranchStatus) DirtyKnown() bool {
	return s.dirtyKnown
}

func (s BranchStatus) MergeKnown() bool {
	return s.mergeKnown
}

func (s BranchStatus) AheadBehindKnown() bool {
	return s.aheadBehindKnown
}

func (s *Service) GetBranchStatus(ctx context.Context, worktreePath string, baseBranch string) (BranchStatus, error) {
	if baseBranch == "" {
		baseBranch = "origin/main"
	}

	status := BranchStatus{}
	problems := make([]string, 0, 3)

	dirty, err := s.IsDirty(ctx, worktreePath)
	if err != nil {
		problems = append(problems, err.Error())
	} else {
		status.IsDirty = dirty
		status.dirtyKnown = true
	}

	upstream, hasUpstream, err := s.GetUpstream(ctx, worktreePath)
	if err != nil {
		problems = append(problems, err.Error())
	} else if hasUpstream {
		status.Upstream = upstream
		status.HasUpstream = true
		status.RemoteExists = true
	}

	if status.HasUpstream {
		ahead, behind, err := s.GetAheadBehind(ctx, worktreePath)
		if err != nil {
			if errors.Is(err, errRemoteReferenceMissing) {
				status.RemoteExists = false
			}
			problems = append(problems, err.Error())
		} else {
			status.Ahead = ahead
			status.Behind = behind
			status.aheadBehindKnown = true
		}
	}

	merged, err := s.IsMergedIntoBase(ctx, worktreePath, baseBranch)
	if err != nil {
		problems = append(problems, err.Error())
	} else {
		status.MergedIntoBase = merged
		status.mergeKnown = true
	}

	if len(problems) > 0 {
		status.Error = strings.Join(problems, "; ")
		return status, errors.New(status.Error)
	}

	return status, nil
}

func (s *Service) IsDirty(ctx context.Context, worktreePath string) (bool, error) {
	stdout, stderr, exitCode, err := s.run(ctx, worktreePath, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("load dirty status for %q: %w", worktreePath, err)
	}
	if exitCode != 0 {
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if message == "" {
			message = "git status returned a non-zero exit status"
		}
		return false, fmt.Errorf("load dirty status for %q: %s", worktreePath, message)
	}

	return parseDirtyOutput(stdout), nil
}

func (s *Service) GetUpstream(ctx context.Context, worktreePath string) (string, bool, error) {
	stdout, stderr, exitCode, err := s.run(ctx, worktreePath, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return "", false, fmt.Errorf("load upstream for %q: %w", worktreePath, err)
	}
	if exitCode != 0 {
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if isMissingUpstreamMessage(message) {
			return "", false, nil
		}
		if message == "" {
			message = "git rev-parse returned a non-zero exit status"
		}
		return "", false, fmt.Errorf("load upstream for %q: %s", worktreePath, message)
	}

	upstream := strings.TrimSpace(stdout)
	if upstream == "" {
		return "", false, errors.New("git rev-parse returned an empty upstream")
	}

	return upstream, true, nil
}

func (s *Service) GetAheadBehind(ctx context.Context, worktreePath string) (ahead int, behind int, err error) {
	stdout, stderr, exitCode, runErr := s.run(ctx, worktreePath, "git", "rev-list", "--left-right", "--count", "HEAD...@{u}")
	if runErr != nil {
		return 0, 0, fmt.Errorf("load ahead/behind for %q: %w", worktreePath, runErr)
	}
	if exitCode != 0 {
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if message == "" {
			message = "git rev-list returned a non-zero exit status"
		}
		if isMissingRemoteReferenceMessage(message) {
			return 0, 0, fmt.Errorf("load ahead/behind for %q: %w: %s", worktreePath, errRemoteReferenceMissing, message)
		}
		return 0, 0, fmt.Errorf("load ahead/behind for %q: %s", worktreePath, message)
	}

	ahead, behind, err = parseAheadBehindOutput(stdout)
	if err != nil {
		return 0, 0, fmt.Errorf("parse ahead/behind for %q: %w", worktreePath, err)
	}

	return ahead, behind, nil
}

func (s *Service) IsMergedIntoBase(ctx context.Context, worktreePath string, baseBranch string) (bool, error) {
	if baseBranch == "" {
		baseBranch = "origin/main"
	}

	stdout, stderr, exitCode, err := s.run(ctx, worktreePath, "git", "merge-base", "--is-ancestor", "HEAD", baseBranch)
	if err != nil {
		return false, fmt.Errorf("check merge status for %q against %q: %w", worktreePath, baseBranch, err)
	}

	switch exitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if message == "" {
			message = fmt.Sprintf("git merge-base returned exit code %d", exitCode)
		}
		return false, fmt.Errorf("check merge status for %q against %q: %s", worktreePath, baseBranch, message)
	}
}

func parseAheadBehindOutput(output string) (ahead int, behind int, err error) {
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) == 0 {
		return 0, 0, errors.New("ahead/behind output is empty")
	}
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("expected 2 fields, got %d", len(fields))
	}

	ahead, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parse ahead count: %w", err)
	}

	behind, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse behind count: %w", err)
	}

	return ahead, behind, nil
}

func parseDirtyOutput(output string) bool {
	return strings.TrimSpace(output) != ""
}

func isMissingUpstreamMessage(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "no upstream configured") ||
		strings.Contains(message, "no upstream") ||
		strings.Contains(message, "does not point to a branch")
}

func isMissingRemoteReferenceMessage(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "unknown revision") ||
		strings.Contains(message, "bad revision") ||
		strings.Contains(message, "no such branch") ||
		strings.Contains(message, "no such ref") ||
		strings.Contains(message, "ambiguous argument")
}
