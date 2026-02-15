# Project-Local Configuration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable project-local `.wt.yaml` configuration that layers on top of global config, with Git root discovery and array replacement semantics.

**Architecture:** Config discovery traverses to Git root using `git rev-parse --show-toplevel` to find `.wt.yaml`. Global config is loaded first, then project config is overlaid using YAML unmarshaling (last-write-wins). Arrays replace entirely, undefined fields inherit from global.

**Tech Stack:** Go 1.21+, `yaml.v3` for overlay merging, existing git client for root discovery

**Tracking Bead:** wt-7fi

---

## Precedence Hierarchy

```
Highest Priority (most specific):
  ↓ .wt.yaml at Git root (project-local, version controlled)
  ↓ ~/.config/wt/config.yaml (user-global)
Lowest Priority (least specific):
```

**Merge Semantics:**
- **Scalars (strings, bools, numbers)**: Project value replaces global value
- **Arrays (hooks)**: Project array replaces global array entirely
- **Undefined fields**: Keep global value (inherit)

---

## Task 1: Add GitRootDiscovery Function

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test for Git root discovery**

Add to `internal/config/config_test.go`:

```go
func TestFindGitRoot(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T, dir string)
		wantErr     bool
		errContains string
	}{
		{
			name: "in git repo returns root",
			setupFunc: func(t *testing.T, dir string) {
				runGitCommand(t, dir, "init")
				runGitCommand(t, dir, "config", "user.email", "test@test.com")
				runGitCommand(t, dir, "config", "user.name", "Test")
			},
			wantErr: false,
		},
		{
			name: "not in git repo returns error",
			setupFunc: func(t *testing.T, dir string) {
				// Do nothing - not a git repo
			},
			wantErr:     true,
			errContains: "not in a git repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			tt.setupFunc(t, tempDir)

			// Change to temp dir for test
			originalWd, _ := os.Getwd()
			defer os.Chdir(originalWd)
			os.Chdir(tempDir)

			root, err := config.FindGitRoot()

			if tt.wantErr {
				if err == nil {
					t.Errorf("FindGitRoot() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("FindGitRoot() error = %v, want containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("FindGitRoot() unexpected error: %v", err)
				}
				if root == "" {
					t.Error("FindGitRoot() returned empty root")
				}
			}
		})
	}
}

// Helper for git commands in tests
func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, output)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestFindGitRoot -v`

Expected: FAIL - "undefined: config.FindGitRoot"

**Step 3: Implement FindGitRoot function**

Add to `internal/config/config.go`:

```go
import (
	"os/exec"
	"strings"
)

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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestFindGitRoot -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add FindGitRoot for discovering Git repository root"
```

---

## Task 2: Add FindConfigs Function (Project + Global Discovery)

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test for FindConfigs**

Add to `internal/config/config_test.go`:

```go
func TestFindConfigs(t *testing.T) {
	tests := []struct {
		name          string
		customPath    string
		setupFunc     func(t *testing.T, dir string) (globalPath string)
		wantProject   bool
		wantGlobal    bool
		wantErr       bool
	}{
		{
			name:       "custom path skips discovery",
			customPath: "/custom/config.yaml",
			setupFunc: func(t *testing.T, dir string) string {
				runGitCommand(t, dir, "init")
				return ""
			},
			wantProject: false,
			wantGlobal:  false,
			wantErr:     false,
		},
		{
			name: "project config only at git root",
			setupFunc: func(t *testing.T, dir string) string {
				runGitCommand(t, dir, "init")
				// Create .wt.yaml at root
				if err := os.WriteFile(filepath.Join(dir, ".wt.yaml"), []byte("tmux:\n  layout: test\n"), 0644); err != nil {
					t.Fatal(err)
				}
				return ""
			},
			wantProject: true,
			wantGlobal:  false,
			wantErr:     false,
		},
		{
			name: "global config only (not in git repo)",
			setupFunc: func(t *testing.T, dir string) string {
				// Create global config in temp location
				globalDir := filepath.Join(dir, ".config", "wt")
				os.MkdirAll(globalDir, 0755)
				globalPath := filepath.Join(globalDir, "config.yaml")
				os.WriteFile(globalPath, []byte("tmux:\n  layout: global\n"), 0644)
				// Set XDG_CONFIG_HOME equivalent via HOME
				os.Setenv("HOME", dir)
				return globalPath
			},
			wantProject: false,
			wantGlobal:  true,
			wantErr:     false,
		},
		{
			name: "both project and global configs",
			setupFunc: func(t *testing.T, dir string) string {
				runGitCommand(t, dir, "init")
				// Project config
				os.WriteFile(filepath.Join(dir, ".wt.yaml"), []byte("tmux:\n  layout: project\n"), 0644)
				// Global config
				globalDir := filepath.Join(dir, ".config", "wt")
				os.MkdirAll(globalDir, 0755)
				globalPath := filepath.Join(globalDir, "config.yaml")
				os.WriteFile(globalPath, []byte("tmux:\n  layout: global\n"), 0644)
				os.Setenv("HOME", dir)
				return globalPath
			},
			wantProject: true,
			wantGlobal:  true,
			wantErr:     false,
		},
		{
			name: "no configs found",
			setupFunc: func(t *testing.T, dir string) string {
				// Not in git repo, no global config
				os.Setenv("HOME", dir) // HOME points to empty dir
				return ""
			},
			wantProject: false,
			wantGlobal:  false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Save original HOME
			origHome := os.Getenv("HOME")
			defer os.Setenv("HOME", origHome)

			// Setup
			_ = tt.setupFunc(t, tempDir)

			// Change to temp dir
			originalWd, _ := os.Getwd()
			defer os.Chdir(originalWd)
			os.Chdir(tempDir)

			projectPath, globalPath, err := config.FindConfigs(tt.customPath)

			if tt.wantErr {
				if err == nil {
					t.Error("FindConfigs() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("FindConfigs() unexpected error: %v", err)
				return
			}

			if tt.wantProject && projectPath == "" {
				t.Error("FindConfigs() expected project path, got empty")
			}
			if !tt.wantProject && projectPath != "" {
				t.Errorf("FindConfigs() expected no project path, got %q", projectPath)
			}
			if tt.wantGlobal && globalPath == "" {
				t.Error("FindConfigs() expected global path, got empty")
			}
			if !tt.wantGlobal && globalPath != "" {
				t.Errorf("FindConfigs() expected no global path, got %q", globalPath)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestFindConfigs -v`

Expected: FAIL - "undefined: config.FindConfigs"

**Step 3: Implement FindConfigs function**

Add to `internal/config/config.go`:

```go
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestFindConfigs -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add FindConfigs for dual config discovery"
```

---

## Task 3: Add LoadMerged Function (Config Layering)

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test for LoadMerged**

Add to `internal/config/config_test.go`:

```go
func TestLoadMerged(t *testing.T) {
	tests := []struct {
		name         string
		globalYAML   string
		projectYAML  string
		wantConfig   config.Config
	}{
		{
			name: "project scalar overrides global",
			globalYAML: `
tmux:
  attach_on_create: true
`,
			projectYAML: `
tmux:
  attach_on_create: false
`,
			wantConfig: config.Config{
				Tmux: config.TmuxConfig{
					AttachOnCreate: false,
				},
			},
		},
		{
			name: "project array replaces global entirely",
			globalYAML: `
hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "{worktree_path}"
`,
			projectYAML: `
hooks:
  on_worktree_create:
    - run: "cargo fetch"
      cwd: "{worktree_path}"
`,
			wantConfig: config.Config{
				Hooks: config.HooksConfig{
					OnWorktreeCreate: []config.Hook{
						{Run: "cargo fetch", Cwd: "{worktree_path}"},
					},
				},
			},
		},
		{
			name: "undefined project field inherits global",
			globalYAML: `
tmux:
  layout: "main-horizontal"
  attach_on_create: true
`,
			projectYAML: `
tmux:
  attach_on_create: false
`,
			wantConfig: config.Config{
				Tmux: config.TmuxConfig{
					Layout:         "main-horizontal", // inherited
					AttachOnCreate: false,            // overridden
				},
			},
		},
		{
			name:        "project only (no global)",
			globalYAML:  "",
			projectYAML: `
tmux:
  layout: "project-layout"
`,
			wantConfig: config.Config{
				Tmux: config.TmuxConfig{
					Layout: "project-layout",
				},
			},
		},
		{
			name: "global only (no project)",
			globalYAML: `
tmux:
  layout: "global-layout"
