package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultCommand string      `yaml:"default_command"`
	Keybindings    Keybindings `yaml:"keybindings"`
	Git            GitConfig   `yaml:"git"`
	UI             UIConfig    `yaml:"ui"`
}

type Keybindings struct {
	Refresh      string `yaml:"refresh"`
	FetchRefresh string `yaml:"fetch_refresh"`
	Quit         string `yaml:"quit"`
	Help         string `yaml:"help"`
}

type GitConfig struct {
	BaseBranch       string `yaml:"base_branch"`
	FetchOnRefresh   bool   `yaml:"fetch_on_refresh"`
	ShowRemoteStatus bool   `yaml:"show_remote_status"`
	ShowMergeStatus  bool   `yaml:"show_merge_status"`
	ShowDirtyStatus  bool   `yaml:"show_dirty_status"`
}

type UIConfig struct {
	ShowCommitHash  bool `yaml:"show_commit_hash"`
	ShowBranch      bool `yaml:"show_branch"`
	ShowDirtyStatus bool `yaml:"show_dirty_status"`
}

func Defaults() Config {
	return Config{
		DefaultCommand: "git status",
		Keybindings: Keybindings{
			Refresh:      "r",
			FetchRefresh: "R",
			Quit:         "q",
			Help:         "?",
		},
		Git: GitConfig{
			BaseBranch:       "origin/main",
			FetchOnRefresh:   false,
			ShowRemoteStatus: true,
			ShowMergeStatus:  true,
			ShowDirtyStatus:  true,
		},
		UI: UIConfig{
			ShowCommitHash:  true,
			ShowBranch:      true,
			ShowDirtyStatus: true,
		},
	}
}

func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("detect user config directory: %w", err)
	}

	return filepath.Join(configDir, "gwtui", "config.yaml"), nil
}

func Load(path string) (Config, error) {
	cfg := Defaults()

	if path == "" {
		defaultPath, err := DefaultPath()
		if err != nil {
			return cfg, err
		}
		path = defaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config file %q: %w", path, err)
	}

	if cfg.DefaultCommand == "" {
		cfg.DefaultCommand = Defaults().DefaultCommand
	}

	if cfg.Keybindings.Refresh == "" {
		cfg.Keybindings.Refresh = Defaults().Keybindings.Refresh
	}
	if cfg.Keybindings.FetchRefresh == "" {
		cfg.Keybindings.FetchRefresh = Defaults().Keybindings.FetchRefresh
	}
	if cfg.Keybindings.Quit == "" {
		cfg.Keybindings.Quit = Defaults().Keybindings.Quit
	}
	if cfg.Keybindings.Help == "" {
		cfg.Keybindings.Help = Defaults().Keybindings.Help
	}
	if cfg.Git.BaseBranch == "" {
		cfg.Git.BaseBranch = Defaults().Git.BaseBranch
	}

	// Keep older configs working if they still use ui.show_dirty_status.
	if cfg.Git.ShowDirtyStatus == Defaults().Git.ShowDirtyStatus &&
		cfg.UI.ShowDirtyStatus != Defaults().UI.ShowDirtyStatus {
		cfg.Git.ShowDirtyStatus = cfg.UI.ShowDirtyStatus
	}

	return cfg, nil
}
