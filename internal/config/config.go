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
	UI             UIConfig    `yaml:"ui"`
}

type Keybindings struct {
	Refresh string `yaml:"refresh"`
	Quit    string `yaml:"quit"`
	Help    string `yaml:"help"`
}

type UIConfig struct {
	ShowDirtyStatus bool `yaml:"show_dirty_status"`
	ShowCommitHash  bool `yaml:"show_commit_hash"`
	ShowBranch      bool `yaml:"show_branch"`
}

func Defaults() Config {
	return Config{
		DefaultCommand: "git status",
		Keybindings: Keybindings{
			Refresh: "r",
			Quit:    "q",
			Help:    "?",
		},
		UI: UIConfig{
			ShowDirtyStatus: true,
			ShowCommitHash:  true,
			ShowBranch:      true,
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
	if cfg.Keybindings.Quit == "" {
		cfg.Keybindings.Quit = Defaults().Keybindings.Quit
	}
	if cfg.Keybindings.Help == "" {
		cfg.Keybindings.Help = Defaults().Keybindings.Help
	}

	return cfg, nil
}
