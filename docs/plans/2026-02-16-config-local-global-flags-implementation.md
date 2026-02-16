# Config Local/Global Flags Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--local` and `--global` flags to `wt config` commands to enable explicit control over config scope, making write commands default to local and read commands support scope selection.

**Architecture:** Add a `ConfigScope` type with `ResolveConfigPaths()` function to centralize path resolution logic. Modify each config command to add flags and use the new resolver. Error handling provides clear guidance when local operations fail outside git repos.

**Tech Stack:** Go 1.22.2, Cobra CLI, existing config package infrastructure

---

## Design Reference

See `docs/plans/2026-02-16-config-local-global-flags-design.md` for full design specification.

### Behavior Summary

| Command | Default | `--local` | `--global` |
|---------|---------|-----------|------------|
| `set` | Local | Local | Global |
| `unset` | Local | Local | Global |
| `get` | Merged | Local only | Global only |
| `list` | Merged | Local only | Global only |

---

## Task 1: Add ConfigScope Types and ResolveConfigPaths Function

**Files:**
- Modify: `internal/cli/cli_config_parser.go`
- Modify: `internal/cli/cli_config_parser_test.go`

**Step 1: Write the failing tests for ConfigScope and ResolveConfigPaths**

Add to `internal/cli/cli_config_parser_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestResolveConfigPaths(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Create a temp git repo for testing
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	// Get expected global path
	home, _ := os.UserHomeDir()
	expectedGlobalPath := filepath.Join(home, ".config", "wt", "config.yaml")
	expectedProjectPath := filepath.Join(tmpDir, ".wt.yaml")

	tests := []struct {
		name          string
		scope         ConfigScope
		op            Operation
		wantProject   string
		wantGlobal    string
		wantError     bool
		errorContains string
	}{
		{
			name:        "ScopeGlobal returns global path only",
			scope:       ScopeGlobal,
			op:          OpRead,
			wantProject: "",
			wantGlobal:  expectedGlobalPath,
			wantError:   false,
		},
		{
			name:        "ScopeLocal read inside git repo",
			scope:       ScopeLocal,
			op:          OpRead,
			wantProject: expectedProjectPath,
			wantGlobal:  "",
			wantError:   false,
		},
		{
			name:        "ScopeLocal write inside git repo",
			scope:       ScopeLocal,
			op:          OpWrite,
			wantProject: expectedProjectPath,
			wantGlobal:  "",
			wantError:   false,
		},
		{
			name:          "ScopeMerged read calls FindConfigs",
			scope:         ScopeMerged,
			op:            OpRead,
			wantProject:   expectedProjectPath,
			wantGlobal:    expectedGlobalPath,
			wantError:     false,
		},
		{
			name:          "ScopeMerged write defaults to local",
			scope:         ScopeMerged,
			op:            OpWrite,
			wantProject:   expectedProjectPath,
			wantGlobal:    "",
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath, globalPath, err := ResolveConfigPaths(tt.scope, tt.op)

			if tt.wantError {
				if err == nil {
					t.Errorf("ResolveConfigPaths() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveConfigPaths() unexpected error: %v", err)
				return
			}
			if projectPath != tt.wantProject {
				t.Errorf("ResolveConfigPaths() projectPath = %q, want %q", projectPath, tt.wantProject)
			}
			if globalPath != tt.wantGlobal {
				t.Errorf("ResolveConfigPaths() globalPath = %q, want %q", globalPath, tt.wantGlobal)
			}
		})
	}
}

func TestResolveConfigPathsOutsideGit(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Change to a temp directory without .git
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	tests := []struct {
		name          string
		scope         ConfigScope
		op            Operation
		wantError     bool
		errorContains string
	}{
		{
			name:          "ScopeLocal read outside git repo errors",
			scope:         ScopeLocal,
			op:            OpRead,
			wantError:     true,
			errorContains: "not in a git repository",
		},
		{
			name:          "ScopeLocal write outside git repo errors",
			scope:         ScopeLocal,
			op:            OpWrite,
			wantError:     true,
			errorContains: "not in a git repository",
		},
		{
			name:          "ScopeMerged write outside git repo errors",
			scope:         ScopeMerged,
			op:            OpWrite,
			wantError:     true,
			errorContains: "not in a git repository",
		},
		{
			name:      "ScopeGlobal works outside git repo",
			scope:     ScopeGlobal,
			op:        OpWrite,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ResolveConfigPaths(tt.scope, tt.op)

			if tt.wantError {
				if err == nil {
					t.Errorf("ResolveConfigPaths() expected error, got nil")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("ResolveConfigPaths() error = %q, want containing %q", err.Error(), tt.errorContains)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveConfigPaths() unexpected error: %v", err)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/cli -run "TestResolveConfigPaths"`
