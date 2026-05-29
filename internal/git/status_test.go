package git

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type fakeRunner struct {
	results map[string]fakeResult
	calls   []string
}

type fakeResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		results: make(map[string]fakeResult),
	}
}

func (f *fakeRunner) stub(dir string, name string, result fakeResult, args ...string) {
	f.results[f.key(dir, name, args...)] = result
}

func (f *fakeRunner) Run(dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	f.calls = append(f.calls, f.key(dir, name, args...))
	result, ok := f.results[f.key(dir, name, args...)]
	if !ok {
		return "", "", -1, fmt.Errorf("unexpected command: dir=%q name=%q args=%q", dir, name, strings.Join(args, " "))
	}
	return result.stdout, result.stderr, result.exitCode, result.err
}

func (f *fakeRunner) key(dir string, name string, args ...string) string {
	return dir + "|" + name + "|" + strings.Join(args, "\x00")
}

func TestParseAheadBehindOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		output     string
		wantAhead  int
		wantBehind int
		wantErr    bool
	}{
		{name: "tab separated", output: "2\t5\n", wantAhead: 2, wantBehind: 5},
		{name: "space separated", output: "0 0\n", wantAhead: 0, wantBehind: 0},
		{name: "invalid output", output: "nope", wantErr: true},
		{name: "empty output", output: "", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ahead, behind, err := parseAheadBehindOutput(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseAheadBehindOutput(%q) error = nil, want error", tt.output)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseAheadBehindOutput(%q) error = %v", tt.output, err)
			}
			if ahead != tt.wantAhead || behind != tt.wantBehind {
				t.Fatalf("parseAheadBehindOutput(%q) = (%d, %d), want (%d, %d)", tt.output, ahead, behind, tt.wantAhead, tt.wantBehind)
			}
		})
	}
}

