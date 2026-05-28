package editor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/seanmayer/citadel/internal/config"
)

type stubRunner struct {
	calls    []runnerCall
	stdout   string
	stderr   string
	exitCode int
	err      error
}

type runnerCall struct {
	dir  string
	name string
	args []string
}

func (s *stubRunner) Run(_ context.Context, dir string, name string, args ...string) (string, string, int, error) {
	s.calls = append(s.calls, runnerCall{
		dir:  dir,
		name: name,
		args: append([]string(nil), args...),
	})
	return s.stdout, s.stderr, s.exitCode, s.err
}

func TestOpenRunsConfiguredEditorInsideWorktree(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{}
	service := NewService(config.EditorConfig{
		Command: "code",
		Args:    []string{"--reuse-window", "{path}"},
	}, runner)

	output, err := service.Open(context.Background(), "/repo/feature")
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	if output != "(no output)" {
		t.Fatalf("output = %q, want %q", output, "(no output)")
	}

	if len(runner.calls) != 1 {
		t.Fatalf("Run() call count = %d, want 1", len(runner.calls))
	}

	call := runner.calls[0]
	if call.dir != "/repo/feature" {
		t.Fatalf("dir = %q, want %q", call.dir, "/repo/feature")
	}
	if call.name != "code" {
		t.Fatalf("name = %q, want %q", call.name, "code")
	}
	if got := strings.Join(call.args, " "); got != "--reuse-window /repo/feature" {
		t.Fatalf("args = %q, want %q", got, "--reuse-window /repo/feature")
	}
}

func TestOpenReturnsRunnerError(t *testing.T) {
	t.Parallel()

	service := NewService(config.EditorConfig{
		Command: "code",
		Args:    []string{"."},
	}, &stubRunner{err: errors.New("executable file not found")})

	_, err := service.Open(context.Background(), "/repo/feature")
	if err == nil {
		t.Fatal("Open() error = nil, want error")
	}
	if !strings.Contains(err.Error(), `open editor in "/repo/feature"`) {
		t.Fatalf("error = %q, want worktree path context", err)
	}
}

func TestOpenReturnsCommandOutputOnNonZeroExit(t *testing.T) {
	t.Parallel()

	service := NewService(config.EditorConfig{
		Command: "code",
		Args:    []string{"."},
	}, &stubRunner{
		stderr:   "code command failed",
		exitCode: 1,
	})

	output, err := service.Open(context.Background(), "/repo/feature")
	if err == nil {
		t.Fatal("Open() error = nil, want error")
	}
	if output != "code command failed" {
		t.Fatalf("output = %q, want %q", output, "code command failed")
	}
	if !strings.Contains(err.Error(), "code command failed") {
		t.Fatalf("error = %q, want command output", err)
	}
}