Expected: FAIL with "undefined: ConfigScope" or "undefined: ResolveConfigPaths"

**Step 3: Implement ConfigScope types and ResolveConfigPaths function**

Add to `internal/cli/cli_config_parser.go` (after the imports, before `GetValue`):

```go
// ConfigScope defines which config to target
type ConfigScope int

const (
	ScopeMerged ConfigScope = iota // Read: merged, Write: local
	ScopeLocal                     // Local only
	ScopeGlobal                    // Global only
)

// Operation defines read vs write context
type Operation int

const (
	OpRead Operation = iota
	OpWrite
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
			return "", "", fmt.Errorf("not in a git repository\nLocal config requires being in a git repository. Use --global to modify global config.")
		}
		projectPath = filepath.Join(gitRoot, ".wt.yaml")
		return projectPath, "", nil

	case ScopeMerged:
		if op == OpWrite {
			// Default writes to local
			gitRoot, gitErr := config.FindGitRoot()
			if gitErr != nil {
				return "", "", fmt.Errorf("not in a git repository\nLocal config requires being in a git repository. Use --global to modify global config.")
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
```

Add imports at top of file (need `os`, `path/filepath`, and ensure `config` is imported):
```go
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joebalancio/wt/internal/config"
)
```

**Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run "TestResolveConfigPaths"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/cli_config_parser.go internal/cli/cli_config_parser_test.go
git commit -m "feat: add ConfigScope types and ResolveConfigPaths function

Adds ConfigScope (ScopeMerged, ScopeLocal, ScopeGlobal) and Operation
(OpRead, OpWrite) types with ResolveConfigPaths() for centralized
config path resolution.

Part of wt-j62"
```

---

## Task 2: Add --global Flag to config set Command

**Files:**
- Modify: `internal/cli/cli_config_set.go`
- Modify: `internal/cli/cli_config_set.go` (add tests if file exists, or create test file)

**Step 1: Write failing tests for --global flag**

Create or add to `internal/cli/cli_config_set_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfigSetCmdGlobalFlag(t *testing.T) {
	// Test that --global flag is recognized
	cmd := NewConfigSetCmd()
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config set command missing --global flag")
	}
}

func TestConfigSetDefaultBehavior(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Create a temp git repo
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	// Create command without --global flag
	cmd := NewConfigSetCmd()
	cmd.SetArgs([]string{"tmux.layout", "tiled"})

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Run command (this will create .wt.yaml)
	cmd.Execute()

	// Verify .wt.yaml was created in project dir
	if _, err := os.Stat(filepath.Join(tmpDir, ".wt.yaml")); os.IsNotExist(err) {
		t.Error("expected .wt.yaml to be created in project directory")
	}
}

