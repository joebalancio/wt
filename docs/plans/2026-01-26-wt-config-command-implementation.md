# Implementation Plan: wt config command

**Design:** `docs/plans/2026-01-26-wt-config-command-design.md`
**Issue:** wt-979
**Created:** 2026-01-26

---

## Overview

Implement programmatic configuration management for wt with 5 subcommands: `get`, `set`, `unset`, `list`, `validate`.

---

## File Structure

```
internal/cli/
├── config.go              # Update: Register subcommands
└── config/
    ├── get.go             # New: wt config get
    ├── set.go             # New: wt config set
    ├── unset.go           # New: wt config unset
    ├── list.go            # New: wt config list
    ├── validate.go        # New: wt config validate
    └── parser.go          # New: Dot-notation parser

internal/config/
├── config.go              # Update: Add ValidateSchema()
└── config_test.go         # New: Schema validation tests

internal/cli/config/
├── get_test.go            # New: Command tests
├── set_test.go            # New: Command tests
├── unset_test.go          # New: Command tests
├── list_test.go           # New: Command tests
├── validate_test.go       # New: Command tests
└── parser_test.go         # New: Parser tests
```

---

## Phase 1: Config Package Extensions

### Task 1.1: Add ValidateSchema() to Config

**File:** `internal/config/config.go`

Add a new method that validates config schema (not just syntax):

```go
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
```

**Acceptance:**
- [ ] Method added to `internal/config/config.go`
- [ ] Returns error for invalid `worktree.location` values
- [ ] Returns nil for valid values (including empty string which defaults to "dedicated")

**Test:** `internal/config/config_schema_test.go`

---

## Phase 2: Dot-Notation Parser

### Task 2.1: Create parser package

**File:** `internal/cli/config/parser.go`

Implement dot-notation key parser with these functions:

```go
// GetValue retrieves a value from config using dot-notation key
func GetValue(cfg *config.Config, key string) (interface{}, error)

// SetValue sets a value in config using dot-notation key
func SetValue(cfg *config.Config, key, value string) error

// UnsetValue removes a key from config, reverting to default
func UnsetValue(cfg *config.Config, key string) error

// isSupportedKey returns true if key can be manipulated via CLI
func isSupportedKey(key string) bool

// formatValue converts a value to string for output
func formatValue(v interface{}) string
```

**Key mapping (dot-notation → struct field):**
- `global.worktree_root` → `cfg.Global.WorktreeRoot`
- `global.tmux_session_prefix` → `cfg.Global.TmuxSessionPrefix`
- `worktree.location` → `cfg.Worktree.Location`
- `worktree.dedicated_path` → `cfg.Worktree.DedicatedPath`
- `tmux.layout` → `cfg.Tmux.Layout`
- `tmux.window_name` → `cfg.Tmux.WindowName`
- `tmux.attach_on_create` → `cfg.Tmux.AttachOnCreate`

**Type inference:**
- Booleans: `true`, `false`, `1`, `0`, `yes`, `no` (case-insensitive)
- Enums: `worktree.location` must be `dedicated` or `per-repo`
- All other: string

**Acceptance:**
- [ ] `GetValue` returns correct values for all 7 supported keys
- [ ] `GetValue` returns error for unsupported keys (`hooks.*`, `project_overrides.*`)
- [ ] `SetValue` correctly sets string values
- [ ] `SetValue` correctly converts boolean inputs
- [ ] `SetValue` validates enum values for `worktree.location`
- [ ] `UnsetValue` reverts keys to defaults
- [ ] `formatValue` outputs booleans as `true`/`false`, strings as-is

**Test:** `internal/cli/config/parser_test.go`

---

## Phase 3: CLI Subcommands

### Task 3.1: Implement `wt config get`

**File:** `internal/cli/config/get.go`

```go
package config

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/user/wt/internal/cli"
    "github.com/user/wt/internal/config"
)

func NewGetCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "get <key>",
        Short: "Get a config value",
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            key := args[0]
            cfg, err := loadActiveConfig()
            if err != nil {
                cli.Fatal("loading config: %v", err)
            }

            value, err := GetValue(cfg, key)
            if err != nil {
                cli.Fatal("key %q not found in config", key)
            }

            fmt.Fprintln(cmd.OutOrStdout(), formatValue(value))
        },
    }
}

func loadActiveConfig() (*config.Config, error) {
    configPath, err := config.FindConfig("")
    if err != nil {
        return config.DefaultConfig(), nil
    }
    return config.Load(configPath)
}
```

