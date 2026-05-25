package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg != Defaults() {
		t.Fatalf("Load() = %#v, want %#v", cfg, Defaults())
	}

	if cfg.Git.BaseBranch != "origin/main" {
		t.Fatalf("Git.BaseBranch = %q, want %q", cfg.Git.BaseBranch, "origin/main")
	}
	if cfg.Git.FetchOnRefresh {
		t.Fatalf("Git.FetchOnRefresh = true, want false")
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
	content := []byte("default_command: git fetch\nui:\n  show_commit_hash: false\n")
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
	if cfg.Keybindings.Refresh != Defaults().Keybindings.Refresh {
		t.Fatalf("Refresh key = %q, want %q", cfg.Keybindings.Refresh, Defaults().Keybindings.Refresh)
	}
	if cfg.Git.BaseBranch != Defaults().Git.BaseBranch {
		t.Fatalf("Git.BaseBranch = %q, want %q", cfg.Git.BaseBranch, Defaults().Git.BaseBranch)
	}
	if cfg.Git.FetchOnRefresh != Defaults().Git.FetchOnRefresh {
		t.Fatalf("Git.FetchOnRefresh = %t, want %t", cfg.Git.FetchOnRefresh, Defaults().Git.FetchOnRefresh)
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