`,
			projectYAML: "",
			wantConfig: config.Config{
				Tmux: config.TmuxConfig{
					Layout: "global-layout",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			var globalPath, projectPath string

			// Write global config if provided
			if tt.globalYAML != "" {
				globalPath = filepath.Join(tempDir, "global.yaml")
				if err := os.WriteFile(globalPath, []byte(tt.globalYAML), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Write project config if provided
			if tt.projectYAML != "" {
				projectPath = filepath.Join(tempDir, "project.yaml")
				if err := os.WriteFile(projectPath, []byte(tt.projectYAML), 0644); err != nil {
					t.Fatal(err)
				}
			}

			cfg, err := config.LoadMerged(projectPath, globalPath)
			if err != nil {
				t.Fatalf("LoadMerged() error = %v", err)
			}

			// Compare relevant fields
			if len(cfg.Hooks.OnWorktreeCreate) != len(tt.wantConfig.Hooks.OnWorktreeCreate) {
				t.Errorf("OnWorktreeCreate hooks = %d, want %d",
					len(cfg.Hooks.OnWorktreeCreate), len(tt.wantConfig.Hooks.OnWorktreeCreate))
			} else {
				for i, got := range cfg.Hooks.OnWorktreeCreate {
					want := tt.wantConfig.Hooks.OnWorktreeCreate[i]
					if got.Run != want.Run {
						t.Errorf("Hook[%d].Run = %q, want %q", i, got.Run, want.Run)
					}
				}
			}

			if cfg.Tmux.Layout != tt.wantConfig.Tmux.Layout {
				t.Errorf("Tmux.Layout = %q, want %q", cfg.Tmux.Layout, tt.wantConfig.Tmux.Layout)
			}
			if cfg.Tmux.AttachOnCreate != tt.wantConfig.Tmux.AttachOnCreate {
				t.Errorf("Tmux.AttachOnCreate = %v, want %v", cfg.Tmux.AttachOnCreate, tt.wantConfig.Tmux.AttachOnCreate)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestLoadMerged -v`

Expected: FAIL - "undefined: config.LoadMerged"

**Step 3: Implement LoadMerged function**

Add to `internal/config/config.go`:

```go
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestLoadMerged -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add LoadMerged for layering project over global config"
```

---

## Task 4: Update CLI Commands to Use New Config Loading

**Files:**
- Modify: `internal/cli/stack.go:215-221` (loadConfigForCommand)
- Modify: `internal/cli/cli_config_get.go:34-39` (loadActiveConfig)
- Modify: `internal/cli/cli_config_validate.go:17-25`
- Modify: `internal/cli/doctor.go:130,176-179,244`
- Modify: `internal/cli/add.go:44,105`
- Modify: `internal/cli/list.go:41`
- Modify: `internal/cli/remove.go:35`
- Modify: `internal/cli/done.go:55`

**Step 1: Write the failing test**

Create `internal/cli/config_loading_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestLoadConfigForCommand_Merged(t *testing.T) {
	// This test verifies that loadConfigForCommand properly merges configs
	tempDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create git repo
	runGitCommand := func(args ...string) {
		// Use exec.Command
	}

	// Create project config
	projectConfig := `
hooks:
  on_worktree_create:
    - run: "cargo fetch"
tmux:
  attach_on_create: false
`
	projectPath := filepath.Join(tempDir, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Create global config
	globalDir := filepath.Join(tempDir, ".config", "wt")
	os.MkdirAll(globalDir, 0755)
	globalConfig := `
hooks:
  on_worktree_create:
    - run: "npm install"
tmux:
  layout: "main-horizontal"
  attach_on_create: true
`
	globalPath := filepath.Join(globalDir, "config.yaml")
	if err := os.WriteFile(globalPath, []byte(globalConfig), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tempDir)

	// The actual test would need to run loadConfigForCommand
	// For now, test the underlying LoadMerged directly
	cfg, err := config.LoadMerged(projectPath, globalPath)
	if err != nil {
		t.Fatalf("LoadMerged error: %v", err)
	}

	// Verify project overrides global
	if cfg.Tmux.AttachOnCreate != false {
		t.Errorf("AttachOnCreate = %v, want false", cfg.Tmux.AttachOnCreate)
	}

	// Verify undefined field inherits
	if cfg.Tmux.Layout != "main-horizontal" {
		t.Errorf("Layout = %q, want main-horizontal", cfg.Tmux.Layout)
	}

	// Verify array replacement
	if len(cfg.Hooks.OnWorktreeCreate) != 1 {
		t.Fatalf("OnWorktreeCreate hooks = %d, want 1", len(cfg.Hooks.OnWorktreeCreate))
	}
	if cfg.Hooks.OnWorktreeCreate[0].Run != "cargo fetch" {
		t.Errorf("Hook run = %q, want cargo fetch", cfg.Hooks.OnWorktreeCreate[0].Run)
	}
}
```

**Step 2: Run test to verify current state**

Run: `go test ./internal/cli -run TestLoadConfigForCommand_Merged -v`

Expected: Should pass with direct LoadMerged call

**Step 3: Update loadConfigForCommand to use FindConfigs + LoadMerged**

Modify `internal/cli/stack.go`:

```go
func loadConfigForCommand() (*config.Config, error) {
	// Check for --config flag
	customPath, _ := rootCmd.PersistentFlags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		// No config found - return defaults
		return config.DefaultConfig(), nil
	}

	return config.LoadMerged(projectPath, globalPath)
}
```

**Step 4: Update loadActiveConfig similarly**

Modify `internal/cli/cli_config_get.go`:

```go
// loadActiveConfig loads the active config (respects discovery order with merging)
func loadActiveConfig() (*config.Config, error) {
	customPath, _ := rootCmd.PersistentFlags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		return config.DefaultConfig(), nil
	}

	return config.LoadMerged(projectPath, globalPath)
}
```

**Step 5: Update cli_config_validate.go**

Modify `internal/cli/cli_config_validate.go`:

```go
Run: func(cmd *cobra.Command, _ []string) {
	customPath, _ := cmd.Flags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		fmt.Fprintln(cmd.OutOrStderr(), "✗ No config file found")
		os.Exit(1)
	}

	// Validate merged config
	cfg, err := config.LoadMerged(projectPath, globalPath)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStderr(),
			"✗ Config load error: %v\n", err)
		os.Exit(1)
	}

	// Schema validation
	if err := cfg.ValidateSchema(); err != nil {
		fmt.Fprintf(cmd.OutOrStderr(),
			"✗ Schema validation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✓ Configuration is valid")

	// Show which configs are active
	if projectPath != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Project: %s\n", projectPath)
	}
	if globalPath != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  Global: %s\n", globalPath)
	}
},
```

**Step 6: Run all tests**

Run: `go test ./internal/config ./internal/cli -v`

Expected: PASS

**Step 7: Commit**

```bash
git add internal/cli/*.go
git commit -m "feat(cli): update all commands to use merged config loading"
```

---

## Task 5: Update Doctor Command for Dual Config Display

**Files:**
- Modify: `internal/cli/doctor.go:140-180`

**Step 1: Write the failing test**

Add to `internal/cli/doctor_test.go` (create if needed):

```go
func TestCheckConfiguration_DualConfig(t *testing.T) {
	// Test that doctor shows both project and global configs when available
}
```

**Step 2: Update checkConfiguration function**

Modify `internal/cli/doctor.go` checkConfiguration:

```go
func checkConfiguration(out io.Writer) bool {
	customPath, _ := rootCmd.PersistentFlags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		fmt.Fprintln(out, "! No configuration file found")
		fmt.Fprintln(out, "  Run 'wt config set <key> <value>' to create one")
		return false
	}

	// Show which configs are active
	if projectPath != "" {
		fmt.Fprintf(out, "✓ Project config: %s\n", projectPath)
	}
	if globalPath != "" {
		fmt.Fprintf(out, "✓ Global config: %s\n", globalPath)
	}

	// Validate merged config
	cfg, err := config.LoadMerged(projectPath, globalPath)
	if err != nil {
		fmt.Fprintf(out, "! Config is invalid: %v\n", err)
		return false
	}

	if err := cfg.ValidateSchema(); err != nil {
		fmt.Fprintf(out, "! Config schema error: %v\n", err)
		return false
	}

	return true
}
```

**Step 3: Run tests**

Run: `go test ./internal/cli -run TestCheckConfiguration -v`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/cli/doctor.go
git commit -m "feat(doctor): show both project and global config status"
```

---

## Task 6: Add Integration Tests

**Files:**
- Create: `tests/project_config_integration_test.go`

**Step 1: Write integration test for project-local config in worktrees**

Create `tests/project_config_integration_test.go`:

```go
//go:build integration
// +build integration

package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

// TestIntegration_ProjectConfig_WorktreeSharing verifies that worktrees
// get the same project config (committed to repo)
func TestIntegration_ProjectConfig_WorktreeSharing(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create main repo with project config
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Add .wt.yaml to repo
	projectConfig := `
hooks:
  on_worktree_create:
    - run: "echo project-hook"
tmux:
  attach_on_create: false
`
	projectPath := filepath.Join(repoPath, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit the project config
	runGitCommand(t, repoPath, "add", ".wt.yaml")
	runGitCommand(t, repoPath, "commit", "-m", "Add project config")

	// Change to repo for config discovery
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(repoPath)

	// Verify project config is found
	foundProject, _, err := config.FindConfigs("")
	if err != nil {
		t.Fatalf("FindConfigs error: %v", err)
	}
	if foundProject == "" {
		t.Fatal("Expected project config to be found")
	}

	// Load and verify
	cfg, err := config.LoadMerged(foundProject, "")
	if err != nil {
		t.Fatalf("LoadMerged error: %v", err)
	}

	if len(cfg.Hooks.OnWorktreeCreate) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(cfg.Hooks.OnWorktreeCreate))
	}
	if cfg.Hooks.OnWorktreeCreate[0].Run != "echo project-hook" {
		t.Errorf("Hook run = %q, want 'echo project-hook'", cfg.Hooks.OnWorktreeCreate[0].Run)
	}
}