**Acceptance:**
- [ ] Command registered with parent `configCmd`
- [ ] Outputs value to stdout
- [ ] Exits 1 if key not found
- [ ] Works with all 7 supported keys

**Test:** `internal/cli/config/get_test.go`

---

### Task 3.2: Implement `wt config list`

**File:** `internal/cli/config/list.go`

```go
func NewListCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List all config values",
        Args:  cobra.NoArgs,
        Run: func(cmd *cobra.Command, _ []string) {
            cfg, err := loadActiveConfig()
            if err != nil {
                cli.Fatal("loading config: %v", err)
            }

            data, err := yaml.Marshal(cfg)
            if err != nil {
                cli.Fatal("marshaling config: %v", err)
            }

            fmt.Fprintln(cmd.OutOrStdout(), string(data))
        },
    }
}
```

**Acceptance:**
- [ ] Outputs full config as YAML
- [ ] Respects config discovery order
- [ ] No args required

**Test:** `internal/cli/config/list_test.go`

---

### Task 3.3: Implement `wt config set`

**File:** `internal/cli/config/set.go`

```go
func NewSetCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "set <key> <value>",
        Short: "Set a config value (global config only)",
        Args:  cobra.ExactArgs(2),
        Run: func(cmd *cobra.Command, args []string) {
            key := args[0]
            value := args[1]

            // Always use global config path
            cfgPath := getGlobalConfigPath()
            cfg, err := loadOrCreateConfig(cfgPath)
            if err != nil {
                cli.Fatal("loading config: %v", err)
            }

            // Set value
            if err := SetValue(cfg, key, value); err != nil {
                cli.Fatal("%v", err)
            }

            // Validate schema
            if err := cfg.ValidateSchema(); err != nil {
                cli.Fatal("config validation failed: %v", err)
            }

            // Save
            if err := cfg.Save(cfgPath); err != nil {
                cli.Fatal("saving config: %v", err)
            }

            fmt.Fprintf(cmd.OutOrStdout(),
                "✓ Updated %s: %s in %s\n", key, value, cfgPath)
        },
    }
}

func getGlobalConfigPath() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config", "wt", "config.yaml")
}

func loadOrCreateConfig(path string) (*config.Config, error) {
    cfg, err := config.Load(path)
    if err != nil {
        return config.DefaultConfig(), nil
    }
    return cfg, nil
}
```

**Acceptance:**
- [ ] Sets values in global config only
- [ ] Validates before saving
- [ ] Shows success message with file path
- [ ] Rejects unsupported keys with helpful error
- [ ] Rejects invalid enum values

**Test:** `internal/cli/config/set_test.go`

---

### Task 3.4: Implement `wt config unset`

**File:** `internal/cli/config/unset.go`

```go
func NewUnsetCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "unset <key>",
        Short: "Remove a config key (global config only)",
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            key := args[0]

            cfgPath := getGlobalConfigPath()
            cfg, err := config.Load(cfgPath)
            if err != nil {
                cli.Fatal("loading config: %v", err)
            }

            if err := UnsetValue(cfg, key); err != nil {
                cli.Fatal("%v", err)
            }

            if err := cfg.Save(cfgPath); err != nil {
                cli.Fatal("saving config: %v", err)
            }

            fmt.Fprintf(cmd.OutOrStdout(),
                "✓ Removed %s from %s\n", key, cfgPath)
        },
    }
}
```

**Acceptance:**
- [ ] Removes key from global config
- [ ] Reverts to default value
- [ ] Exits 1 if key not found

**Test:** `internal/cli/config/unset_test.go`

---

### Task 3.5: Implement `wt config validate`

**File:** `internal/cli/config/validate.go`

