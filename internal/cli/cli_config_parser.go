package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joebalancio/wt/internal/config"
)

// ConfigScope defines which config to target
type ConfigScope int

// Config scope constants define which configuration file(s) to use.
// ScopeMerged: Read returns merged config, write defaults to local.
// ScopeLocal: Local .wt.yaml only.
// ScopeGlobal: Global ~/.config/wt/config.yaml only.
const (
	ScopeMerged ConfigScope = iota // Read: merged, Write: local
	ScopeLocal                     // Local only
	ScopeGlobal                    // Global only
)

// Operation defines read vs write context
type Operation int

// Operation constants define whether we're reading or writing config.
const (
	OpRead  Operation = iota // Read operation
	OpWrite                  // Write operation
)

// ResolveConfigPaths returns the appropriate paths based on scope and operation.
// For ScopeMerged with OpRead: returns both project and global paths for merging
// For ScopeMerged with OpWrite: returns project path only (default to local)
// For ScopeLocal: returns project path only, errors if not in git repo
// For ScopeGlobal: returns global path only
func ResolveConfigPaths(scope ConfigScope, op Operation) (projectPath, globalPath string, err error) {
	// Get global path (always available)
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		home = os.Getenv("HOME")
	}
	globalPath = filepath.Join(home, ".config", "wt", "config.yaml")

	switch scope {
	case ScopeGlobal:
		return "", globalPath, nil

	case ScopeLocal:
		gitRoot, gitErr := config.FindGitRoot()
		if gitErr != nil {
			return "", "", fmt.Errorf("not in a git repository: local config requires being in a git repository, use --global to modify global config")
		}
		projectPath = filepath.Join(gitRoot, ".wt.yaml")
		return projectPath, "", nil

	case ScopeMerged:
		if op == OpWrite {
			// Default writes to local
			gitRoot, gitErr := config.FindGitRoot()
			if gitErr != nil {
				return "", "", fmt.Errorf("not in a git repository: local config requires being in a git repository, use --global to modify global config")
			}
			projectPath = filepath.Join(gitRoot, ".wt.yaml")
			return projectPath, "", nil
		}
		// Read operation: use FindConfigs for merged behavior
		projectPath, globalPath, err = config.FindConfigs("")
		if err != nil {
			// No configs found, but that's okay for reads (will use defaults)
			return "", "", nil
		}
		return projectPath, globalPath, nil

	default:
		return "", "", fmt.Errorf("unknown config scope: %d", scope)
	}
}

// GetValue retrieves a value from config using dot-notation key
func GetValue(cfg *config.Config, key string) (interface{}, error) {
	parts := strings.Split(key, ".")

	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid key format: %q (use <section>.<field> or <section>.<subsection>.<field>)", key)
	}

	// Check if key is supported for CLI manipulation
	if !isSupportedKey(key) {
		return nil, fmt.Errorf("key %q not supported for CLI manipulation", key)
	}

	section := parts[0]

	switch section {
	case "worktree":
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key format: %q (worktree keys are <section>.<field>)", key)
		}
		return getWorktreeValue(cfg, parts[1])
	default:
		return nil, fmt.Errorf("unknown section: %s", section)
	}
}

// getWorktreeValue retrieves a worktree config value
func getWorktreeValue(cfg *config.Config, field string) (interface{}, error) {
	switch field {
	case "location":
		return cfg.Worktree.Location, nil
	case "dedicated_path":
		return cfg.Worktree.GetDedicatedPath(), nil
	default:
		return nil, fmt.Errorf("unknown key: worktree.%s", field)
	}
}

// SetValue sets a value in config using dot-notation key
func SetValue(cfg *config.Config, key, value string) error {
	parts := strings.Split(key, ".")

	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("invalid key format: %q (use <section>.<field> or <section>.<subsection>.<field>)", key)
	}

	// Check if key is supported for CLI manipulation
	if !isSupportedKey(key) {
		return fmt.Errorf("key %q not supported for CLI manipulation\n       Edit config file directly to modify hooks", key)
	}

	section := parts[0]

	switch section {
	case "worktree":
		if len(parts) != 2 {
			return fmt.Errorf("invalid key format: %q (worktree keys are <section>.<field>)", key)
		}
		return setWorktreeValue(cfg, parts[1], value)
	default:
		return fmt.Errorf("unknown section: %s", section)
	}
}

// setWorktreeValue sets a worktree config value
func setWorktreeValue(cfg *config.Config, field, value string) error {
	switch field {
	case "location":
		// Validate enum value
		validValues := []string{"dedicated", "per-repo"}
		for _, v := range validValues {
			if value == v {
				cfg.Worktree.Location = value
				return nil
			}
		}
		return fmt.Errorf("invalid value %q for worktree.location\n       Valid values: dedicated, per-repo", value)
	case "dedicated_path":
		cfg.Worktree.DedicatedPath = value
		return nil
	default:
		return fmt.Errorf("unknown key: worktree.%s", field)
	}
}

// UnsetValue removes a key from config, reverting to default
func UnsetValue(cfg *config.Config, key string) error {
	parts := strings.Split(key, ".")

	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("invalid key format: %q (use <section>.<field> or <section>.<subsection>.<field>)", key)
	}

	// Check if key is supported for CLI manipulation
	if !isSupportedKey(key) {
		return fmt.Errorf("key %q not supported for CLI manipulation", key)
	}

	section := parts[0]

	switch section {
	case "worktree":
		if len(parts) != 2 {
			return fmt.Errorf("invalid key format: %q (worktree keys are <section>.<field>)", key)
		}
		return unsetWorktreeValue(cfg, parts[1])
	default:
		return fmt.Errorf("unknown section: %s", section)
	}
}

// unsetWorktreeValue unsets a worktree config value to default
func unsetWorktreeValue(cfg *config.Config, field string) error {
	switch field {
	case "location":
		cfg.Worktree.Location = "dedicated" // default
		return nil
	case "dedicated_path":
		cfg.Worktree.DedicatedPath = "" // empty triggers default in GetDedicatedPath()
		return nil
	default:
		return fmt.Errorf("unknown key: worktree.%s", field)
	}
}

// isSupportedKey returns true if key can be manipulated via CLI
func isSupportedKey(key string) bool {
	supportedKeys := map[string]bool{
		"worktree.location":       true,
		"worktree.dedicated_path": true,
	}
	return supportedKeys[key]
}

// formatValue converts a value to string for output
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case string:
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseBool converts a string to boolean with flexible input
func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %q (use: true, false, 1, 0, yes, no)", s)
	}
}

// parseInt converts a string to integer with range validation
func parseInt(s string, minVal, maxVal int) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value: %q", s)
	}
	if i < minVal || i > maxVal {
		return 0, fmt.Errorf("integer value %q out of range (must be between %d and %d)", s, minVal, maxVal)
	}
	return i, nil
}
