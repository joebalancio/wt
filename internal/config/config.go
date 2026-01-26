package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Global    GlobalConfig     `yaml:"global"`
	Hooks     HooksConfig      `yaml:"hooks"`
	Tmux      TmuxConfig       `yaml:"tmux"`
	Worktree  WorktreeConfig   `yaml:"worktree"`
	Overrides []OverrideConfig `yaml:"project_overrides,omitempty"`
}

// GlobalConfig contains global settings
type GlobalConfig struct {
	WorktreeRoot      string `yaml:"worktree_root"`
	TmuxSessionPrefix string `yaml:"tmux_session_prefix"`
}

// HooksConfig defines hook configurations
type HooksConfig struct {
	OnWorktreeCreate []Hook `yaml:"on_worktree_create"`
	OnWorktreeRemove []Hook `yaml:"on_worktree_remove,omitempty"`
}

// Hook represents a single command to run
type Hook struct {
	Run        string `yaml:"run"`
	Cwd        string `yaml:"cwd,omitempty"`
	Background bool   `yaml:"background,omitempty"`
	Parallel   bool   `yaml:"parallel,omitempty"`
}

// TmuxConfig contains tmux-specific settings
type TmuxConfig struct {
	Layout         string `yaml:"layout,omitempty"`
	WindowName     string `yaml:"window_name,omitempty"`
	AttachOnCreate bool   `yaml:"attach_on_create,omitempty"`
}

// WorktreeConfig contains worktree-specific settings
type WorktreeConfig struct {
	Location      string `yaml:"location"`       // "dedicated" or "per-repo"
	DedicatedPath string `yaml:"dedicated_path"` // custom path for dedicated mode
}

// IsDedicated returns true if using dedicated worktree location
func (w *WorktreeConfig) IsDedicated() bool {
	return w.Location == "" || w.Location == "dedicated"
}

// GetDedicatedPath returns the dedicated path (with default fallback)
func (w *WorktreeConfig) GetDedicatedPath() string {
	if w.DedicatedPath != "" {
		return w.DedicatedPath
	}
	return "~/worktrees" // default
}

// OverrideConfig allows project-specific overrides
type OverrideConfig struct {
	Match string      `yaml:"match"`
	Hooks HooksConfig `yaml:"hooks,omitempty"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			WorktreeRoot:      "~/dev/worktrees",
			TmuxSessionPrefix: "wt-",
		},
		Tmux: TmuxConfig{
			Layout:         "main-vertical",
			WindowName:     "work",
			AttachOnCreate: true,
		},
		Worktree: WorktreeConfig{
			Location:      "dedicated",
			DedicatedPath: "~/worktrees",
		},
	}
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Global.WorktreeRoot == "" {
		return fmt.Errorf("worktree_root cannot be empty")
	}
	return nil
}

// Save writes the configuration to a file
func (c *Config) Save(path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// FindConfig looks for a config file in standard locations
func FindConfig(customPath string) (string, error) {
	if customPath != "" {
		if _, err := os.Stat(customPath); err != nil {
			return "", fmt.Errorf("custom config path not found: %w", err)
		}
		return customPath, nil
	}

	// Check current directory
	if _, err := os.Stat(".wt.yaml"); err == nil {
		return ".wt.yaml", nil
	}

	// Check XDG config directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	xdgConfig := filepath.Join(home, ".config", "wt", "config.yaml")
	if _, err := os.Stat(xdgConfig); err == nil {
		return xdgConfig, nil
	}

	return "", fmt.Errorf("no configuration file found")
}