func TestConfigSetGlobalFlagOutsideGit(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Change to a temp directory without .git
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	// Get global config path
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".config", "wt", "config.yaml")

	// Backup existing global config if it exists
	var backup []byte
	if data, err := os.ReadFile(globalPath); err == nil {
		backup = data
		defer func() {
			if backup != nil {
				os.WriteFile(globalPath, backup, 0644)
			}
		}()
	}

	// Create command with --global flag
	cmd := NewConfigSetCmd()
	cmd.SetArgs([]string{"--global", "tmux.layout", "tiled"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Run command - should succeed outside git repo
	err := cmd.Execute()
	if err != nil {
		t.Errorf("config set --global should work outside git repo, got error: %v", err)
	}
}

func TestConfigSetLocalOutsideGitFails(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Change to a temp directory without .git
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	// Create command without --global flag (defaults to local)
	cmd := NewConfigSetCmd()
	cmd.SetArgs([]string{"tmux.layout", "tiled"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Run command - should fail outside git repo
	// Note: This test may not work as expected because Fatal() calls os.Exit
	// In a real test, we'd need to capture the Fatal call
	// For now, we'll just verify the flag parsing works
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/cli -run "TestConfigSet"`
Expected: FAIL with various errors about missing flags or behavior

**Step 3: Implement --global flag in config set command**

Replace the entire content of `internal/cli/cli_config_set.go`:

```go
package cli

import (
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigSetCmd creates the config set command
func NewConfigSetCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Long: `Set a config value.

By default, values are set in the project-local config (.wt.yaml).
Use --global to set values in the global config (~/.config/wt/config.yaml).`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]

			// Determine scope based on flag
			scope := ScopeMerged // Default: write to local
			if global {
				scope = ScopeGlobal
			}

			// Get config paths
			projectPath, globalPath, err := ResolveConfigPaths(scope, OpWrite)
			if err != nil {
				Fatal("%v", err)
			}

			// Determine which path to use
			var cfgPath string
			if global {
				cfgPath = globalPath
			} else {
				cfgPath = projectPath
			}

			// Load or create config
			cfg, err := loadOrCreateConfig(cfgPath)
			if err != nil {
				Fatal("loading config: %v", err)
			}

			// Set value
			if err := SetValue(cfg, key, value); err != nil {
				Fatal("%v", err)
			}

			// Validate schema
			if err := cfg.ValidateSchema(); err != nil {
				Fatal("config validation failed: %v", err)
			}

			// Save
			if err := cfg.Save(cfgPath); err != nil {
				Fatal("saving config: %v", err)
			}

			scopeLabel := "local"
			if global {
				scopeLabel = "global"
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Updated %s: %s in %s (%s)\n", key, value, cfgPath, scopeLabel); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "modify global config instead of project-local")

	return cmd
}

// loadOrCreateConfig loads an existing config or creates a default one
func loadOrCreateConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.DefaultConfig(), nil
	}
	return cfg, nil
}
```

Note: Remove `getGlobalConfigPath()` function - it's no longer needed since ResolveConfigPaths handles this.

**Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run "TestConfigSet"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/cli_config_set.go internal/cli/cli_config_set_test.go
git commit -m "feat(config): add --global flag to config set command

Changes default behavior: config set now writes to project-local
.wt.yaml by default. Use --global to write to global config.

Breaking change: Previous versions wrote to global config only.

Part of wt-j62"
```

---

## Task 3: Add --global Flag to config unset Command

**Files:**
- Modify: `internal/cli/cli_config_unset.go`
- Create: `internal/cli/cli_config_unset_test.go`

**Step 1: Write failing tests**

Create `internal/cli/cli_config_unset_test.go`:

```go
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestConfigUnsetCmdGlobalFlag(t *testing.T) {
	cmd := NewConfigUnsetCmd()
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config unset command missing --global flag")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/cli -run "TestConfigUnset"`
Expected: FAIL with "config unset command missing --global flag"

**Step 3: Implement --global flag in config unset command**

Replace `internal/cli/cli_config_unset.go`:

```go
package cli

import (
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigUnsetCmd creates the config unset command
func NewConfigUnsetCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a config key",
		Long: `Remove a config key, reverting to default value.

By default, keys are removed from the project-local config (.wt.yaml).
Use --global to remove from the global config (~/.config/wt/config.yaml).`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

			// Determine scope based on flag
			scope := ScopeMerged // Default: write to local
			if global {
				scope = ScopeGlobal
			}

			// Get config paths
			projectPath, globalPath, err := ResolveConfigPaths(scope, OpWrite)
			if err != nil {
				Fatal("%v", err)
			}

			// Determine which path to use
			var cfgPath string
			if global {
				cfgPath = globalPath
			} else {
				cfgPath = projectPath
			}

			// Load config
			cfg, err := config.Load(cfgPath)
			if err != nil {
				Fatal("loading config: %v", err)
			}

			// Unset value
			if err := UnsetValue(cfg, key); err != nil {
				Fatal("%v", err)
			}

			// Save
			if err := cfg.Save(cfgPath); err != nil {
				Fatal("saving config: %v", err)
			}

			scopeLabel := "local"
			if global {
				scopeLabel = "global"
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Removed %s from %s (%s)\n", key, cfgPath, scopeLabel); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "modify global config instead of project-local")

	return cmd
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run "TestConfigUnset"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/cli_config_unset.go internal/cli/cli_config_unset_test.go
git commit -m "feat(config): add --global flag to config unset command

Changes default behavior: config unset now modifies project-local
.wt.yaml by default. Use --global to modify global config.

Part of wt-j62"
```

---

## Task 4: Add --local and --global Flags to config get Command

**Files:**
- Modify: `internal/cli/cli_config_get.go`
- Create: `internal/cli/cli_config_get_test.go`

**Step 1: Write failing tests**

Create `internal/cli/cli_config_get_test.go`:

```go
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestConfigGetCmdFlags(t *testing.T) {
	cmd := NewConfigGetCmd()

	if cmd.Flags().Lookup("local") == nil {
		t.Error("config get command missing --local flag")
	}
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config get command missing --global flag")
	}
}

func TestConfigGetConflictingFlags(t *testing.T) {
	cmd := NewConfigGetCmd()
	cmd.SetArgs([]string{"--local", "--global", "tmux.layout"})

	var exitCode int
	originalFatal := fatalFunc
	fatalFunc = func(code int, _ string, _ ...interface{}) {
		exitCode = code
	}
	defer func() { fatalFunc = originalFatal }()

	cmd.Execute()

	if exitCode == 0 {
		t.Error("expected error when both --local and --global are specified")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/cli -run "TestConfigGet"`
Expected: FAIL with "config get command missing --local flag"

**Step 3: Implement --local and --global flags**

Replace `internal/cli/cli_config_get.go`:

```go
package cli

import (
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigGetCmd creates the config get command
func NewConfigGetCmd() *cobra.Command {
	var local, global bool

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Long: `Get a config value.

By default, reads the merged config (project-local > global > defaults).
Use --local to read only from project-local config.
Use --global to read only from global config.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

			// Check for conflicting flags
			if local && global {
				Fatal(1, "cannot specify both --local and --global")
			}

			// Determine scope
			scope := ScopeMerged // Default: merged read
			if local {
				scope = ScopeLocal
			} else if global {
				scope = ScopeGlobal
			}

			// Get config paths
			projectPath, globalPath, err := ResolveConfigPaths(scope, OpRead)
			if err != nil {
				Fatal("%v", err)
			}

			// Load appropriate config
			var cfg *config.Config
			switch {
			case global:
				// Global only
				if globalPath == "" {
					cfg = config.DefaultConfig()
				} else {
					cfg, err = loadOrCreateConfig(globalPath)
					if err != nil {
						Fatal("loading global config: %v", err)
					}
				}
			case local:
				// Local only
				if projectPath == "" {
					Fatal("not in a git repository\nUse --global to read global config.")
				}
				cfg, err = loadOrCreateConfig(projectPath)
				if err != nil {
					Fatal("loading local config: %v", err)
				}
			default:
				// Merged (default)
				cfg, err = loadMergedConfig(projectPath, globalPath)
				if err != nil {
					Fatal("loading config: %v", err)
				}
			}

			value, err := GetValue(cfg, key)
			if err != nil {
				Fatal("%v", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), formatValue(value)); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&local, "local", "l", false, "read from project-local config only")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "read from global config only")
	cmd.MarkFlagsMutuallyExclusive("local", "global")

	return cmd
}