```go
func NewValidateCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "validate",
        Short: "Validate configuration (YAML + schema)",
        Args:  cobra.NoArgs,
        Run: func(cmd *cobra.Command, _ []string) {
            configPath, err := config.FindConfig("")
            if err != nil {
                fmt.Fprintln(cmd.OutOrStderr(), "✗ No config file found")
                os.Exit(1)
            }

            // Parse YAML
            cfg, err := config.Load(configPath)
            if err != nil {
                fmt.Fprintf(cmd.OutOrStderr(),
                    "✗ YAML syntax error: %v\n", err)
                os.Exit(1)
            }

            // Validate schema
            if err := cfg.ValidateSchema(); err != nil {
                fmt.Fprintf(cmd.OutOrStderr(),
                    "✗ Schema validation failed: %v\n", err)
                os.Exit(1)
            }

            fmt.Fprintf(cmd.OutOrStdout(),
                "✓ Config is valid: %s\n", configPath)
            fmt.Fprintln(cmd.OutOrStdout(), "✓ YAML syntax valid")
            fmt.Fprintln(cmd.OutOrStdout(), "✓ Schema validation passed")
        },
    }
}
```

**Acceptance:**
- [ ] Checks YAML syntax
- [ ] Checks schema validation
- [ ] Reports which file was validated
- [ ] Exits 1 on any error

**Test:** `internal/cli/config/validate_test.go`

---

### Task 3.6: Register subcommands

**File:** `internal/cli/config.go`

Update existing parent command to register subcommands:

```go
var configCmd = &cobra.Command{
    Use:   "config",
    Short: "Manage wt configuration",
    Long:  `Initialize, validate, and view wt configuration files.`,
}

func init() {
    // Register subcommands
    configCmd.AddCommand(
        config.NewGetCmd(),
        config.NewListCmd(),
        config.NewSetCmd(),
        config.NewUnsetCmd(),
        config.NewValidateCmd(),
    )

    RegisterCommand(configCmd)
}
```

**Acceptance:**
- [ ] All 5 subcommands registered
- [ ] `wt config` shows help with subcommands listed
- [ ] `wt config -h` works

---

## Phase 4: Testing

### Task 4.1: Parser unit tests

**File:** `internal/cli/config/parser_test.go`

```go
func TestGetValue(t *testing.T) {
    cfg := config.DefaultConfig()

    tests := []struct {
        key      string
        expected string
        hasError bool
    }{
        {"worktree.location", "dedicated", false},
        {"worktree.dedicated_path", "~/worktrees", false},
        {"tmux.attach_on_create", "true", false},
        {"global.worktree_root", "~/dev/worktrees", false},
        {"invalid.key", "", true},
        {"hooks.on_worktree_create", "", true}, // unsupported
    }

    for _, tt := range tests {
        t.Run(tt.key, func(t *testing.T) {
            value, err := GetValue(cfg, tt.key)
            if tt.hasError {
                if err == nil {
                    t.Errorf("expected error for %q", tt.key)
                }
                return
            }
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if formatValue(value) != tt.expected {
                t.Errorf("got %q, want %q", formatValue(value), tt.expected)
            }
        })
    }
}

func TestSetValue(t *testing.T) {
    tests := []struct {
        name      string
        key       string
        value     string
        wantError bool
    }{
        {"valid string", "worktree.dedicated_path", "/tmp/wt", false},
        {"valid bool", "tmux.attach_on_create", "false", false},
        {"valid enum", "worktree.location", "per-repo", false},
        {"invalid enum", "worktree.location", "invalid", true},
        {"invalid bool", "tmux.attach_on_create", "maybe", true},
        {"unsupported key", "hooks.on_worktree_create", "echo hi", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := config.DefaultConfig()
            err := SetValue(cfg, tt.key, tt.value)
            if (err != nil) != tt.wantError {
                t.Errorf("SetValue() error = %v, wantError %v", err, tt.wantError)
            }
        })
    }
}
```

**Acceptance:**
- [ ] All GetValue tests pass
- [ ] All SetValue tests pass
- [ ] All UnsetValue tests pass

---

### Task 4.2: Command integration tests

**File:** `internal/cli/config/get_test.go`, etc.

Test each command end-to-end with temp config files.

