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
	case "tmux":
		return getTmuxValue(cfg, parts[1:])
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

// getTmuxValue retrieves a tmux config value (supports 2-level and 3-level keys)
func getTmuxValue(cfg *config.Config, parts []string) (interface{}, error) {
	if len(parts) == 1 {
		// 2-level key: tmux.layout, tmux.window_name, tmux.attach_on_create
		field := parts[0]
		switch field {
		case "layout":
			return cfg.Tmux.Layout, nil
		case "window_name":
			return cfg.Tmux.WindowName, nil
		case "attach_on_create":
			return cfg.Tmux.AttachOnCreate, nil
		default:
			return nil, fmt.Errorf("unknown key: tmux.%s", field)
		}
	}

	if len(parts) == 2 {
		// 3-level key: tmux.window_naming.*
		subsection := parts[0]
		field := parts[1]

		switch subsection {
		case "window_naming":
			return getTmuxWindowNamingValue(cfg, field)
		default:
			return nil, fmt.Errorf("unknown subsection: tmux.%s", subsection)
		}
	}

	return nil, fmt.Errorf("invalid tmux key format")
}

// getTmuxWindowNamingValue retrieves a tmux window_naming config value
func getTmuxWindowNamingValue(cfg *config.Config, field string) (interface{}, error) {
	switch field {
	case "max_length":
		return cfg.Tmux.WindowNaming.MaxLength, nil
	case "abbreviate_issue_id":
		return cfg.Tmux.WindowNaming.AbbreviateIssueID, nil
	default:
		return nil, fmt.Errorf("unknown key: tmux.window_naming.%s", field)
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
		return fmt.Errorf("key %q not supported for CLI manipulation\n       Edit config file directly to modify hooks or project_overrides", key)
	}

	section := parts[0]

	switch section {
	case "worktree":
		if len(parts) != 2 {
			return fmt.Errorf("invalid key format: %q (worktree keys are <section>.<field>)", key)
		}
		return setWorktreeValue(cfg, parts[1], value)
	case "tmux":
		return setTmuxValue(cfg, parts[1:], value)
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

// setTmuxValue sets a tmux config value (supports 2-level and 3-level keys)
func setTmuxValue(cfg *config.Config, parts []string, value string) error {
	if len(parts) == 1 {
		// 2-level key: tmux.layout, tmux.window_name, tmux.attach_on_create
		field := parts[0]
		switch field {
		case "layout":
			cfg.Tmux.Layout = value
			return nil
		case "window_name":
			cfg.Tmux.WindowName = value
			return nil
		case "attach_on_create":
			// Convert string to boolean
			boolValue, err := parseBool(value)
			if err != nil {
				return err
			}
			cfg.Tmux.AttachOnCreate = boolValue
			return nil
		default:
			return fmt.Errorf("unknown key: tmux.%s", field)
		}
	}

	if len(parts) == 2 {
		// 3-level key: tmux.window_naming.*
		subsection := parts[0]
		field := parts[1]

		switch subsection {
		case "window_naming":
			return setTmuxWindowNamingValue(cfg, field, value)
		default:
			return fmt.Errorf("unknown subsection: tmux.%s", subsection)
		}
	}

	return fmt.Errorf("invalid tmux key format")
}

// setTmuxWindowNamingValue sets a tmux window_naming config value
func setTmuxWindowNamingValue(cfg *config.Config, field, value string) error {
	switch field {
	case "max_length":
		// Parse and validate integer value
		intValue, err := parseInt(value, 1, 32)
		if err != nil {
			return err
		}
		cfg.Tmux.WindowNaming.MaxLength = intValue
		return nil
	case "abbreviate_issue_id":
		// Convert string to boolean
		boolValue, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.Tmux.WindowNaming.AbbreviateIssueID = boolValue
		return nil
	default:
		return fmt.Errorf("unknown key: tmux.window_naming.%s", field)
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
	case "tmux":
		return unsetTmuxValue(cfg, parts[1:])
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

// unsetTmuxValue unsets a tmux config value to default (supports 2-level and 3-level keys)
func unsetTmuxValue(cfg *config.Config, parts []string) error {
	if len(parts) == 1 {
		// 2-level key: tmux.layout, tmux.window_name, tmux.attach_on_create
		field := parts[0]
		switch field {
		case "layout":
			cfg.Tmux.Layout = "main-vertical" // default
			return nil
		case "window_name":
			cfg.Tmux.WindowName = "work" // default
			return nil
		case "attach_on_create":
			cfg.Tmux.AttachOnCreate = true // default
			return nil
		default:
			return fmt.Errorf("unknown key: tmux.%s", field)
		}
	}

	if len(parts) == 2 {
		// 3-level key: tmux.window_naming.*
		subsection := parts[0]
		field := parts[1]

		switch subsection {
		case "window_naming":
			return unsetTmuxWindowNamingValue(cfg, field)
		default:
			return fmt.Errorf("unknown subsection: tmux.%s", subsection)
		}
	}

	return fmt.Errorf("invalid tmux key format")
}

// unsetTmuxWindowNamingValue unsets a tmux window_naming config value to default
func unsetTmuxWindowNamingValue(cfg *config.Config, field string) error {
	switch field {
	case "max_length":
		cfg.Tmux.WindowNaming.MaxLength = 16 // default
		return nil
	case "abbreviate_issue_id":
		cfg.Tmux.WindowNaming.AbbreviateIssueID = true // default
		return nil
	default:
		return fmt.Errorf("unknown key: tmux.window_naming.%s", field)
	}
}

// isSupportedKey returns true if key can be manipulated via CLI
func isSupportedKey(key string) bool {
	supportedKeys := map[string]bool{
		"worktree.location":                      true,
		"worktree.dedicated_path":                true,
		"tmux.layout":                            true,
		"tmux.window_name":                       true,
		"tmux.attach_on_create":                  true,
		"tmux.window_naming.max_length":          true,
		"tmux.window_naming.abbreviate_issue_id": true,
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