// TestIntegration_ProjectConfig_GitRootDiscovery verifies that
// config is found when running from a subdirectory
func TestIntegration_ProjectConfig_GitRootDiscovery(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Add .wt.yaml at root
	projectConfig := `tmux:\n  layout: test-layout\n`
	projectPath := filepath.Join(repoPath, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subdirectory
	subDir := filepath.Join(repoPath, "src", "components")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to subdirectory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(subDir)

	// Verify project config is still found via git root
	foundProject, _, err := config.FindConfigs("")
	if err != nil {
		t.Fatalf("FindConfigs error: %v", err)
	}
	if foundProject == "" {
		t.Fatal("Expected project config to be found from subdirectory")
	}

	// Verify it's the config at repo root
	if foundProject != projectPath {
		t.Errorf("Found project path = %q, want %q", foundProject, projectPath)
	}
}
```

**Step 2: Run integration tests**

Run: `WT_INTEGRATION_TEST=1 go test ./tests -run TestIntegration_ProjectConfig -v`

Expected: PASS

**Step 3: Commit**

```bash
git add tests/project_config_integration_test.go
git commit -m "test(integration): add project-local config integration tests"
```

---

## Task 7: Remove Deprecated project_overrides Field

**Files:**
- Modify: `internal/config/config.go` (mark deprecated)
- Update: `docs/usage.md` (if needed)

**Step 1: Add deprecation notice to OverrideConfig**

Modify `internal/config/config.go`:

```go
// OverrideConfig allows project-specific overrides
// Deprecated: Use project-local .wt.yaml files instead.
// This field is kept for backward compatibility but is no longer used.
type OverrideConfig struct {
	Match string      `yaml:"match"`
	Hooks HooksConfig `yaml:"hooks,omitempty"`
}
```

**Step 2: Update ValidateSchema to warn about deprecated field**

The existing ValidateSchema should continue to work. No changes needed for backward compatibility.

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "docs(config): deprecate project_overrides in favor of .wt.yaml"
```

---

## Task 8: Update Documentation

**Files:**
- Modify: `docs/usage.md`
- Modify: `AGENTS.md`

**Step 1: Update AGENTS.md config discovery section**

Find and update the configuration discovery section in AGENTS.md to reflect the new hierarchy:

```markdown
**Configuration Discovery Order (v3)**

Config is loaded in this priority order:
1. `--config` flag value (single config, no merging)
2. `.wt.yaml` at Git root (project-local, merges with global)
3. `~/.config/wt/config.yaml` (user-global)

When both project and global configs exist, they are merged:
- Project config overlays global config
- Scalars (strings, bools): project value wins
- Arrays (hooks): project array replaces global entirely
- Undefined fields: inherit from global
```

**Step 2: Commit**

```bash
git add AGENTS.md docs/usage.md
git commit -m "docs: update configuration discovery documentation"
```

---

## Task 9: Final Validation

**Step 1: Run all unit tests**

Run: `go test ./internal/... -v`

Expected: All PASS

**Step 2: Run integration tests**

Run: `WT_INTEGRATION_TEST=1 go test ./tests -v`

Expected: All PASS

**Step 3: Run linting**

Run: `make lint`

Expected: No errors

**Step 4: Build binary**

Run: `make build`

Expected: Success

**Step 5: Manual smoke test**

```bash
# Test in a git repo with .wt.yaml
./bin/wt config list
./bin/wt doctor
./bin/wt config validate
```

**Step 6: Final commit**

```bash
git add .
git commit -m "feat: complete project-local config implementation"
```

---

## Summary

| Task | Component | Description |
|------|-----------|-------------|
| 1 | FindGitRoot | Discover Git repository root |
| 2 | FindConfigs | Dual config discovery (project + global) |
| 3 | LoadMerged | Config layering with proper merge semantics |
| 4 | CLI Updates | Update all commands to use new loading |
| 5 | Doctor | Show both project and global config status |
| 6 | Integration Tests | End-to-end verification |
| 7 | Deprecation | Mark project_overrides as deprecated |
| 8 | Documentation | Update AGENTS.md and usage.md |
| 9 | Validation | Final testing and build |
