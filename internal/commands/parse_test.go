package commands

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseGitCommand(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(`git log --oneline -5 --grep "fix bug"`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	wantArgs := []string{"log", "--oneline", "-5", "--grep", "fix bug"}
	if parsed.Command != "git" {
		t.Fatalf("Command = %q, want git", parsed.Command)
	}
	if !reflect.DeepEqual(parsed.Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", parsed.Args, wantArgs)
	}
}

func TestParseRejectsNonGitCommands(t *testing.T) {
	t.Parallel()

	_, err := Parse("ls -la")
	if err == nil {
		t.Fatal("Parse() error = nil, want validation error")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
}

func TestParseRejectsEmptyCommand(t *testing.T) {
	t.Parallel()

	_, err := Parse("   ")
	if err == nil {
		t.Fatal("Parse() error = nil, want validation error")
	}
}
