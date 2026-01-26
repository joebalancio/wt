# WT Config Command Design

**Status:** Design Phase
**Author:** WT Team
**Created:** 2026-01-26
**Issue:** wt-979

## Overview

The `wt config` command provides programmatic configuration management, enabling users to view and modify configuration values without manually editing YAML files. This addresses the gap between the stub `wt config` parent command and manual YAML editing.

### Goals

1. **Ergonomic Read API** - Simple dot-notation key lookup
2. **Safe Write API** - Type-aware value setting with validation
3. **Fast Validation** - YAML syntax + schema checks (no slow I/O)
4. **Pragmatic Scope** - Scalar values only, defer hooks to YAML editing

### Non-Goals

- Hook array manipulation (users edit YAML for hooks)
- Project-local config writes (global only in v1)
- Path/dependency checking (use `wt doctor`)
- Config file merging/inheritance display

---

## Command Structure

### Design Philosophy

**Read-heavy, write-careful** - Most operations are reads, writes are explicit and validated.

```
wt config get <key>        # Read single value
wt config list             # Dump full config
wt config set <key> <val>  # Write value (global only)
wt config unset <key>      # Remove key
wt config validate         # Check YAML + schema
```

### Command Reference

| Command | Args | Exit Codes | Description |
|---------|------|------------|-------------|
| `wt config get` | `<key>` | 0=found, 1=not found | Retrieve scalar value |
| `wt config list` | | 0=success | Dump full config YAML |
| `wt config set` | `<key> <value>` | 0=success, 1=error | Set scalar value (global) |
| `wt config unset` | `<key>` | 0=success, 1=not found | Remove key from config |
| `wt config validate` | | 0=valid, 1=invalid | Check YAML + schema |

---

## Key Notation: Dot-Path Syntax

### Syntax Rules

```
<section>.<field>
```

- Dot-separated path to config field
- Matches YAML structure (not Go struct names)
- Case-sensitive as defined in YAML

### Examples

| Dot-Path | YAML Equivalent | Type |
|----------|----------------|------|
| `worktree.location` | `worktree.location` | string |
| `worktree.dedicated_path` | `worktree.dedicated_path` | string |
| `global.tmux_session_prefix` | `global.tmux_session_prefix` | string |
| `tmux.attach_on_create` | `tmux.attach_on_create` | bool |

### Supported Keys (v1)

#### Global Settings
| Key | Type | Default | Valid Values |
|-----|------|---------|--------------|
| `global.worktree_root` | string | `~/dev/worktrees` | Any path |
| `global.tmux_session_prefix` | string | `wt-` | Any string |

#### Worktree Settings
| Key | Type | Default | Valid Values |
|-----|------|---------|--------------|
| `worktree.location` | string | `dedicated` | `dedicated`, `per-repo` |
| `worktree.dedicated_path` | string | `~/worktrees` | Any path |

#### Tmux Settings
| Key | Type | Default | Valid Values |
|-----|------|---------|--------------|
| `tmux.layout` | string | `main-vertical` | tmux layout name |
| `tmux.window_name` | string | `work` | Any string |
| `tmux.attach_on_create` | bool | `true` | `true`, `false` |

#### Not Supported (defer to YAML editing)
- `hooks.*` - Hook arrays
- `project_overrides.*` - Override configurations

---

## Command Specifications

### `wt config get`

Retrieve a scalar configuration value.

**Synopsis:**
```bash
wt config get <key>
```

**Behavior:**
1. Parse key using dot-notation
2. Traverse config struct to find value
3. Output value as plain text to stdout
4. Exit 1 if key not found

**Examples:**
```bash
$ wt config get worktree.location
dedicated

$ wt config get tmux.attach_on_create
true

$ wt config get invalid.key
Error: key "invalid.key" not found in config
```

**Implementation:**
```go
func getConfigGet() *cobra.Command {
    return &cobra.Command{
        Use:   "get <key>",
        Short: "Get a config value",
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            key := args[0]
            cfg, _ := loadConfig()

            value, err := getValue(cfg, key)
            if err != nil {
                Fatal("key %q not found in config", key)
            }

            fmt.Fprintln(cmd.OutOrStdout(), formatValue(value))
        },
    }
}
```

