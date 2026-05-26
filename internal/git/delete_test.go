package git

import (
	"context"
	"strings"
	"testing"
)

func TestDeleteWorktreeRemovesWorktreeAndBranch(t *testing.T) {
	t.Parallel()

	runner := newFakeRunner()
	runner.stub("/repo", "git", fakeResult{}, "worktree", "remove", "/repo-feature")
	runner.stub("/repo", "git", fakeResult{
		stdout: "Deleted branch feature/demo (was abc1234)\n",
	}, "branch", "-d", "feature/demo")

	service := NewService(runner)
	output, err := service.DeleteWorktree(context.Background(), "/repo", Worktree{
		Path:   "/repo-feature",
		Branch: "feature/demo",
	}, DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteWorktree() error = %v", err)
	}

	if !strings.Contains(output, "git worktree remove /repo-feature") {
		t.Fatalf("output = %q, want worktree removal transcript", output)
	}
	if !strings.Contains(output, "git branch -d feature/demo") {
		t.Fatalf("output = %q, want branch deletion transcript", output)
	}
}

func TestDeleteWorktreeUsesForceFlagsWhenRequested(t *testing.T) {
	t.Parallel()

	runner := newFakeRunner()
	runner.stub("/repo", "git", fakeResult{}, "worktree", "remove", "--force", "/repo-feature")
	runner.stub("/repo", "git", fakeResult{}, "branch", "-D", "feature/demo")

	service := NewService(runner)
	output, err := service.DeleteWorktree(context.Background(), "/repo", Worktree{
		Path:   "/repo-feature",
		Branch: "feature/demo",
	}, DeleteOptions{
		ForceRemove: true,
		ForceBranch: true,
	})
	if err != nil {
		t.Fatalf("DeleteWorktree() error = %v, output = %q", err, output)
	}

	if !strings.Contains(output, "git worktree remove --force /repo-feature") {
		t.Fatalf("output = %q, want forced worktree removal transcript", output)
	}
	if !strings.Contains(output, "git branch -D feature/demo") {
		t.Fatalf("output = %q, want forced branch deletion transcript", output)
	}
}