**Acceptance:**
- [ ] `get_test.go` tests pass
- [ ] `set_test.go` tests pass
- [ ] `unset_test.go` tests pass
- [ ] `list_test.go` tests pass
- [ ] `validate_test.go` tests pass

---

### Task 4.3: Schema validation tests

**File:** `internal/config/config_schema_test.go`

```go
func TestValidateSchema(t *testing.T) {
    tests := []struct {
        name    string
        modify  func(*config.Config)
        wantErr bool
    }{
        {"valid default", func(c *config.Config) {}, false},
        {"valid dedicated", func(c *config.Config) {
            c.Worktree.Location = "dedicated"
        }, false},
        {"valid per-repo", func(c *config.Config) {
            c.Worktree.Location = "per-repo"
        }, false},
        {"invalid location", func(c *config.Config) {
            c.Worktree.Location = "invalid"
        }, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := config.DefaultConfig()
            tt.modify(cfg)
            err := cfg.ValidateSchema()
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateSchema() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

**Acceptance:**
- [ ] All schema validation tests pass

---

## Phase 5: Documentation

### Task 5.1: Update usage docs

**File:** `docs/usage.md`

Add section after "Configuration":

```markdown
### wt config

Programmatic configuration management.

**Subcommands:**
- `wt config get <key>` - Get a config value
- `wt config list` - List all config values
- `wt config set <key> <value>` - Set a config value
- `wt config unset <key>` - Remove a config key
- `wt config validate` - Validate config

**Supported keys:**
- `global.worktree_root`
- `global.tmux_session_prefix`
- `worktree.location`
- `worktree.dedicated_path`
- `tmux.layout`
- `tmux.window_name`
- `tmux.attach_on_create`

**Examples:**

```bash
# Get a value
wt config get worktree.location
# Output: dedicated

# Set a value
wt config set worktree.location per-repo
# Output: ✓ Updated worktree.location: per-repo in ~/.config/wt/config.yaml

# Validate config
wt config validate
# Output: ✓ Config is valid: ~/.config/wt/config.yaml
```
```

**Acceptance:**
- [ ] Documentation added
- [ ] Examples are accurate

---

### Task 5.2: Update CLAUDE.md

**File:** `CLAUDE.md`

Add to `Build and Development Commands` section:

```markdown
### wt config command
- `wt config get <key>` - Get config value
- `wt config set <key> <value>` - Set config value (global only)
- `wt config unset <key>` - Remove config key
- `wt config list` - Dump full config YAML
- `wt config validate` - Validate config syntax and schema
```

**Acceptance:**
- [ ] CLAUDE.md updated

---

## Verification Checklist

Before marking wt-979 complete:

- [ ] All 5 subcommands implemented and registered
- [ ] `wt config -h` shows all subcommands
- [ ] All parser functions implemented with type inference
- [ ] `ValidateSchema()` added to config package
- [ ] All unit tests pass (`go test ./internal/...`)
- [ ] All integration tests pass
- [ ] Documentation updated (usage.md, CLAUDE.md)
- [ ] Manual testing completed:
  - [ ] `wt config get worktree.location` works
  - [ ] `wt config set worktree.location per-repo` works
  - [ ] `wt config set worktree.location invalid` fails with helpful error
  - [ ] `wt config unset worktree.dedicated_path` works
  - [ ] `wt config list` outputs YAML
  - [ ] `wt config validate` checks syntax and schema
  - [ ] `wt config get hooks.on_worktree_create` fails with "not supported" message

---

## Order of Implementation

Recommended order (linear dependencies):

1. **Phase 1** - Config package extensions (foundational)
2. **Phase 2** - Parser (needed by all commands)
3. **Phase 3.1-3.2** - Read commands (get, list) - simpler
4. **Phase 3.5** - Validate command (uses parser, no writes)
5. **Phase 3.3-3.4** - Write commands (set, unset) - most complex
6. **Phase 3.6** - Register all subcommands
7. **Phase 4** - Testing
8. **Phase 5** - Documentation

**Total estimated tasks:** 17
**Estimated completion time:** 1-2 days

---

## Dependencies

- Go 1.21+
- Existing `internal/config` package
- Existing `github.com/spf13/cobra` package
- Existing `gopkg.in/yaml.v3` package

**No new external dependencies required.**