### `wt config list`

Dump the entire active configuration as YAML.

**Synopsis:**
```bash
wt config list
```

**Behavior:**
1. Load active config (respecting discovery order)
2. Marshal to YAML
3. Output to stdout

**Examples:**
```bash
$ wt config list
global:
  worktree_root: ~/dev/worktrees
  tmux_session_prefix: wt-
hooks:
  on_worktree_create: []
tmux:
  layout: main-vertical
  window_name: work
  attach_on_create: true
worktree:
  location: dedicated
  dedicated_path: ~/worktrees
```

**Implementation:**
```go
func getConfigList() *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List all config values",
        Args:  cobra.NoArgs,
        Run: func(cmd *cobra.Command, _ []string) {
            cfg, _ := loadConfig()

            data, err := yaml.Marshal(cfg)
            if err != nil {
                Fatal("marshaling config: %v", err)
            }

            fmt.Fprintln(cmd.OutOrStdout(), string(data))
        },
    }
}
```

### `wt config set`

Set a scalar configuration value (global config only).

**Synopsis:**
```bash
wt config set <key> <value>
```

**Behavior:**
1. Parse key and value
2. Validate key is supported
3. Type-check value
4. Load global config (`~/.config/wt/config.yaml`)
5. Update value
6. Validate schema
7. Save to disk
8. Exit 1 on any error

**Type Inference:**
- `true`, `false` → boolean
- Numeric strings → string (not int, paths are strings)
- All other → string

**Examples:**
```bash
$ wt config set worktree.location per-repo
✓ Updated worktree.location: per-repo in ~/.config/wt/config.yaml

$ wt config set tmux.attach_on_create false
✓ Updated tmux.attach_on_create: false in ~/.config/wt/config.yaml

$ wt config set worktree.location invalid
Error: invalid value "invalid" for worktree.location
       Valid values: dedicated, per-repo

$ wt config set hooks.on_worktree_create "echo hi"
Error: hooks manipulation not supported via CLI
       Edit ~/.config/wt/config.yaml directly to modify hooks
```

**Implementation:**
```go
func getConfigSet() *cobra.Command {
    return &cobra.Command{
        Use:   "set <key> <value>",
        Short: "Set a config value (global config only)",
        Args:  cobra.ExactArgs(2),
        Run: func(cmd *cobra.Command, args []string) {
            key := args[0]
            value := args[1]

            // Load global config
            cfgPath := filepath.Join(os.Getenv("HOME"), ".config", "wt", "config.yaml")
            cfg, err := config.Load(cfgPath)
            if err != nil {
                cfg = config.DefaultConfig()
            }

            // Set value
            if err := setValue(cfg, key, value); err != nil {
                Fatal("%v", err)
            }

            // Validate
            if err := cfg.ValidateSchema(); err != nil {
                Fatal("config validation failed: %v", err)
            }

            // Save
            if err := cfg.Save(cfgPath); err != nil {
                Fatal("saving config: %v", err)
            }

            fmt.Fprintf(cmd.OutOrStdout(),
                "✓ Updated %s: %s in %s\n", key, value, cfgPath)
        },
    }
}
```

### `wt config unset`

Remove a key from the configuration (global only).

**Synopsis:**
```bash
wt config unset <key>
```

**Behavior:**
1. Parse key
2. Load global config
3. Remove key (revert to default if needed)
4. Save to disk
5. Exit 1 if key not found

**Examples:**
```bash
$ wt config unset worktree.dedicated_path
✓ Removed worktree.dedicated_path from ~/.config/wt/config.yaml
# (Reverts to default ~/worktrees)

$ wt config unset invalid.key
Error: key "invalid.key" not found in config
```

