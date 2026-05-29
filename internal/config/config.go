package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a scalar value")
	}

	raw := strings.TrimSpace(value.Value)
	if raw == "" {
		d.Duration = 0
		return nil
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", raw, err)
	}
	if parsed < 0 {
		return fmt.Errorf("duration %q must be non-negative", raw)
	}

	d.Duration = parsed
	return nil
}

type Config struct {
	DefaultCommand string         `yaml:"default_command"`
	Keybindings    Keybindings    `yaml:"keybindings"`
	Editor         EditorConfig   `yaml:"editor"`
	Terminal       TerminalConfig `yaml:"terminal"`
	Git            GitConfig      `yaml:"git"`
	UI             UIConfig       `yaml:"ui"`
}

type Keybindings struct {
	OpenEditor   string `yaml:"open_editor"`
	OpenTerminal string `yaml:"open_terminal"`
	Refresh      string `yaml:"refresh"`
	FetchRefresh string `yaml:"fetch_refresh"`
	Quit         string `yaml:"quit"`
	Help         string `yaml:"help"`
}

type EditorConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type TerminalConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type GitConfig struct {
	BaseBranch          string   `yaml:"base_branch"`
	FetchOnRefresh      bool     `yaml:"fetch_on_refresh"`
	AutoRefreshInterval Duration `yaml:"auto_refresh_interval"`
	ShowRemoteStatus    bool     `yaml:"show_remote_status"`
	ShowMergeStatus     bool     `yaml:"show_merge_status"`
	ShowDirtyStatus     bool     `yaml:"show_dirty_status"`
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
			OpenEditor:   "o",
			OpenTerminal: "t",
			Refresh:      "r",
			FetchRefresh: "R",
			Quit:         "q",
			Help:         "?",
		},
		Editor: EditorConfig{
			Command: "code",
			Args:    []string{"."},
		},
		Terminal: defaultTerminalConfig(),
		Git: GitConfig{
			BaseBranch:          "origin/main",
			FetchOnRefresh:      false,
			AutoRefreshInterval: Duration{Duration: 30 * time.Second},
			ShowRemoteStatus:    true,
			ShowMergeStatus:     true,
			ShowDirtyStatus:     true,
		},
		UI: UIConfig{
			ShowCommitHash:  true,
			ShowBranch:      true,
			ShowDirtyStatus: true,
		},
	}
}

func defaultTerminalConfig() TerminalConfig {
	switch runtime.GOOS {
	case "darwin":
		return TerminalConfig{
			Command: "open",
			Args:    []string{"-a", "Terminal", "{path}"},
		}
	case "windows":
		return TerminalConfig{
			Command: "wt",
			Args:    []string{"-d", "{path}"},
		}
	default:
		return TerminalConfig{
			Command: "x-terminal-emulator",
			Args:    []string{"--working-directory={path}"},
		}
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
	if cfg.Keybindings.OpenEditor == "" {
		cfg.Keybindings.OpenEditor = Defaults().Keybindings.OpenEditor
	}
	if cfg.Keybindings.OpenTerminal == "" {
		cfg.Keybindings.OpenTerminal = Defaults().Keybindings.OpenTerminal
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
	if cfg.Editor.Command == "" {
		cfg.Editor.Command = Defaults().Editor.Command
	}
	if cfg.Editor.Args == nil {
		cfg.Editor.Args = append([]string(nil), Defaults().Editor.Args...)
	}
	if cfg.Terminal.Command == "" {
		cfg.Terminal.Command = Defaults().Terminal.Command
	}
	if cfg.Terminal.Args == nil {
		cfg.Terminal.Args = append([]string(nil), Defaults().Terminal.Args...)
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
