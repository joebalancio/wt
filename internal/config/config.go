// Package config handles YAML configuration loading, validation, and discovery for wt.
// Configuration is loaded from .wt.yaml in the current directory or ~/.config/wt/config.yaml
// following XDG standards, with support for hooks and worktree location modes.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Hooks    HooksConfig    `yaml:"hooks"`
	Worktree WorktreeConfig `yaml:"worktree"`
	Spice    SpiceConfig    `yaml:"spice"`
}

// HooksConfig defines hook configurations
type HooksConfig struct {
	OnWorktreeCreate []Hook `yaml:"on_worktree_create"`
	OnWorktreeDone   []Hook `yaml:"on_worktree_done,omitempty"`
	OnWorktreeRemove []Hook `yaml:"on_worktree_remove,omitempty"`
}

// Hook represents a single command to run
type Hook struct {
	Run     string `yaml:"run"`
	Cwd     string `yaml:"cwd,omitempty"`
	Timeout string `yaml:"timeout,omitempty"` // e.g., "30s", "2m", "1h"
}

// DefaultHookTimeout is the default timeout for hook execution
const DefaultHookTimeout = 30 * time.Second

// ParseTimeout parses the timeout string and returns the duration.
// Returns DefaultHookTimeout if timeout is empty.
// Returns error for invalid formats (bare numbers, malformed strings).
func (h *Hook) ParseTimeout() (time.Duration, error) {
	if h.Timeout == "" {
		return DefaultHookTimeout, nil
	}

	// time.ParseDuration requires units, so bare numbers will fail
	d, err := time.ParseDuration(h.Timeout)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w (hint: use units like 30s, 2m, 1h)", h.Timeout, err)
	}
	return d, nil
}

// WorktreeConfig contains worktree-specific settings
type WorktreeConfig struct {
	Location      string `yaml:"location"`       // "dedicated" or "per-repo"
	DedicatedPath string `yaml:"dedicated_path"` // custom path for dedicated mode
}

// SpiceConfig contains git-spice specific settings
type SpiceConfig struct {
	BinaryPath string `yaml:"binary_path"` // Path to git-spice binary
}

// IsDedicated returns true if using dedicated worktree location
func (w *WorktreeConfig) IsDedicated() bool {
	return w.Location == "dedicated"
}

// GetDedicatedPath returns the dedicated path (with default fallback)
func (w *WorktreeConfig) GetDedicatedPath() string {
	if w.DedicatedPath != "" {
		return w.DedicatedPath
	}
	return "~/worktrees" // default
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Worktree: WorktreeConfig{
			Location:      "", // empty means per-repo (default)
			DedicatedPath: "~/worktrees",
		},
		Spice: SpiceConfig{
			BinaryPath: "", // Empty means not configured
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

// LoadMerged loads and merges project and global configurations
// Precedence: project > global > defaults
// Merge semantics:
//   - Scalars: project value replaces global
//   - Arrays: project array replaces global entirely
//   - Undefined: inherits from global/defaults
func LoadMerged(projectPath, globalPath string) (*Config, error) {
	cfg := DefaultConfig()

	// Load global config first (if exists)
	if globalPath != "" {
		data, err := os.ReadFile(globalPath)
		if err != nil {
			return nil, fmt.Errorf("reading global config: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing global config: %w", err)
		}
	}

	// Overlay project config (if exists)
	if projectPath != "" {
		data, err := os.ReadFile(projectPath)
		if err != nil {
			return nil, fmt.Errorf("reading project config: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing project config: %w", err)
		}
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// No global validation required currently
	return nil
}

// ValidateSchema checks if configuration values conform to schema constraints
func (c *Config) ValidateSchema() error {
	// Validate worktree.location enum
	if c.Worktree.Location != "" &&
		c.Worktree.Location != "dedicated" &&
		c.Worktree.Location != "per-repo" {
		return fmt.Errorf("invalid worktree.location: %q (must be 'dedicated' or 'per-repo')",
			c.Worktree.Location)
	}
	return nil
}

// Save writes the configuration to a file
func (c *Config) Save(path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
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

// FindGitRoot discovers the Git repository root using git rev-parse --show-toplevel
// Returns an error if not in a Git repository
func FindGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// FindConfigs discovers project and global config paths
// projectPath: .wt.yaml at Git root (may be "")
// globalPath: ~/.config/wt/config.yaml (may be "")
// Returns error only if neither config exists
func FindConfigs(customPath string) (projectPath, globalPath string, err error) {
	// If custom path provided, use it exclusively
	if customPath != "" {
		if _, statErr := os.Stat(customPath); statErr != nil {
			return "", "", fmt.Errorf("custom config path not found: %w", statErr)
		}
		return customPath, "", nil
	}

	// Try to find project config at Git root
	gitRoot, gitErr := FindGitRoot()
	if gitErr == nil {
		candidateProject := filepath.Join(gitRoot, ".wt.yaml")
		if _, statErr := os.Stat(candidateProject); statErr == nil {
			projectPath = candidateProject
		}
	}

	// Check for global config
	home, homeErr := os.UserHomeDir()
	if homeErr == nil {
		candidateGlobal := filepath.Join(home, ".config", "wt", "config.yaml")
		if _, statErr := os.Stat(candidateGlobal); statErr == nil {
			globalPath = candidateGlobal
		}
	}

	// Return error only if no configs found
	if projectPath == "" && globalPath == "" {
		return "", "", fmt.Errorf("no configuration file found")
	}

	return projectPath, globalPath, nil
}
