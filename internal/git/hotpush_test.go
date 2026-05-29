package git

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecuteHotPushContinuesWhenNothingToCommit(t *testing.T) {
	t.Parallel()

	runner := newFakeRunner()
	runner.stub("/repo", "git", fakeResult{}, "fetch", "--prune")
	runner.stub("/repo", "git", fakeResult{stdout: "origin/feature/demo\n"}, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	runner.stub("/repo", "git", fakeResult{stdout: "Already up to date.\n"}, "pull", "--rebase", "--autostash")
	runner.stub("/repo", "git", fakeResult{}, "add", ".")
	runner.stub("/repo", "git", fakeResult{
		stdout:   "On branch feature/demo\nnothing to commit, working tree clean\n",
		exitCode: 1,
	}, "commit", "-m", "hot push")
	runner.stub("/repo", "git", fakeResult{stdout: "feature/demo\n"}, "branch", "--show-current")
	runner.stub("/repo", "git", fakeResult{stdout: "Everything up-to-date\n"}, "push")

	service := NewService(runner)
	output, err := service.ExecuteHotPush(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("ExecuteHotPush() error = %v\noutput:\n%s", err, output)
	}

	wantCalls := []string{
		runner.key("/repo", "git", "fetch", "--prune"),
		runner.key("/repo", "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"),
		runner.key("/repo", "git", "pull", "--rebase", "--autostash"),
		runner.key("/repo", "git", "add", "."),
		runner.key("/repo", "git", "commit", "-m", "hot push"),
		runner.key("/repo", "git", "branch", "--show-current"),
		runner.key("/repo", "git", "push"),
	}
	if len(runner.calls) != len(wantCalls) {
		t.Fatalf("call count = %d, want %d", len(runner.calls), len(wantCalls))
	}
	for i := range wantCalls {
		if runner.calls[i] != wantCalls[i] {
			t.Fatalf("call %d = %q, want %q", i, runner.calls[i], wantCalls[i])
		}
	}

	for _, snippet := range []string{
		"git fetch --prune",
		"git pull --rebase --autostash",
		"git commit -m hot push",
		"nothing to commit, working tree clean",
		"git push",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("output missing %q:\n%s", snippet, output)
		}
	}
}

func TestExecuteHotPushFallsBackToNewRemoteBranchOnRejectedPush(t *testing.T) {
	t.Parallel()

	runner := newFakeRunner()
	runner.stub("/repo", "git", fakeResult{}, "fetch", "--prune")
	runner.stub("/repo", "git", fakeResult{stdout: "origin/feature/demo\n"}, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	runner.stub("/repo", "git", fakeResult{stdout: "Already up to date.\n"}, "pull", "--rebase", "--autostash")
	runner.stub("/repo", "git", fakeResult{}, "add", ".")
	runner.stub("/repo", "git", fakeResult{stdout: "[feature/demo abc1234] hot push\n 1 file changed, 1 insertion(+)\n"}, "commit", "-m", "hot push")
	runner.stub("/repo", "git", fakeResult{stdout: "feature/demo\n"}, "branch", "--show-current")
	runner.stub("/repo", "git", fakeResult{
		stderr:   "To github.com:example/repo.git\n ! [rejected]        feature/demo -> feature/demo (non-fast-forward)\nerror: failed to push some refs to 'github.com:example/repo.git'\n",
		exitCode: 1,
	}, "push")
	runner.stub("/repo", "git", fakeResult{
		stdout: "remote: Create a pull request for 'feature/demo-hot-push-20260529-120000' on GitHub by visiting:\n",
	}, "push", "-u", "origin", "HEAD:feature/demo-hot-push-20260529-120000")

	service := NewService(runner)
	service.now = func() time.Time {
		return time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)
	}

	output, err := service.ExecuteHotPush(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("ExecuteHotPush() error = %v\noutput:\n%s", err, output)
	}

	if got := runner.calls[len(runner.calls)-1]; got != runner.key("/repo", "git", "push", "-u", "origin", "HEAD:feature/demo-hot-push-20260529-120000") {
		t.Fatalf("last call = %q, want fallback push call", got)
	}
	if !strings.Contains(output, "feature/demo-hot-push-20260529-120000") {
		t.Fatalf("output missing fallback branch name:\n%s", output)
	}
}

func TestExecuteHotPushUsesRemoteAndSkipsPullWithoutUpstream(t *testing.T) {
	t.Parallel()

	runner := newFakeRunner()
	runner.stub("/repo", "git", fakeResult{}, "fetch", "--prune")
	runner.stub("/repo", "git", fakeResult{
		stderr:   "fatal: no upstream configured for branch 'feature/demo'\n",
		exitCode: 128,
	}, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	runner.stub("/repo", "git", fakeResult{}, "add", ".")
	runner.stub("/repo", "git", fakeResult{stdout: "[feature/demo abc1234] hot push\n"}, "commit", "-m", "hot push")
	runner.stub("/repo", "git", fakeResult{stdout: "origin\nbackup\n"}, "remote")
	runner.stub("/repo", "git", fakeResult{stdout: "feature/demo\n"}, "branch", "--show-current")
	runner.stub("/repo", "git", fakeResult{stdout: "branch 'feature/demo' set up to track 'origin/feature/demo'.\n"}, "push", "-u", "origin", "HEAD:feature/demo")

	service := NewService(runner)
	output, err := service.ExecuteHotPush(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("ExecuteHotPush() error = %v\noutput:\n%s", err, output)
	}

	if !strings.Contains(output, "git pull --rebase --autostash\n(skipped: no upstream branch)") {
		t.Fatalf("output missing skipped pull note:\n%s", output)
	}
	if !strings.Contains(output, "git push -u origin HEAD:feature/demo") {
		t.Fatalf("output missing upstream setup push:\n%s", output)
	}
}
