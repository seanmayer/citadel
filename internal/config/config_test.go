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
}