**Implementation:**
```go
func getConfigUnset() *cobra.Command {
    return &cobra.Command{
        Use:   "unset <key>",
        Short: "Remove a config key (global config only)",
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            key := args[0]

            cfgPath := filepath.Join(os.Getenv("HOME"), ".config", "wt", "config.yaml")
            cfg, err := config.Load(cfgPath)
            if err != nil {
                Fatal("loading config: %v", err)
            }

            // Unset (revert to default)
            if err := unsetValue(cfg, key); err != nil {
                Fatal("%v", err)
            }

            if err := cfg.Save(cfgPath); err != nil {
                Fatal("saving config: %v", err)
            }

            fmt.Fprintf(cmd.OutOrStdout(),
                "✓ Removed %s from %s\n", key, cfgPath)
        },
    }
}
```

### `wt config validate`

Validate configuration YAML syntax and schema.

**Synopsis:**
```bash
wt config validate
```

**Behavior:**
1. Find active config
2. Parse YAML syntax
3. Validate schema (enum values, required fields)
4. Report results
5. Exit 1 if invalid

**Scope:**
- ✓ YAML syntax errors
- ✓ Schema validation (enums, required fields)
- ✗ Path existence (use `wt doctor`)
- ✗ Dependency checks (use `wt doctor`)

**Examples:**
```bash
$ wt config validate
✓ Config is valid: ~/.config/wt/config.yaml
✓ YAML syntax valid
✓ Schema validation passed

$ wt config validate
✗ Config validation failed: ~/.config/wt/config.yaml
✗ YAML syntax error: line 15: unexpected '{'

$ wt config validate
✗ Config validation failed
✗ Invalid value for worktree.location: "invalid"
  Valid values: dedicated, per-repo
```

**Implementation:**
```go
func getConfigValidate() *cobra.Command {
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

---

## Implementation Details

### Key Parser Design

The dot-notation parser must map `worktree.location` to `cfg.Worktree.Location`.

**Approach 1: Direct Field Mapping (Recommended)**
```go
func getValue(cfg *config.Config, key string) (interface{}, error) {
    parts := strings.Split(key, ".")

    switch parts[0] {
    case "global":
        return getGlobalValue(cfg, parts[1:])
    case "worktree":
        return getWorktreeValue(cfg, parts[1:])
    case "tmux":
        return getTmuxValue(cfg, parts[1:])
    default:
        return nil, fmt.Errorf("unknown section: %s", parts[0])
    }
}

func getWorktreeValue(cfg *config.Config, parts []string) (interface{}, error) {
    if len(parts) != 1 {
        return nil, fmt.Errorf("invalid key path")
    }
    switch parts[0] {
    case "location":
        return cfg.Worktree.Location, nil
    case "dedicated_path":
        return cfg.Worktree.GetDedicatedPath(), nil
    default:
        return nil, fmt.Errorf("unknown key: worktree.%s", parts[0])
    }
}
```

**Approach 2: Reflection (More flexible, more complex)**
```go
func getValueReflect(cfg *config.Config, key string) (interface{}, error) {
    v := reflect.ValueOf(cfg).Elem()
    parts := strings.Split(key, ".")

    // Map YAML names to struct field names
    fieldMap := map[string]map[string]string{
        "worktree": {"location": "Location", "dedicated_path": "DedicatedPath"},
        // ...
    }

    for _, part := range parts {
        // Traverse struct using field map
        // ...
    }

    return value.Interface(), nil
}
```

### Value Setter with Type Inference

```go
func setValue(cfg *config.Config, key, valueStr string) error {
    parts := strings.Split(key, ".")

    // Check if key is supported
    if !isSupportedKey(key) {
        return fmt.Errorf("key %q not supported for CLI manipulation", key)
    }

    // Infer type from known key types
    switch key {
    case "tmux.attach_on_create":
        return setBoolValue(cfg, key, valueStr)
    case "worktree.location":
        return setEnumValue(cfg, key, valueStr, []string{"dedicated", "per-repo"})
    default:
        return setStringValue(cfg, key, valueStr)
    }
}

func setEnumValue(cfg *config.Config, key, value string, valid []string) error {
    for _, v := range valid {
        if value == v {
            setStringValue(cfg, key, value)
            return nil
        }
    }
    return fmt.Errorf("invalid value %q for %s\n       Valid values: %s",
        value, key, strings.Join(valid, ", "))
}

