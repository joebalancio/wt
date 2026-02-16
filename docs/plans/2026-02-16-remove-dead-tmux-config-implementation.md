# Remove Dead Tmux Config Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove dead tmux configuration structs, functions, and documentation while preserving active window-based tmux operations.

**Architecture:** Delete unused config structs (GlobalConfig, TmuxConfig), remove corresponding CLI parser handlers, clean up dead session methods from tmux client, update docs and tests.

**Tech Stack:** Go 1.21+, golangci-lint, standard testing

---

## Task 1: Remove Config Structs

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Remove GlobalConfig struct and Tmux-related structs**

Delete lines 26-29 (GlobalConfig), lines 44-56 (TmuxConfig and TmuxWindowNamingConfig):

```go
// DELETE THIS BLOCK (lines 26-29):
// GlobalConfig contains global settings
type GlobalConfig struct {
	TmuxSessionPrefix string `yaml:"tmux_session_prefix"`
}

// DELETE THIS BLOCK (lines 44-56):
// TmuxConfig contains tmux-specific settings
type TmuxConfig struct {
	Layout         string                 `yaml:"layout,omitempty"`
	WindowName     string                 `yaml:"window_name,omitempty"`
	AttachOnCreate bool                   `yaml:"attach_on_create,omitempty"`
	WindowNaming   TmuxWindowNamingConfig `yaml:"window_naming,omitempty"`
}

// TmuxWindowNamingConfig contains window naming configuration
type TmuxWindowNamingConfig struct {
	MaxLength         int  `yaml:"max_length,omitempty"`
	AbbreviateIssueID bool `yaml:"abbreviate_issue_id,omitempty"`
}
```

**Step 2: Update Config struct to remove Global and Tmux fields**

Replace lines 16-24 with:

```go
// Config represents the main configuration structure
type Config struct {
	Hooks     HooksConfig      `yaml:"hooks"`
	Worktree  WorktreeConfig   `yaml:"worktree"`
	Spice     SpiceConfig      `yaml:"spice"`
	Overrides []OverrideConfig `yaml:"project_overrides,omitempty"`
}
```

**Step 3: Update DefaultConfig() to remove Global and Tmux blocks**

Replace lines 92-115 with:

```go
// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Worktree: WorktreeConfig{
			Location:      "dedicated",
			DedicatedPath: "~/worktrees",
		},
		Spice: SpiceConfig{
			BinaryPath: "", // Empty means not configured
		},
	}
}
```

**Step 4: Update package comment to remove tmux reference**

Replace line 3 with:

```go
// Configuration is loaded from .wt.yaml in the current directory or ~/.config/wt/config.yaml
// following XDG standards, with support for hooks and worktree location modes.
```

**Step 5: Verify build**

Run: `make build`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add internal/config/config.go
git commit -m "refactor(config): remove dead GlobalConfig and TmuxConfig structs (wt-9i2)"
```

---

## Task 2: Remove CLI Config Parser Functions

**Files:**
- Modify: `internal/cli/cli_config_parser.go`

**Step 1: Remove getGlobalValue function**

Delete lines 115-123:

```go
// DELETE THIS BLOCK:
// getGlobalValue retrieves a global config value
func getGlobalValue(cfg *config.Config, field string) (interface{}, error) {
	switch field {
	case "tmux_session_prefix":
		return cfg.Global.TmuxSessionPrefix, nil
	default:
		return nil, fmt.Errorf("unknown key: global.%s", field)
	}
}
```

**Step 2: Remove setGlobalValue function**

Delete lines 215-224:

```go
// DELETE THIS BLOCK:
// setGlobalValue sets a global config value
func setGlobalValue(cfg *config.Config, field, value string) error {
	switch field {
	case "tmux_session_prefix":
		cfg.Global.TmuxSessionPrefix = value
		return nil
	default:
		return fmt.Errorf("unknown key: global.%s", field)
	}
}
```

**Step 3: Remove unsetGlobalValue function**

Delete lines 345-354:

```go
// DELETE THIS BLOCK:
// unsetGlobalValue unsets a global config value to default
func unsetGlobalValue(cfg *config.Config, field string) error {
	switch field {
	case "tmux_session_prefix":
		cfg.Global.TmuxSessionPrefix = "wt-" // default
		return nil
	default:
		return fmt.Errorf("unknown key: global.%s", field)
	}
}
```

**Step 4: Remove getTmuxValue function**

Delete lines 137-168:

```go
// DELETE THIS BLOCK:
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
```

**Step 5: Remove getTmuxWindowNamingValue function**

Delete lines 170-180:

```go
// DELETE THIS BLOCK:
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
```

**Step 6: Remove setTmuxValue function**

Delete lines 247-286:

```go
// DELETE THIS BLOCK:
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
```

**Step 7: Remove setTmuxWindowNamingValue function**

Delete lines 288-310:

```go
// DELETE THIS BLOCK:
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
```

**Step 8: Remove unsetTmuxValue function**

Delete lines 370-404:

```go
// DELETE THIS BLOCK:
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
```

**Step 9: Remove unsetTmuxWindowNamingValue function**

Delete lines 406-418:

```go
// DELETE THIS BLOCK:
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
```

**Step 10: Update GetValue switch statement**

Replace lines 97-112 with:

```go
	switch section {
	case "worktree":
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key format: %q (worktree keys are <section>.<field>)", key)
		}
		return getWorktreeValue(cfg, parts[1])
	default:
		return nil, fmt.Errorf("unknown section: %s", section)
	}