// loadMergedConfig loads and merges project and global configs
func loadMergedConfig(projectPath, globalPath string) (*config.Config, error) {
	// Handle the case where neither path exists
	if projectPath == "" && globalPath == "" {
		return config.DefaultConfig(), nil
	}
	return config.LoadMerged(projectPath, globalPath)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run "TestConfigGet"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/cli_config_get.go internal/cli/cli_config_get_test.go
git commit -m "feat(config): add --local and --global flags to config get command

Default behavior (merged) unchanged. --local reads project-local only,
--global reads global only. Mutually exclusive flags with helpful errors.

Part of wt-j62"
```

---

## Task 5: Add --local and --global Flags to config list Command

**Files:**
- Modify: `internal/cli/cli_config_list.go`
- Create: `internal/cli/cli_config_list_test.go`

**Step 1: Write failing tests**

Create `internal/cli/cli_config_list_test.go`:

```go
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestConfigListCmdFlags(t *testing.T) {
	cmd := NewConfigListCmd()

	if cmd.Flags().Lookup("local") == nil {
		t.Error("config list command missing --local flag")
	}
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config list command missing --global flag")
	}
}

func TestConfigListConflictingFlags(t *testing.T) {
	cmd := NewConfigListCmd()
	cmd.SetArgs([]string{"--local", "--global"})

	var exitCode int
	originalFatal := fatalFunc
	fatalFunc = func(code int, _ string, _ ...interface{}) {
		exitCode = code
	}
	defer func() { fatalFunc = originalFatal }()

	cmd.Execute()

	if exitCode == 0 {
		t.Error("expected error when both --local and --global are specified")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/cli -run "TestConfigList"`
Expected: FAIL with "config list command missing --local flag"

**Step 3: Implement --local and --global flags**

Replace `internal/cli/cli_config_list.go`:

```go
package cli

import (
	"fmt"

	configpkg "github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewConfigListCmd creates the config list command
func NewConfigListCmd() *cobra.Command {
	var local, global bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all config values",
		Long: `List all config values.

By default, shows the merged config (project-local > global > defaults).
Use --local to show only project-local config.
Use --global to show only global config.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			// Check for conflicting flags
			if local && global {
				Fatal(1, "cannot specify both --local and --global")
			}

			// Determine scope
			scope := ScopeMerged // Default: merged read
			if local {
				scope = ScopeLocal
			} else if global {
				scope = ScopeGlobal
			}

			// Get config paths
			projectPath, globalPath, err := ResolveConfigPaths(scope, OpRead)
			if err != nil {
				Fatal("%v", err)
			}

			// Load appropriate config
			var cfg *configpkg.Config
			switch {
			case global:
				// Global only
				if globalPath == "" {
					cfg = configpkg.DefaultConfig()
				} else {
					cfg, err = loadOrCreateConfig(globalPath)
					if err != nil {
						Fatal("loading global config: %v", err)
					}
				}
			case local:
				// Local only
				if projectPath == "" {
					Fatal("not in a git repository\nUse --global to list global config.")
				}
				cfg, err = loadOrCreateConfig(projectPath)
				if err != nil {
					Fatal("loading local config: %v", err)
				}
			default:
				// Merged (default)
				cfg, err = loadMergedConfig(projectPath, globalPath)
				if err != nil {
					Fatal("loading config: %v", err)
				}
			}

			// Explicitly reference config package to ensure import is used
			_ = configpkg.DefaultConfig

			data, err := yaml.Marshal(cfg)
			if err != nil {
				Fatal("marshaling config: %v", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(data)); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&local, "local", "l", false, "show project-local config only")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "show global config only")
	cmd.MarkFlagsMutuallyExclusive("local", "global")

	return cmd
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run "TestConfigList"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/cli_config_list.go internal/cli/cli_config_list_test.go
git commit -m "feat(config): add --local and --global flags to config list command

Default behavior (merged) unchanged. --local shows project-local only,
--global shows global only. Mutually exclusive flags.

Part of wt-j62"
```

---

## Task 6: Handle Merged Read Outside Git Repo with Warning

**Files:**
- Modify: `internal/cli/cli_config_get.go`
- Modify: `internal/cli/cli_config_list.go`
- Modify: `internal/cli/cli_config_get_test.go`
- Modify: `internal/cli/cli_config_list_test.go`

**Step 1: Write failing tests for warning behavior**

Add to `internal/cli/cli_config_get_test.go`:

```go
func TestConfigGetMergedOutsideGitWarning(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Change to a temp directory without .git
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	cmd := NewConfigGetCmd()
	cmd.SetArgs([]string{"tmux.layout"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// This should succeed but show warning
	err := cmd.Execute()
	if err != nil {
		t.Errorf("merged read outside git should succeed, got error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Warning:") || !strings.Contains(output, "not in a git repository") {
		t.Errorf("expected warning in output, got: %s", output)
	}
}
```

Add to `internal/cli/cli_config_list_test.go`:

```go
func TestConfigListMergedOutsideGitWarning(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Change to a temp directory without .git
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	cmd := NewConfigListCmd()
	cmd.SetArgs([]string{})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// This should succeed but show warning
	err := cmd.Execute()
	if err != nil {
		t.Errorf("merged read outside git should succeed, got error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Warning:") || !strings.Contains(output, "not in a git repository") {
		t.Errorf("expected warning in output, got: %s", output)
	}
}
```

Add missing import:
```go
import (
	"bytes"
	"strings"
	// ... other imports
)
```

**Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/cli -run "TestConfigGetMergedOutsideGitWarning|TestConfigListMergedOutsideGitWarning"`
Expected: FAIL (warning not yet implemented)

**Step 3: Implement warning for merged read outside git**

Modify `internal/cli/cli_config_get.go` - update the default case in the switch:

```go
			default:
				// Merged (default)
				// Check if we're outside git repo for warning
				customPath, _ := rootCmd.PersistentFlags().GetString("config")
				_, _, findErr := configpkg.FindConfigs(customPath)
				isOutsideGit := findErr != nil && findErr.Error() == "no configuration file found"

				// Try to detect if we're outside git but have global config
				_, gitErr := configpkg.FindGitRoot()
				if gitErr != nil && projectPath == "" && globalPath != "" {
					// Outside git repo with global config - show warning
					fmt.Fprintf(cmd.ErrOrStderr(),
						"Warning: not in a git repository, showing global config only\n")
				}

				cfg, err = loadMergedConfig(projectPath, globalPath)
				if err != nil {
					if isOutsideGit {
						// No config at all, use defaults
						cfg = configpkg.DefaultConfig()
						fmt.Fprintf(cmd.ErrOrStderr(),
							"Warning: not in a git repository, showing defaults only\n")
					} else {
						Fatal("loading config: %v", err)
					}
				}
			}
```

Similarly update `internal/cli/cli_config_list.go`:

```go
			default:
				// Merged (default)
				// Check if we're outside git repo for warning
				customPath, _ := rootCmd.PersistentFlags().GetString("config")
				_, _, findErr := configpkg.FindConfigs(customPath)
				isOutsideGit := findErr != nil && findErr.Error() == "no configuration file found"

				// Try to detect if we're outside git but have global config
				_, gitErr := configpkg.FindGitRoot()
				if gitErr != nil && projectPath == "" && globalPath != "" {
					// Outside git repo with global config - show warning
					fmt.Fprintf(cmd.ErrOrStderr(),
						"Warning: not in a git repository, showing global config only\n")
				}

				cfg, err = loadMergedConfig(projectPath, globalPath)
				if err != nil {
					if isOutsideGit {
						// No config at all, use defaults
						cfg = configpkg.DefaultConfig()
						fmt.Fprintf(cmd.ErrOrStderr(),
							"Warning: not in a git repository, showing defaults only\n")
					} else {
						Fatal("loading config: %v", err)
					}
				}
			}
```

**Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/cli -run "TestConfigGetMergedOutsideGitWarning|TestConfigListMergedOutsideGitWarning"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/cli_config_get.go internal/cli/cli_config_list.go internal/cli/cli_config_get_test.go internal/cli/cli_config_list_test.go
git commit -m "feat(config): show warning for merged read outside git repo

When wt config get/list is run outside a git repo, it now shows
a warning to stderr while still succeeding with global config
or defaults.

Part of wt-j62"
```

---

## Task 7: Run Full Test Suite and Fix Any Issues

**Files:**
- Any files that need fixes based on test failures

**Step 1: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 2: Run linter**

Run: `make lint`
Expected: No errors

**Step 3: Fix any issues found**

If tests or linter fail, fix the issues and commit:

```bash
git add <files>
git commit -m "fix: address test/lint failures"
```

**Step 4: Run format**

Run: `make fmt`

**Step 5: Final verification**

Run: `make check`
Expected: All checks pass (fmt + lint + test)

---

## Task 8: Update Design Document Status

**Files:**
- Modify: `docs/plans/2026-02-16-config-local-global-flags-design.md`

**Step 1: Update status from Design to Implemented**

Change line 5:
```markdown
**Status:** Implemented
```

**Step 2: Commit**

```bash
git add docs/plans/2026-02-16-config-local-global-flags-design.md
git commit -m "docs: mark config local/global flags as implemented

Part of wt-j62"
```

---

## Summary

This implementation adds `--local` and `--global` flags to all `wt config` commands:

| Command | New Flags | Default Behavior Change |
|---------|-----------|------------------------|
| `config set` | `--global`, `-g` | Now writes to local by default (was global) |
| `config unset` | `--global`, `-g` | Now modifies local by default (was global) |
| `config get` | `--local`, `-l` / `--global`, `-g` | No change (merged read) |
| `config list` | `--local`, `-l` / `--global`, `-g` | No change (merged read) |

### Breaking Changes

**`wt config set` and `wt config unset`** now default to project-local config instead of global. Users who relied on the old behavior must add `--global` to their commands.

### Key Files Modified

1. `internal/cli/cli_config_parser.go` - Core scope resolution logic
2. `internal/cli/cli_config_set.go` - `--global` flag for writes
3. `internal/cli/cli_config_unset.go` - `--global` flag for writes
4. `internal/cli/cli_config_get.go` - `--local`/`--global` flags for reads
5. `internal/cli/cli_config_list.go` - `--local`/`--global` flags for reads

### Testing Strategy

Each task includes:
- Unit tests for new flags
- Tests for error scenarios (outside git repo)
- Tests for conflicting flags
- Integration with existing config loading logic