func setBoolValue(cfg *config.Config, key, value string) error {
    switch strings.ToLower(value) {
    case "true", "1", "yes":
        setStringValue(cfg, key, "true")
        return nil
    case "false", "0", "no":
        setStringValue(cfg, key, "false")
        return nil
    default:
        return fmt.Errorf("invalid boolean value: %q", value)
    }
}
```

### Schema Validation Extension

Extend `internal/config/config.go` with schema validation:

```go
// ValidateSchema checks if configuration values are valid
func (c *Config) ValidateSchema() error {
    // Validate worktree.location
    if c.Worktree.Location != "" &&
        c.Worktree.Location != "dedicated" &&
        c.Worktree.Location != "per-repo" {
        return fmt.Errorf("invalid worktree.location: %q (must be 'dedicated' or 'per-repo')",
            c.Worktree.Location)
    }

    // Validate booleans
    if c.Tmux.AttachOnCreate && !reflect.ValueOf(c.Tmux.AttachOnCreate).IsBool() {
        return fmt.Errorf("tmux.attach_on_create must be boolean")
    }

    return nil
}
```

### File Structure

```
internal/cli/
├── config.go              # Existing parent command
└── config/
    ├── get.go             # wt config get
    ├── set.go             # wt config set
    ├── unset.go           # wt config unset
    ├── list.go            # wt config list
    ├── validate.go        # wt config validate
    └── parser.go          # Dot-notation parser
```

---

## Error Messages

All errors should be:
1. **Specific** - What went wrong
2. **Actionable** - How to fix it
3. **Consistent** - Same format across commands

### Format

```bash
# Key not found
Error: key "invalid.key" not found in config

# Invalid value
Error: invalid value "invalid" for worktree.location
       Valid values: dedicated, per-repo

# Unsupported operation
Error: hooks manipulation not supported via CLI
       Edit ~/.config/wt/config.yaml directly to modify hooks

# YAML syntax error
✗ Config validation failed: ~/.config/wt/config.yaml
✗ YAML syntax error: line 15: unexpected '{'
```

---

## Future Enhancements (Post-v1)

### `--local` Flag Support

```bash
wt config set --local worktree.location dedicated
# Writes to .wt/config.yaml instead of ~/.config/wt/config.yaml
```

### Hook Manipulation

```bash
wt config add-hook on_worktree_create "npm install"
wt config remove-hook on_worktree_create 0
wt config list-hooks on_worktree_create
```

### Config Diff

```bash
wt config diff
# Shows differences between config and defaults

wt config diff --local
# Shows local vs global config differences
```

### Output Formats

```bash
wt config get worktree.location --output json
{"key": "worktree.location", "value": "dedicated"}

wt config list --output json
{...}
```

---

## Testing Strategy

### Unit Tests

- `parser_test.go` - Dot-notation parsing
- `setter_test.go` - Type inference, validation
- `validator_test.go` - Schema validation

### Integration Tests

- `config_test.go` - Full command workflows
- Test file creation, modification, validation

### Test Cases

```go
func TestConfigGet(t *testing.T) {
    tests := []struct {
        key      string
        expected string
        hasError bool
    }{
        {"worktree.location", "dedicated", false},
        {"tmux.attach_on_create", "true", false},
        {"invalid.key", "", true},
    }
    // ...
}

func TestConfigSet(t *testing.T) {
    tests := []struct {
        key      string
        value    string
        hasError bool
    }{
        {"worktree.location", "per-repo", false},
        {"worktree.location", "invalid", true},  // invalid enum
        {"hooks.on_worktree_create", "echo hi", true},  // not supported
    }
    // ...
}
```

---

## Related Issues

- **wt-979** - Implement wt config command (this issue)
- **wt-cvr** - Refactor project-local config from .wt.yaml to .wt/config.yaml
- **docs/plans/2025-01-25-wt-v2-stacking-design.md:45** - Original design reference

## References

- **Config package** - `internal/config/config.go`
- **Usage docs** - `docs/usage.md:162`
- **Cobra docs** - https://github.com/spf13/cobra
- **YAML v3** - https://pkg.go.dev/gopkg.in/yaml.v3