func TestGetUpstream(t *testing.T) {
	t.Parallel()

	t.Run("valid upstream", func(t *testing.T) {
		t.Parallel()

		runner := newFakeRunner()
		runner.stub("/repo", "git", fakeResult{stdout: "origin/feature/test\n"}, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")

		service := NewService(runner)
		upstream, hasUpstream, err := service.GetUpstream(context.Background(), "/repo")
		if err != nil {
			t.Fatalf("GetUpstream() error = %v", err)
		}
		if !hasUpstream {
			t.Fatalf("GetUpstream() hasUpstream = false, want true")
		}
		if upstream != "origin/feature/test" {
			t.Fatalf("GetUpstream() upstream = %q, want %q", upstream, "origin/feature/test")
		}
	})

	t.Run("missing upstream is local only", func(t *testing.T) {
		t.Parallel()

		runner := newFakeRunner()
		runner.stub("/repo", "git", fakeResult{
			stderr:   "fatal: no upstream configured for branch 'feature/test'\n",
			exitCode: 128,
		}, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")

		service := NewService(runner)
		upstream, hasUpstream, err := service.GetUpstream(context.Background(), "/repo")
		if err != nil {
			t.Fatalf("GetUpstream() error = %v", err)
		}
		if hasUpstream {
			t.Fatalf("GetUpstream() hasUpstream = true, want false")
		}
		if upstream != "" {
			t.Fatalf("GetUpstream() upstream = %q, want empty string", upstream)
		}
	})
}

func TestOpenPullRequest(t *testing.T) {
	t.Parallel()

	t.Run("opens pull request in browser", func(t *testing.T) {
		t.Parallel()

		runner := newFakeRunner()
		runner.stub("/repo", "gh", fakeResult{exitCode: 0}, "pr", "view", "feature/test", "--web")

		service := NewService(runner)
		output, err := service.OpenPullRequest(context.Background(), "/repo", "feature/test")
		if err != nil {
			t.Fatalf("OpenPullRequest() error = %v", err)
		}
		if output != "(no output)" {
			t.Fatalf("output = %q, want %q", output, "(no output)")
		}
	})

	t.Run("returns gh command output on failure", func(t *testing.T) {
		t.Parallel()

		runner := newFakeRunner()
		runner.stub("/repo", "gh", fakeResult{
			stderr:   "no pull requests found for branch \"feature/test\"\n",
			exitCode: 1,
		}, "pr", "view", "feature/test", "--web")

		service := NewService(runner)
		output, err := service.OpenPullRequest(context.Background(), "/repo", "feature/test")
		if err == nil {
			t.Fatal("OpenPullRequest() error = nil, want error")
		}
		if output != "no pull requests found for branch \"feature/test\"" {
			t.Fatalf("output = %q, want gh error output", output)
		}
		if !strings.Contains(err.Error(), "no pull requests found") {
			t.Fatalf("error = %q, want gh error context", err)
		}
	})
}

func TestParseDirtyOutput(t *testing.T) {
	t.Parallel()

	if parseDirtyOutput("") {
		t.Fatalf("parseDirtyOutput(\"\") = true, want false")
	}

	if !parseDirtyOutput(" M README.md\n") {
		t.Fatalf("parseDirtyOutput(non-empty) = false, want true")
	}
}

func TestIsMergedIntoBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		result    fakeResult
		want      bool
		wantError bool
	}{
		{name: "merged", result: fakeResult{exitCode: 0}, want: true},
		{name: "not merged", result: fakeResult{exitCode: 1}, want: false},
		{name: "command error", result: fakeResult{stderr: "fatal: bad revision 'origin/main'\n", exitCode: 128}, wantError: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runner := newFakeRunner()
			runner.stub("/repo", "git", tt.result, "merge-base", "--is-ancestor", "HEAD", "origin/main")

			service := NewService(runner)
			merged, err := service.IsMergedIntoBase(context.Background(), "/repo", "origin/main")
			if tt.wantError {
				if err == nil {
					t.Fatalf("IsMergedIntoBase() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("IsMergedIntoBase() error = %v", err)
			}
			if merged != tt.want {
				t.Fatalf("IsMergedIntoBase() = %t, want %t", merged, tt.want)
			}
		})
	}
}

func TestEnrichWorktreesKeepsGoingOnStatusError(t *testing.T) {
	t.Parallel()

	runner := newFakeRunner()

	runner.stub("/repo-main", "git", fakeResult{stdout: ""}, "status", "--porcelain")
	runner.stub("/repo-main", "git", fakeResult{stdout: "origin/main\n"}, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	runner.stub("/repo-main", "git", fakeResult{stdout: "0\t0\n"}, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	runner.stub("/repo-main", "git", fakeResult{exitCode: 0}, "merge-base", "--is-ancestor", "HEAD", "origin/main")

	runner.stub("/repo-feature", "git", fakeResult{
		stderr:   "fatal: status unavailable\n",
		exitCode: 128,
	}, "status", "--porcelain")
	runner.stub("/repo-feature", "git", fakeResult{
		stderr:   "fatal: no upstream configured for branch 'feature/test'\n",
		exitCode: 128,
	}, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	runner.stub("/repo-feature", "git", fakeResult{exitCode: 1}, "merge-base", "--is-ancestor", "HEAD", "origin/main")

	service := NewService(runner)
	worktrees := []Worktree{
		{Path: "/repo-main", Branch: "main"},
		{Path: "/repo-feature", Branch: "feature/test"},
	}

	enriched := service.EnrichWorktrees(context.Background(), worktrees, "origin/main")
	if len(enriched) != 2 {
		t.Fatalf("len(enriched) = %d, want 2", len(enriched))
	}

	if enriched[0].Status.Error != "" {
		t.Fatalf("enriched[0].Status.Error = %q, want empty string", enriched[0].Status.Error)
	}
	if !enriched[0].Status.HasUpstream || !enriched[0].Status.RemoteExists {
		t.Fatalf("enriched[0] upstream state = %#v, want upstream and remote to be set", enriched[0].Status)
	}
	if enriched[0].Status.Ahead != 0 || enriched[0].Status.Behind != 0 {
		t.Fatalf("enriched[0] ahead/behind = (%d, %d), want (0, 0)", enriched[0].Status.Ahead, enriched[0].Status.Behind)
	}
	if !enriched[0].Status.MergedIntoBase {
		t.Fatalf("enriched[0].Status.MergedIntoBase = false, want true")
	}
	if !enriched[0].Status.DirtyKnown() || enriched[0].Status.IsDirty {
		t.Fatalf("enriched[0] dirty state = %#v, want known clean", enriched[0].Status)
	}

	if enriched[1].Status.Error == "" {
		t.Fatalf("enriched[1].Status.Error = empty, want non-empty error")
	}
	if enriched[1].Status.HasUpstream {
		t.Fatalf("enriched[1].Status.HasUpstream = true, want false")
	}
	if enriched[1].Status.MergedIntoBase {
		t.Fatalf("enriched[1].Status.MergedIntoBase = true, want false")
	}
	if enriched[1].Status.DirtyKnown() {
		t.Fatalf("enriched[1].Status.DirtyKnown() = true, want false after status error")
	}
}
