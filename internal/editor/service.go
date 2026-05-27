package editor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/seanmayer/citadel/internal/config"
)

const pathPlaceholder = "{path}"

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
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
	runner Runner
	config config.EditorConfig
}

func NewService(cfg config.EditorConfig, runner Runner) *Service {
	if runner == nil {
		runner = execRunner{}
	}

	args := append([]string(nil), cfg.Args...)
	return &Service{
		runner: runner,
		config: config.EditorConfig{
			Command: strings.TrimSpace(cfg.Command),
			Args:    args,
		},
	}
}

func (s *Service) Open(ctx context.Context, worktreePath string) error {
	if s.config.Command == "" {
		return errors.New("editor command is empty")
	}

	args := make([]string, 0, len(s.config.Args))
	for _, arg := range s.config.Args {
		args = append(args, strings.ReplaceAll(arg, pathPlaceholder, worktreePath))
	}

	stdout, stderr, exitCode, err := s.runner.Run(ctx, worktreePath, s.config.Command, args...)
	if err != nil {
		return fmt.Errorf("open editor in %q: %w", worktreePath, err)
	}
	if exitCode != 0 {
		message := strings.TrimSpace(firstNonEmpty(stderr, stdout))
		if message == "" {
			message = fmt.Sprintf("editor command exited with status %d", exitCode)
		}
		return fmt.Errorf("open editor in %q: %s", worktreePath, message)
	}

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