```

**Step 11: Update SetValue switch statement**

Replace lines 197-212 with:

```go
	switch section {
	case "worktree":
		if len(parts) != 2 {
			return fmt.Errorf("invalid key format: %q (worktree keys are <section>.<field>)", key)
		}
		return setWorktreeValue(cfg, parts[1], value)
	default:
		return fmt.Errorf("unknown section: %s", section)
	}
```

**Step 12: Update UnsetValue switch statement**

Replace lines 327-342 with:

```go
	switch section {
	case "worktree":
		if len(parts) != 2 {
			return fmt.Errorf("invalid key format: %q (worktree keys are <section>.<field>)", key)
		}
		return unsetWorktreeValue(cfg, parts[1])
	default:
		return fmt.Errorf("unknown section: %s", section)
	}
```

**Step 13: Update isSupportedKey function**

Replace lines 421-433 with:

```go
// isSupportedKey returns true if key can be manipulated via CLI
func isSupportedKey(key string) bool {
	supportedKeys := map[string]bool{
		"worktree.location":       true,
		"worktree.dedicated_path": true,
	}
	return supportedKeys[key]
}
```

**Step 14: Verify build**

Run: `make build`
Expected: Build succeeds

**Step 15: Commit**

```bash
git add internal/cli/cli_config_parser.go
git commit -m "refactor(cli): remove dead global/tmux config handlers (wt-9i2)"
```

---

## Task 3: Remove Dead Tmux Session Methods

**Files:**
- Modify: `internal/tmux/session.go`

**Step 1: Remove Session struct**

Delete lines 17-21:

```go
// DELETE THIS BLOCK:
// Session represents a tmux session
type Session struct {
	ID   string
	Name string
}
```

**Step 2: Remove ListSessions method**

Delete lines 37-52:

```go
// DELETE THIS BLOCK:
// ListSessions returns all tmux sessions
func (c *Client) ListSessions() ([]Session, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(c.tmuxPath, "list-sessions", "-F", "#{session_id} #{session_name}")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// tmux returns error if no sessions exist
		if strings.Contains(err.Error(), "no server running") {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	return parseSessionList(stdout.String())
}
```

**Step 3: Remove HasSession method**

Delete lines 54-67:

```go
// DELETE THIS BLOCK:
// HasSession checks if a session with the given name exists
func (c *Client) HasSession(name string) (bool, error) {
	sessions, err := c.ListSessions()
	if err != nil {
		return false, err
	}

	for _, s := range sessions {
		if s.Name == name {
			return true, nil
		}
	}
	return false, nil
}
```

**Step 4: Remove CreateSession method**

Delete lines 69-90:

```go
// DELETE THIS BLOCK:
// CreateSession creates a new tmux session
func (c *Client) CreateSession(name, path, layout, windowName string, attach bool) error {
	args := []string{"new-session", "-d", "-s", name, "-c", path, "-n", windowName}

	cmd := exec.Command(c.tmuxPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	if layout != "" {
		cmd = exec.Command(c.tmuxPath, "select-layout", "-t", name, layout)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("setting layout: %w", err)
		}
	}

	if attach {
		return c.AttachSession(name)
	}

	return nil
}
```

**Step 5: Remove AttachSession method**

Delete lines 92-101:

```go
// DELETE THIS BLOCK:
// AttachSession attaches to an existing session
func (c *Client) AttachSession(name string) error {
	cmd := exec.Command(c.tmuxPath, "attach-session", "-t", name)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// This typically replaces the current process
	return cmd.Run()
}
```

**Step 6: Remove KillSession method**

Delete lines 103-110:

```go
// DELETE THIS BLOCK:
// KillSession kills a tmux session
func (c *Client) KillSession(name string) error {
	cmd := exec.Command(c.tmuxPath, "kill-session", "-t", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("killing session: %w", err)
	}
	return nil
}
```

**Step 7: Remove parseSessionList helper**

Delete lines 112-133:

```go
// DELETE THIS BLOCK:
func parseSessionList(output string) ([]Session, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	sessions := make([]Session, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		sessions = append(sessions, Session{
			ID:   parts[0],
			Name: parts[1],
		})
	}

	return sessions, nil
}
```

**Step 8: Update package comment**

Replace line 3 with:

```go
// It handles window management with smart naming, and collision-resistant window names using branch name hashing.
```

**Step 9: Verify build**

Run: `make build`
Expected: Build succeeds

**Step 10: Commit**

```bash
git add internal/tmux/session.go
git commit -m "refactor(tmux): remove dead session methods (wt-9i2)"
```

---

## Task 4: Update Example Config File

**Files:**
- Modify: `.wt.yaml.example`

**Step 1: Remove global section and tmux section**

Replace the entire file content with:

```yaml
# Example wt GLOBAL configuration file
# Copy to ~/.config/wt/config.yaml
#
# For project-specific settings, create a .wt.yaml file at your repository root.
# Project config overlays global config with these merge semantics:
#   - Scalars (strings, bools): project value replaces global
#   - Arrays (hooks): project array replaces global entirely
#   - Undefined fields: inherit from global

# Hooks run automatically after worktree operations
# NOTE: These are global defaults. Override in .wt.yaml for project-specific hooks.
hooks:
  on_worktree_create:
    # Example: Install Node.js dependencies
    - run: "npm install"
      cwd: "{worktree_path}"
      background: false

    # Example: Run build in background
    - run: "npm run build"
      cwd: "{worktree_path}"
      background: true

  on_worktree_remove:
    # Example: Clean up on remove
    - run: "rm -rf node_modules"
      cwd: "{worktree_path}"

# Git-spice configuration for branch stacking
spice:
  # Path to git-spice binary. Set this via 'wt init' for auto-detection.
  # If empty, stacking commands will prompt you to run 'wt init'.
  binary_path: ""  # Example: "/usr/local/bin/git-spice" or "C:\\Users\\you\\.cargo\\bin\\git-spice.exe"

# Worktree settings
worktree:
  # Location mode: "dedicated" (outside repo) or "per-repo" (inside repo)
  location: "dedicated"

  # Custom path for dedicated mode (default: ~/worktrees)
  dedicated_path: "~/worktrees"

# -----------------------------------------------------------------------------
# PROJECT-LOCAL CONFIG (.wt.yaml)
# -----------------------------------------------------------------------------
# For project-specific settings, create a .wt.yaml file at your repository root.
# This file is committed to version control and shared with your team.
#
# Example .wt.yaml for a Rust project:
#
# hooks:
#   on_worktree_create:
#     - run: "cargo fetch"
#       cwd: "{worktree_path}"
#     - run: "cargo build"
#       cwd: "{worktree_path}"
#       background: true
#
# worktree:
#   location: "per-repo"  # Use .worktrees/ inside repo
#
# The project config will overlay these settings on top of your global config.
# Arrays like hooks are replaced entirely (not merged), so define all hooks
# you need in the project config.
# -----------------------------------------------------------------------------
```

**Step 2: Commit**

```bash
git add .wt.yaml.example
git commit -m "docs: remove dead tmux config from example file (wt-9i2)"
```

---

## Task 5: Update AGENTS.md Documentation

**Files:**
- Modify: `AGENTS.md`

**Step 1: Remove tmux example from config unset section**

Find and remove line containing:
```markdown
wt config unset tmux.attach_on_create
```

**Step 2: Remove tmux-related supported keys**

Find the "Supported keys" section and remove:
```markdown
- `global.tmux_session_prefix` - Tmux session prefix
- `tmux.layout` - Default tmux layout
- `tmux.window_name` - Default tmux window name
- `tmux.attach_on_create` - Attach to tmux on worktree creation (boolean)
- `tmux.window_naming.max_length` - Maximum length for tmux window names (integer, 1-32)
- `tmux.window_naming.abbreviate_issue_id` - Abbreviate issue IDs in window names (boolean)
```

Keep only:
```markdown
**Supported keys:**
- `worktree.location` - Worktree location mode (dedicated/per-repo)
- `worktree.dedicated_path` - Path for dedicated mode
```

**Step 3: Commit**

```bash
git add AGENTS.md
git commit -m "docs: remove dead tmux config from AGENTS.md (wt-9i2)"
```

---

## Task 6: Update docs/usage.md Documentation

**Files:**
- Modify: `docs/usage.md`

**Step 1: Remove tmux config table entries**

Find the config table and remove rows for:
- `global.tmux_session_prefix`
- `tmux.layout`
- `tmux.window_name`
- `tmux.attach_on_create`
- `tmux.window_naming.max_length`
- `tmux.window_naming.abbreviate_issue_id`

**Step 2: Remove tmux example commands**

Find and remove any example commands using tmux.* keys.

**Step 3: Commit**

```bash
git add docs/usage.md
git commit -m "docs: remove dead tmux config from usage.md (wt-9i2)"
```

---

## Task 7: Update Test Files

**Files:**
- Modify: `internal/cli/cli_config_parser_test.go`
- Modify: `internal/cli/cli_config_get_test.go`
- Modify: `internal/cli/cli_config_set_test.go`
- Modify: `internal/cli/cli_config_unset_test.go`
- Modify: `internal/cli/cli_config_integration_test.go`

**Step 1: Update cli_config_parser_test.go**

Remove all test cases referencing:
- `tmux.*` keys
- `global.tmux_session_prefix`
- `TmuxSessionPrefix`
- `TestSetValueWindowNaming` entire test function
- `TestUnsetWindowNaming` entire test function
- `TestDefaultValue` assertion for TmuxSessionPrefix

**Step 2: Update cli_config_get_test.go**

Remove test case for `tmux.layout`.

**Step 3: Update cli_config_set_test.go**

Remove test cases for `tmux.layout`.

**Step 4: Update cli_config_unset_test.go**

Remove test cases for `tmux.layout`.

**Step 5: Update cli_config_integration_test.go**

Remove:
- tmux entries from default values table
- SetValue tests for `tmux.attach_on_create`
- TmuxSessionPrefix assertion

**Step 6: Run tests to verify**

Run: `make test`
Expected: All tests pass

**Step 7: Commit**

```bash
git add internal/cli/*_test.go
git commit -m "test: remove dead tmux config test cases (wt-9i2)"
```

---

## Task 8: Final Verification

**Files:**
- None (verification only)

**Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass

**Step 2: Build binary**

Run: `make build`
Expected: Build succeeds

**Step 3: Verify config list shows no tmux keys**

Run: `./bin/wt config list`
Expected: No `tmux.*` or `global.*` keys shown

**Step 4: Verify unknown key error**

Run: `./bin/wt config get tmux.layout`
Expected: Error message about unknown key

**Step 5: Run linter**

Run: `make lint`
Expected: No errors

---

## Summary

| Task | Description | Commit Message |
|------|-------------|----------------|
| 1 | Remove Config Structs | `refactor(config): remove dead GlobalConfig and TmuxConfig structs (wt-9i2)` |
| 2 | Remove CLI Parser Functions | `refactor(cli): remove dead global/tmux config handlers (wt-9i2)` |
| 3 | Remove Tmux Session Methods | `refactor(tmux): remove dead session methods (wt-9i2)` |
| 4 | Update Example Config | `docs: remove dead tmux config from example file (wt-9i2)` |
| 5 | Update AGENTS.md | `docs: remove dead tmux config from AGENTS.md (wt-9i2)` |
| 6 | Update docs/usage.md | `docs: remove dead tmux config from usage.md (wt-9i2)` |
| 7 | Update Test Files | `test: remove dead tmux config test cases (wt-9i2)` |
| 8 | Final Verification | N/A |
