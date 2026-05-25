package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/shlex"
)

type ValidationError struct {
	message string
}

func (e *ValidationError) Error() string {
	return e.message
}

type ParsedCommand struct {
	Raw     string
	Command string
	Args    []string
}

type GitExecutor interface {
	ExecuteGitCommand(ctx context.Context, worktreePath string, args []string) (string, error)
}

type Result struct {
	Parsed ParsedCommand
	Output string
}

type Service struct {
	executor GitExecutor
}

func NewService(executor GitExecutor) *Service {
	return &Service{executor: executor}
}

func Parse(raw string) (ParsedCommand, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedCommand{}, &ValidationError{message: "command is empty"}
	}

	parts, err := shlex.Split(raw)
	if err != nil {
		return ParsedCommand{}, &ValidationError{message: fmt.Sprintf("parse command: %v", err)}
	}
	if len(parts) == 0 {
		return ParsedCommand{}, &ValidationError{message: "command is empty"}
	}
	if parts[0] != "git" {
		return ParsedCommand{}, &ValidationError{message: fmt.Sprintf("only git commands are supported, got %q", parts[0])}
	}

	return ParsedCommand{
		Raw:     raw,
		Command: parts[0],
		Args:    parts[1:],
	}, nil
}

func (s *Service) Execute(ctx context.Context, worktreePath string, raw string) (Result, error) {
	parsed, err := Parse(raw)
	if err != nil {
		return Result{}, err
	}

	output, err := s.executor.ExecuteGitCommand(ctx, worktreePath, parsed.Args)
	return Result{
		Parsed: parsed,
		Output: output,
	}, err
}
