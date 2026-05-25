package git

import "testing"

func TestParseWorktreesPorcelain(t *testing.T) {
	t.Parallel()

	output := `worktree /tmp/repo
HEAD 0123456789abcdef
branch refs/heads/main

worktree /tmp/repo-feature
HEAD fedcba9876543210
branch refs/heads/feature/demo
locked

worktree /tmp/repo-detached
HEAD aabbccddeeff0011
detached
prunable
`

	worktrees, err := ParseWorktreesPorcelain(output)
	if err != nil {
		t.Fatalf("ParseWorktreesPorcelain() error = %v", err)
	}

	if len(worktrees) != 3 {
		t.Fatalf("len(worktrees) = %d, want 3", len(worktrees))
	}

	if worktrees[0].Path != "/tmp/repo" {
		t.Fatalf("worktrees[0].Path = %q, want /tmp/repo", worktrees[0].Path)
	}
	if worktrees[0].CommitHash != "0123456789abcdef" {
		t.Fatalf("worktrees[0].CommitHash = %q, want 0123456789abcdef", worktrees[0].CommitHash)
	}
	if worktrees[0].Branch != "main" {
		t.Fatalf("worktrees[0].Branch = %q, want main", worktrees[0].Branch)
	}
	if worktrees[1].Branch != "feature/demo" {
		t.Fatalf("worktrees[1].Branch = %q, want feature/demo", worktrees[1].Branch)
	}
	if !worktrees[1].Locked {
		t.Fatalf("worktrees[1].Locked = false, want true")
	}
	if !worktrees[2].IsDetached {
		t.Fatalf("worktrees[2].IsDetached = false, want true")
	}
	if worktrees[2].Branch != "detached" {
		t.Fatalf("worktrees[2].Branch = %q, want detached", worktrees[2].Branch)
	}
	if !worktrees[2].Prunable {
		t.Fatalf("worktrees[2].Prunable = false, want true")
	}
}
