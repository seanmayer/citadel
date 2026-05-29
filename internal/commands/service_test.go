package commands

import (
	"context"
	"testing"
)

type stubExecutor struct {
	gitCalls     [][]string
	hotPushCalls []string
}

func (s *stubExecutor) ExecuteGitCommand(_ context.Context, _ string, args []string) (string, error) {
	s.gitCalls = append(s.gitCalls, append([]string(nil), args...))
	return "git ok", nil
}

func (s *stubExecutor) ExecuteHotPush(_ context.Context, worktreePath string) (string, error) {
	s.hotPushCalls = append(s.hotPushCalls, worktreePath)
	return "hot push ok", nil
}

func TestExecuteRecognizesHotPushAliases(t *testing.T) {
	t.Parallel()

	tests := []string{
		"hot push",
		"git hot push",
	}

	for _, raw := range tests {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			executor := &stubExecutor{}
			service := NewService(executor)

			result, err := service.Execute(context.Background(), "/repo", raw)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if len(executor.hotPushCalls) != 1 {
				t.Fatalf("hot push call count = %d, want 1", len(executor.hotPushCalls))
			}
			if len(executor.gitCalls) != 0 {
				t.Fatalf("git call count = %d, want 0", len(executor.gitCalls))
			}
			if result.Parsed.Command != "hot" {
				t.Fatalf("Parsed.Command = %q, want %q", result.Parsed.Command, "hot")
			}
			if result.Output != "hot push ok" {
				t.Fatalf("Output = %q, want %q", result.Output, "hot push ok")
			}
		})
	}
}
