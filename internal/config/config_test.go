package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if !reflect.DeepEqual(cfg, Defaults()) {
		t.Fatalf("Load() = %#v, want %#v", cfg, Defaults())
	}

	if cfg.Git.BaseBranch != "origin/main" {
		t.Fatalf("Git.BaseBranch = %q, want %q", cfg.Git.BaseBranch, "origin/main")
	}
	if cfg.Git.FetchOnRefresh {
		t.Fatalf("Git.FetchOnRefresh = true, want false")
	}
	if cfg.Keybindings.OpenEditor != Defaults().Keybindings.OpenEditor {
		t.Fatalf("OpenEditor key = %q, want %q", cfg.Keybindings.OpenEditor, Defaults().Keybindings.OpenEditor)
	}
	if cfg.Keybindings.OpenTerminal != Defaults().Keybindings.OpenTerminal {
		t.Fatalf("OpenTerminal key = %q, want %q", cfg.Keybindings.OpenTerminal, Defaults().Keybindings.OpenTerminal)
	}
	if cfg.Editor.Command != Defaults().Editor.Command {
		t.Fatalf("Editor.Command = %q, want %q", cfg.Editor.Command, Defaults().Editor.Command)
	}
	if !reflect.DeepEqual(cfg.Editor.Args, Defaults().Editor.Args) {
		t.Fatalf("Editor.Args = %#v, want %#v", cfg.Editor.Args, Defaults().Editor.Args)
	}
	if cfg.Terminal.Command != Defaults().Terminal.Command {
		t.Fatalf("Terminal.Command = %q, want %q", cfg.Terminal.Command, Defaults().Terminal.Command)
	}
	if !reflect.DeepEqual(cfg.Terminal.Args, Defaults().Terminal.Args) {
		t.Fatalf("Terminal.Args = %#v, want %#v", cfg.Terminal.Args, Defaults().Terminal.Args)
	}
	if !cfg.Git.ShowRemoteStatus {
		t.Fatalf("Git.ShowRemoteStatus = false, want true")
	}
	if !cfg.Git.ShowMergeStatus {
		t.Fatalf("Git.ShowMergeStatus = false, want true")
	}
	if !cfg.Git.ShowDirtyStatus {
		t.Fatalf("Git.ShowDirtyStatus = false, want true")
	}
}

func TestLoadMergesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("default_command: git fetch\neditor:\n  command: zed\n  args:\n    - .\nterminal:\n  command: wezterm\n  args:\n    - start\n    - --cwd\n    - '{path}'\nkeybindings:\n  open_editor: e\n  open_terminal: x\nui:\n  show_commit_hash: false\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.DefaultCommand != "git fetch" {
		t.Fatalf("DefaultCommand = %q, want %q", cfg.DefaultCommand, "git fetch")
	}
	if cfg.UI.ShowCommitHash {
		t.Fatalf("ShowCommitHash = true, want false")
	}
	if cfg.Keybindings.OpenEditor != "e" {
		t.Fatalf("OpenEditor key = %q, want %q", cfg.Keybindings.OpenEditor, "e")
	}
	if cfg.Keybindings.OpenTerminal != "x" {
		t.Fatalf("OpenTerminal key = %q, want %q", cfg.Keybindings.OpenTerminal, "x")
	}
	if cfg.Editor.Command != "zed" {
		t.Fatalf("Editor.Command = %q, want %q", cfg.Editor.Command, "zed")
	}
	if !reflect.DeepEqual(cfg.Editor.Args, []string{"."}) {
		t.Fatalf("Editor.Args = %#v, want %#v", cfg.Editor.Args, []string{"."})
	}
	if cfg.Terminal.Command != "wezterm" {
		t.Fatalf("Terminal.Command = %q, want %q", cfg.Terminal.Command, "wezterm")
	}
	if !reflect.DeepEqual(cfg.Terminal.Args, []string{"start", "--cwd", "{path}"}) {
		t.Fatalf("Terminal.Args = %#v, want %#v", cfg.Terminal.Args, []string{"start", "--cwd", "{path}"})
	}
	if cfg.Keybindings.Refresh != Defaults().Keybindings.Refresh {
		t.Fatalf("Refresh key = %q, want %q", cfg.Keybindings.Refresh, Defaults().Keybindings.Refresh)
	}
	if cfg.Git.BaseBranch != Defaults().Git.BaseBranch {
		t.Fatalf("Git.BaseBranch = %q, want %q", cfg.Git.BaseBranch, Defaults().Git.BaseBranch)
	}
	if cfg.Git.FetchOnRefresh != Defaults().Git.FetchOnRefresh {
		t.Fatalf("Git.FetchOnRefresh = %t, want %t", cfg.Git.FetchOnRefresh, Defaults().Git.FetchOnRefresh)
	}
	if cfg.Git.AutoRefreshInterval != Defaults().Git.AutoRefreshInterval {
		t.Fatalf("Git.AutoRefreshInterval = %v, want %v", cfg.Git.AutoRefreshInterval, Defaults().Git.AutoRefreshInterval)
	}
	if cfg.Git.ShowRemoteStatus != Defaults().Git.ShowRemoteStatus {
		t.Fatalf("Git.ShowRemoteStatus = %t, want %t", cfg.Git.ShowRemoteStatus, Defaults().Git.ShowRemoteStatus)
	}
	if cfg.Git.ShowMergeStatus != Defaults().Git.ShowMergeStatus {
		t.Fatalf("Git.ShowMergeStatus = %t, want %t", cfg.Git.ShowMergeStatus, Defaults().Git.ShowMergeStatus)
	}
	if cfg.Git.ShowDirtyStatus != Defaults().Git.ShowDirtyStatus {
		t.Fatalf("Git.ShowDirtyStatus = %t, want %t", cfg.Git.ShowDirtyStatus, Defaults().Git.ShowDirtyStatus)
	}
}

func TestLoadParsesAutoRefreshInterval(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("git:\n  auto_refresh_interval: 45s\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Git.AutoRefreshInterval.Duration != 45*time.Second {
		t.Fatalf("Git.AutoRefreshInterval = %v, want %v", cfg.Git.AutoRefreshInterval.Duration, 45*time.Second)
	}
}

func TestLoadRejectsInvalidAutoRefreshInterval(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("git:\n  auto_refresh_interval: later\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
}
