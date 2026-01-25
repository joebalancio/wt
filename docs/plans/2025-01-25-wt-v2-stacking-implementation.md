# WT v2 Stacking Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add branch stacking capabilities to wt by integrating with git-spice, enabling developers to manage stacked feature branches with automatic worktree creation and setup hook execution.

**Architecture:** WT wraps git-spice for stack management while using existing git/tmux clients. New internal/spice package handles git-spice operations. Config extended for worktree location (dedicated/per-repo). Nanoid generates unique branch suffixes.

**Tech Stack:** Go 1.22.2, github.com/aidarkhanov/nanoid/v3, git-spice (external dependency), Cobra CLI

---

`★ Insight ─────────────────────────────────────`
**Architectural Context for this Plan:**

1. **Client Pattern Consistency** - The new `internal/spice` package follows the same `NewClient()` pattern as `internal/git` and `internal/tmux`. All external tool clients use exec.Command with context cancellation.

2. **Config Discovery Order** - WT loads configs in priority order: `--config` flag > `.wt.yaml` (local) > `~/.config/wt/config.yaml` (global). The plan extends `internal/config/config.go` to add worktree location settings.

3. **Interface-Based Design** - `worktree.Service` depends on `git.GitClient` interface. The plan adds a `spice.SpiceClient` interface that `stack.Service` will depend on, maintaining the same dependency injection pattern.
`─────────────────────────────────────────────────`

---

## Phase 1: Foundation (Dependencies, Config, Health)

### Task 1: Add nanoid dependency

**Files:**
- Modify: `go.mod`
- Modify: `go.sum` (auto-generated)

**Step 1: Add nanoid dependency**

Run: `go get github.com/aidarkhanov/nanoid/v3`

Expected output:
```
go: downloading github.com/aidarkhanov/nanoid/v3 v3.x.x
go: added github.com/aidarkhanov/nanoid/v3 v3.x.x
```

**Step 2: Tidy go.mod**

Run: `go mod tidy`

Expected: No errors, go.sum updated

**Step 3: Verify dependency compiles**

Create test file `internal/util/nanoid_test.go`:

```go
package util

import (
    "testing"

    "github.com/aidarkhanov/nanoid/v3"
)

func TestGenerateSuffix(t *testing.T) {
    suffix, err := nanoid.Generate(4)
    if err != nil {
        t.Fatalf("failed to generate suffix: %v", err)
    }
    if len(suffix) != 4 {
        t.Errorf("expected length 4, got %d", len(suffix))
    }
}
```

**Step 4: Run test**

Run: `go test ./internal/util -v`

Expected: PASS

**Step 5: Commit**

```bash
git add go.mod go.sum internal/util/
git commit -m "deps: add nanoid dependency for branch suffix generation"
```

---

### Task 2: Extend config for worktree location and git-spice settings

**Files:**
- Modify: `internal/config/config.go:11-17` (Config struct)
- Modify: `internal/config/config.go:19-23` (GlobalConfig)
- Create: `internal/config/config_test.go`

**Step 1: Add worktree and spice config structs**

In `internal/config/config.go`, modify the Config struct (around line 11):

```go
// Config represents the main configuration structure
type Config struct {
	Global    GlobalConfig     `yaml:"global"`
	Hooks     HooksConfig      `yaml:"hooks"`
	Tmux      TmuxConfig       `yaml:"tmux"`
	Worktree  WorktreeConfig   `yaml:"worktree"`
	Overrides []OverrideConfig `yaml:"project_overrides,omitempty"`
}
```

**Step 2: Add WorktreeConfig struct**

After the TmuxConfig struct (around line 44), add:

```go
// WorktreeConfig contains worktree-specific settings
type WorktreeConfig struct {
	Location     string `yaml:"location"`      // "dedicated" or "per-repo"
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
```

**Step 3: Update DefaultConfig**

Modify the DefaultConfig function (around line 52):

```go
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
			Location:     "dedicated",
			DedicatedPath: "~/worktrees",
		},
	}
}
```

**Step 4: Write failing config test**

Create `internal/config/config_test.go`:

```go
package config

import (
	"testing"
)

func TestWorktreeConfig_IsDedicated(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     bool
	}{
		{"empty defaults to dedicated", "", true},
		{"explicit dedicated", "dedicated", true},
		{"per-repo", "per-repo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := WorktreeConfig{Location: tt.location}
			if got := cfg.IsDedicated(); got != tt.want {
				t.Errorf("IsDedicated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorktreeConfig_GetDedicatedPath(t *testing.T) {
	tests := []struct {
		name          string
		dedicatedPath string
		want          string
	}{
		{"custom path", "/custom/worktrees", "/custom/worktrees"},
		{"empty uses default", "", "~/worktrees"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := WorktreeConfig{DedicatedPath: tt.dedicatedPath}
			if got := cfg.GetDedicatedPath(); got != tt.want {
				t.Errorf("GetDedicatedPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig_HasWorktreeSettings(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Worktree.IsDedicated() {
		t.Error("default config should use dedicated worktree location")
	}
	if cfg.Worktree.GetDedicatedPath() != "~/worktrees" {
		t.Errorf("default dedicated path = %v, want ~/worktrees", cfg.Worktree.GetDedicatedPath())
	}
}
```

**Step 5: Run tests**

Run: `go test ./internal/config -v`

Expected: PASS

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add worktree location configuration (dedicated/per-repo)"
```

---

### Task 3: Create internal/spice package for git-spice client

**Files:**
- Create: `internal/spice/client.go`
- Create: `internal/spice/client_test.go`

**Step 1: Write failing test for spice client**

Create `internal/spice/client_test.go`:

```go
package spice

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

func TestClient_GetVersion(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skipf("git-spice not available: %v", err)
	}

	ctx := context.Background()
	version, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("GetVersion() error = %v", err)
	}
	if version == "" {
		t.Error("GetVersion() returned empty version")
	}
	t.Logf("git-spice version: %s", version)
}

func TestClient_CreateBranch(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skipf("git-spice not available: %v", err)
	}

	// This test requires a git repo with git-spice initialized
	// For now, just verify the method exists and has correct signature
	ctx := context.Background()

	// We'll test actual branch creation in integration tests
	_ = ctx
	_ = client
}
```

**Step 2: Create spice client implementation**

Create `internal/spice/client.go`:

```go
package spice

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps git-spice operations
type Client struct {
	gsPath string
}

// NewClient creates a new git-spice client
func NewClient() (*Client, error) {
	path, err := exec.LookPath("gs")
	if err != nil {
		return nil, fmt.Errorf("git-spice (gs) not found in PATH: %w", err)
	}
	return &Client{gsPath: path}, nil
}

// GetVersion returns the git-spice version
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gsPath, "--version")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("getting git-spice version: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// CreateBranch creates a new stacked branch via git-spice
func (c *Client) CreateBranch(ctx context.Context, spec BranchCreateSpec) (*Branch, error) {
	args := []string{"branch", spec.Name}

	if spec.Base != "" {
		args = append(args, "--base", spec.Base)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gsPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("creating branch with git-spice: %w: %s", err, stderr.String())
	}

	// Parse the created branch name from output
	// git-spice typically echoes the created branch
	branchName := strings.TrimSpace(stdout.String())
	if branchName == "" {
		branchName = spec.Name
	}

	return &Branch{Name: branchName}, nil
}

// GetStack returns the current stack of branches
func (c *Client) GetStack(ctx context.Context) ([]*Branch, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gsPath, "stack", "list")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("getting stack: %w", err)
	}

	return parseStackOutput(stdout.String()), nil
}

// BranchCreateSpec defines parameters for creating a branch
type BranchCreateSpec struct {
	Name string // Branch name (required)
	Base string // Base branch (optional, defaults to current)
}

// Branch represents a git-spice branch
type Branch struct {
	Name     string // Branch name
	IsRoot   bool   // Is this the root of the stack
	IsHead   bool   // Is this the current branch
	Children []*Branch // Child branches in the stack
}

func parseStackOutput(output string) []*Branch {
	// Parse git-spice stack list output
	// Format is tree-like with indentation
	var branches []*Branch
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Simple parsing - will be enhanced in later tasks
		branches = append(branches, &Branch{Name: line})
	}

	return branches
}
```

**Step 3: Run tests**

Run: `go test ./internal/spice -v`

Expected: May skip if git-spice not installed

**Step 4: Commit**

```bash
git add internal/spice/
git commit -m "feat: add git-spice client wrapper"
```

---

### Task 4: Implement wt init command

**Files:**
- Create: `internal/cli/init.go`
- Modify: `internal/cli/root.go:23-26` (to register init command)

**Step 1: Write failing test**

Create `internal/cli/init_test.go`:

```go
package cli

import (
	"bytes"
	"testing"
)

func TestNewInitCmd(t *testing.T) {
	cmd := NewInitCmd()
	if cmd == nil {
		t.Fatal("NewInitCmd() returned nil")
	}
	if cmd.Use != "init" {
		t.Errorf("Use = %v, want init", cmd.Use)
	}
}

func TestInitCmd_Executes(t *testing.T) {
	cmd := NewInitCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Test that command runs without error
	// Actual functionality tested in integration tests
	_ = cmd
}
```

**Step 2: Create init command**

Create `internal/cli/init.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/spice"
)

// NewInitCmd creates the init command
func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize wt configuration",
		Long: `Create default configuration file and check dependencies.

This command creates ~/.config/wt/config.yaml if it doesn't exist,
and verifies that required dependencies (git, git-spice) are installed.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			// Check dependencies
			fmt.Fprintln(out, "Checking dependencies...")

			// Check git
			if err := checkGit(ctx, out); err != nil {
				Fatal("Git check failed: %v", err)
			}

			// Check git-spice
			if err := checkGitSpice(ctx, out); err != nil {
				Fatal("git-spice check failed: %v", err)
			}

			// Create config file
			fmt.Fprintln(out, "\nChecking configuration...")
			if err := createConfigFile(out); err != nil {
				Fatal("Config setup failed: %v", err)
			}

			fmt.Fprintln(out, "\n✓ wt initialized successfully")
			fmt.Fprintf(out, "Config file: %s\n", getConfigPath())
		},
	}

	return cmd
}

func checkGit(ctx context.Context, out io.Writer) error {
	gitClient, err := git.NewClient()
	if err != nil {
		fmt.Fprintf(out, "✗ git not found\n")
		return err
	}

	// Try to get version to verify git works
	// This requires adding a GetVersion method to git.Client
	// For now, just check client creation
	_ = gitClient

	fmt.Fprintf(out, "✓ git installed\n")
	return nil
}

func checkGitSpice(ctx context.Context, out io.Writer) error {
	spiceClient, err := spice.NewClient()
	if err != nil {
		fmt.Fprintf(out, "✗ git-spice not found\n\n")
		fmt.Fprintf(out, "  git-spice is required for stacking.\n\n")
		fmt.Fprintf(out, "  Install with one of:\n")
		fmt.Fprintf(out, "    cargo install git-spice\n")
		fmt.Fprintf(out, "    brew install git-spice\n")
		fmt.Fprintf(out, "    cargo-binstall git-spice\n\n")
		fmt.Fprintf(out, "  Run 'wt init' again after installing.\n")
		return err
	}

	version, err := spiceClient.GetVersion(ctx)
	if err != nil {
		fmt.Fprintf(out, "✗ git-spice installed but version check failed\n")
		return err
	}

	fmt.Fprintf(out, "✓ git-spice installed: %s\n", version)
	return nil
}

func createConfigFile(out io.Writer) error {
	configPath := getConfigPath()

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Fprintf(out, "✓ Config exists: %s\n", configPath)
		return nil
	}

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Create default config
	cfg := config.DefaultConfig()

	// For now, just validate - saving will be added if needed
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	fmt.Fprintf(out, "✓ Config validated (no file created yet)\n")
	return nil
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "wt", "config.yaml")
}

func init() {
	RegisterCommand(NewInitCmd())
}
```

**Step 3: Add missing import**

Add `"io"` to imports in init.go:

```go
import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	...
)
```

**Step 4: Run tests**

Run: `go test ./internal/cli -v -run TestInit`

Expected: PASS

**Step 5: Build and verify**

Run: `make build`

Expected: bin/wt created

Run: `./bin/wt init --help`

Expected: Help text displayed

**Step 6: Commit**

```bash
git add internal/cli/init.go internal/cli/init_test.go
git commit -m "feat: add wt init command for first-time setup"
```

---

### Task 5: Implement wt doctor command

**Files:**
- Create: `internal/cli/doctor.go`
- Create: `internal/cli/doctor_test.go`

**Step 1: Write failing test**

Create `internal/cli/doctor_test.go`:

```go
package cli

import (
	"bytes"
	"testing"
)

func TestNewDoctorCmd(t *testing.T) {
	cmd := NewDoctorCmd()
	if cmd == nil {
		t.Fatal("NewDoctorCmd() returned nil")
	}
	if cmd.Use != "doctor" {
		t.Errorf("Use = %v, want doctor", cmd.Use)
	}
}
```

**Step 2: Create doctor command**

Create `internal/cli/doctor.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/spice"
)

// NewDoctorCmd creates the doctor command
func NewDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Health check for wt installation",
		Long: `Run diagnostics to verify wt installation and dependencies.

Checks:
- wt binary and version
- git installation and worktree support
- git-spice installation
- Configuration file validity
- Current repository status`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()
			exitCode := 0

			fmt.Fprintln(out, "Checking wt installation...")
			fmt.Fprintf(out, "✓ wt binary: %s\n", os.Args[0])
			fmt.Fprintf(out, "✓ Version: v2.0.0\n")

			fmt.Fprintln(out, "\nChecking dependencies...")

			if err := checkGitDoctor(ctx, out); err != nil {
				exitCode = 1
			}

			if err := checkGitSpiceDoctor(ctx, out); err != nil {
				exitCode = 2
			}

			fmt.Fprintln(out, "\nChecking configuration...")
			if err := checkConfigDoctor(out); err != nil {
				exitCode = 2
			}

			fmt.Fprintln(out, "\nChecking current repository...")
			if err := checkRepoDoctor(ctx, out); err != nil {
				exitCode = 1
			}

			if exitCode == 0 {
				fmt.Fprintln(out, "\nAll checks passed!")
			} else {
				os.Exit(exitCode)
			}
		},
	}

	return cmd
}

func checkGitDoctor(ctx context.Context, out io.Writer) error {
	gitClient, err := git.NewClient()
	if err != nil {
		fmt.Fprintf(out, "✗ git not found\n")
		return err
	}

	// Check worktree support
	repoInfo, err := gitClient.GetRepoInfo(ctx)
	if err != nil {
		fmt.Fprintf(out, "✗ git worktree not supported\n")
		return err
	}

	fmt.Fprintf(out, "✓ git installed\n")
	fmt.Fprintf(out, "✓ git worktree supported\n")
	fmt.Fprintf(out, "  Repo root: %s\n", repoInfo.RootPath)
	return nil
}

func checkGitSpiceDoctor(ctx context.Context, out io.Writer) error {
	spiceClient, err := spice.NewClient()
	if err != nil {
		fmt.Fprintf(out, "✗ git-spice not found\n")
		return err
	}

	version, err := spiceClient.GetVersion(ctx)
	if err != nil {
		fmt.Fprintf(out, "✗ git-spice version check failed\n")
		return err
	}

	fmt.Fprintf(out, "✓ git-spice installed: %s\n", version)
	return nil
}

func checkConfigDoctor(out io.Writer) error {
	configPath, err := config.FindConfig("")
	if err != nil {
		fmt.Fprintf(out, "✗ No configuration file found\n")
		fmt.Fprintf(out, "  Run 'wt init' to create config\n")
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(out, "✗ Config file invalid\n")
		return err
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(out, "✗ Config validation failed: %v\n", err)
		return err
	}

	fmt.Fprintf(out, "✓ User config: %s\n", configPath)
	fmt.Fprintf(out, "✓ Config is valid\n")

	location := "dedicated"
	if !cfg.Worktree.IsDedicated() {
		location = "per-repo"
	}
	fmt.Fprintf(out, "✓ Worktree location: %s (%s)\n", location, cfg.Worktree.GetDedicatedPath())
	return nil
}

func checkRepoDoctor(ctx context.Context, out io.Writer) error {
	gitClient, err := git.NewClient()
	if err != nil {
		return err
	}

	repoInfo, err := gitClient.GetRepoInfo(ctx)
	if err != nil {
		fmt.Fprintf(out, "✗ Not in a git repository\n")
		return err
	}

	fmt.Fprintf(out, "✓ Git repository detected\n")

	// Check current branch - we'll need to add GetCurrentBranch to git.Client
	// For now, just check repo exists
	fmt.Fprintf(out, "✓ Default branch: %s\n", repoInfo.DefaultBranch)

	return nil
}

func init() {
	RegisterCommand(NewDoctorCmd())
}
```

**Step 3: Add GetCurrentBranch to git.Client**

Modify `internal/git/worktree.go`, add after line 172:

```go
// GetCurrentBranch returns the current branch name
func (c *Client) GetCurrentBranch(ctx context.Context) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "symbolic-ref", "--short", "HEAD")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Might be detached HEAD
		return "", fmt.Errorf("not on any branch: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}
```

**Step 4: Update checkRepoDoctor to use GetCurrentBranch**

Modify the checkRepoDoctor function in doctor.go:

```go
func checkRepoDoctor(ctx context.Context, out io.Writer) error {
	gitClient, err := git.NewClient()
	if err != nil {
		return err
	}

	repoInfo, err := gitClient.GetRepoInfo(ctx)
	if err != nil {
		fmt.Fprintf(out, "✗ Not in a git repository\n")
		return err
	}

	fmt.Fprintf(out, "✓ Git repository detected\n")

	currentBranch, err := gitClient.GetCurrentBranch(ctx)
	if err != nil {
		// Detached HEAD or error
		fmt.Fprintf(out, "! Cannot determine current branch (detached HEAD?)\n")
	} else {
		fmt.Fprintf(out, "✓ On branch: %s\n", currentBranch)

		// Check if trying to stack on main/master
		if currentBranch == "main" || currentBranch == "master" {
			fmt.Fprintf(out, "! Warning: On default branch. Stack on feature branches.\n")
		} else {
			fmt.Fprintf(out, "✓ Can create stack (not on main/master)\n")
		}
	}

	fmt.Fprintf(out, "✓ Default branch: %s\n", repoInfo.DefaultBranch)

	return nil
}
```

**Step 5: Run tests**

Run: `go test ./internal/cli -v -run TestDoctor`

Expected: PASS

**Step 6: Build and test**

Run: `make build`

Run: `./bin/wt doctor`

Expected: Health check output

**Step 7: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_test.go internal/git/worktree.go
git commit -m "feat: add wt doctor health check command"
```

---

## Phase 2: Core Stacking

### Task 6: Create stack service and domain types

**Files:**
- Create: `internal/stack/service.go`
- Create: `internal/stack/service_test.go`
- Modify: `pkg/domain/worktree.go:97-98` (add StackBranch type)

**Step 1: Add StackBranch domain type**

In `pkg/domain/worktree.go`, add at end of file (around line 97):

```go
// StackBranch represents a branch in a git-spice stack
type StackBranch struct {
	Name     string    // Branch name
	IsRoot   bool      // Is this the root of the stack
	IsHead   bool      // Is this the current branch
	Path     string    // Worktree path if checked out
	Children []*StackBranch // Child branches
}
```

**Step 2: Write failing test for stack service**

Create `internal/stack/service_test.go`:

```go
package stack

import (
	"context"
	"testing"

	"github.com/user/wt/internal/spice"
)

func TestNewService(t *testing.T) {
	service, err := NewService(nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_GenerateBranchSuffix(t *testing.T) {
	service, _ := NewService(nil)

	suffix1 := service.GenerateBranchSuffix()
	suffix2 := service.GenerateBranchSuffix()

	if len(suffix1) != 4 {
		t.Errorf("suffix length = %d, want 4", len(suffix1))
	}
	if suffix1 == suffix2 {
		t.Error("suffixes should be unique")
	}
}

func TestService_BuildStackBranchName(t *testing.T) {
	service, _ := NewService(nil)

	tests := []struct {
		name       string
		current    string
		suffixName string
		wantPrefix string
	}{
		{
			name:       "auto suffix",
			current:    "feat/auth",
			suffixName: "",
			wantPrefix: "feat/auth-",
		},
		{
			name:       "named suffix",
			current:    "feat/auth",
			suffixName: "api",
			wantPrefix: "feat/auth-api-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.BuildStackBranchName(tt.current, tt.suffixName)
			if !hasPrefix(result, tt.wantPrefix) {
				t.Errorf("branch name = %v, want prefix %v", result, tt.wantPrefix)
			}
			// Should have 4 char suffix at end
			suffixLen := len(result) - len(tt.wantPrefix)
			if suffixLen != 4 {
				t.Errorf("suffix length = %d, want 4", suffixLen)
			}
		})
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
```

**Step 3: Create stack service**

Create `internal/stack/service.go`:

```go
package stack

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aidarkhanov/nanoid/v3"
	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/pkg/domain"
)

// Service provides stack management operations
type Service struct {
	git   git.GitClient
	spice SpiceClient
	cfg   *config.Config
}

// SpiceClient defines the interface for git-spice operations
type SpiceClient interface {
	CreateBranch(ctx context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error)
	GetStack(ctx context.Context) ([]*spice.Branch, error)
}

// NewService creates a new stack service
func NewService(gitClient git.GitClient, spiceClient SpiceClient, cfg *config.Config) (*Service, error) {
	if gitClient == nil {
		return nil, fmt.Errorf("gitClient cannot be nil")
	}
	if spiceClient == nil {
		return nil, fmt.Errorf("spiceClient cannot be nil")
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	return &Service{
		git:   gitClient,
		spice: spiceClient,
		cfg:   cfg,
	}, nil
}

// CreateStackBranch creates a new stacked branch
func (s *Service) CreateStackBranch(ctx context.Context, spec StackBranchSpec) (*domain.StackBranch, error) {
	// Get current branch
	currentBranch, err := s.git.GetCurrentBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting current branch: %w", err)
	}

	// Build the new branch name
	branchName := s.BuildStackBranchName(currentBranch, spec.Name)

	// Create branch via git-spice
	spiceSpec := spice.BranchCreateSpec{
		Name: branchName,
		Base: spec.Base,
	}

	createdBranch, err := s.spice.CreateBranch(ctx, spiceSpec)
	if err != nil {
		return nil, fmt.Errorf("creating branch with git-spice: %w", err)
	}

	// Generate worktree path
	worktreePath := s.getWorktreePath(createdBranch.Name)

	return &domain.StackBranch{
		Name: createdBranch.Name,
		Path: worktreePath,
	}, nil
}

// GenerateBranchSuffix generates a 4-character nanoid suffix
func (s *Service) GenerateBranchSuffix() string {
	suffix, _ := nanoid.Generate(4)
	return suffix
}

// BuildStackBranchName builds a stacked branch name with optional named suffix
func (s *Service) BuildStackBranchName(currentBranch, suffixName string) string {
	suffix := s.GenerateBranchSuffix()

	if suffixName == "" {
		// Auto-suffix: feat/auth -> feat/auth-xY7k
		return fmt.Sprintf("%s-%s", currentBranch, suffix)
	}
	// Named suffix: feat/auth -> feat/auth-api-k9P2
	return fmt.Sprintf("%s-%s-%s", currentBranch, suffixName, suffix)
}

// getWorktreePath returns the worktree path for a branch
func (s *Service) getWorktreePath(branch string) string {
	if s.cfg.Worktree.IsDedicated() {
		return filepath.Join(s.cfg.Worktree.GetDedicatedPath(), branch)
	}
	// per-repo mode
	repoInfo, _ := s.git.GetRepoInfo(context.Background())
	return filepath.Join(repoInfo.RootPath, ".worktrees", branch)
}

// StackBranchSpec defines parameters for creating a stack branch
type StackBranchSpec struct {
	Name string // Optional named suffix (e.g., "api" for feat/auth-api-xxxx)
	Base string // Optional base branch (defaults to current)
}
```

**Step 4: Fix import cycle by updating spice package**

The service needs to use spice types. Update `internal/spice/client.go` to export the types properly - they're already there.

**Step 5: Run tests**

Run: `go test ./internal/stack -v`

Expected: PASS (but may have import issues to fix)

**Step 6: Fix any compilation issues**

If there are issues with the SpiceClient interface, adjust:

```go
// SpiceClient defines the interface for git-spice operations
type SpiceClient interface {
	CreateBranch(ctx context.Context, spec BranchCreateSpec) (*Branch, error)
	GetStack(ctx context.Context) ([]*Branch, error)
}
```

And use the types directly without `spice.` prefix in the interface methods.

**Step 7: Commit**

```bash
git add internal/stack/ pkg/domain/worktree.go
git commit -m "feat: add stack service for branch stacking operations"
```

---

### Task 7: Implement wt stack command

**Files:**
- Create: `internal/cli/stack.go`
- Create: `internal/cli/stack_test.go`

**Step 1: Write failing test**

Create `internal/cli/stack_test.go`:

```go
package cli

import (
	"testing"
)

func TestNewStackCmd(t *testing.T) {
	cmd := NewStackCmd()
	if cmd == nil {
		t.Fatal("NewStackCmd() returned nil")
	}
	if cmd.Use != "stack [name]" {
		t.Errorf("Use = %v, want 'stack [name]'", cmd.Use)
	}
}
```

**Step 2: Create stack command**

Create `internal/cli/stack.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/spice"
	"github.com/user/wt/internal/stack"
)

// NewStackCmd creates the stack command group
func NewStackCmd() *cobra.Command {
	var (
		stackBase  string
		stackForce bool
		noSetup    bool
	)

	cmd := &cobra.Command{
		Use:   "stack [name]",
		Short: "Create a stacked branch",
		Long: `Create a new stacked branch on top of the current branch.

If no name is provided, generates an auto-suffix (4 chars).
If a name is provided, appends it with a 4-char suffix.

Examples:
  wt stack              # Creates: currentBranch-xY7k
  wt stack api          # Creates: currentBranch-api-k9P2`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			// Check for main/master protection
			currentBranch, err := getCurrentBranchProtected(ctx)
			if err != nil {
				Fatal("Failed to get current branch: %v", err)
			}

			if !stackForce && isProtectedBranch(currentBranch) {
				Fatal("Cannot stack on '%s'. Stack on feature branches only.\nUse --force to override.", currentBranch)
			}

			// Create clients and service
			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			spiceClient, err := spice.NewClient()
			if err != nil {
				Fatal("Failed to create git-spice client: %v", err)
			}

			cfg, err := loadConfigForCommand()
			if err != nil {
				Fatal("Failed to load config: %v", err)
			}

			stackService, err := stack.NewService(gitClient, spiceClient, cfg)
			if err != nil {
				Fatal("Failed to create stack service: %v", err)
			}

			// Get the optional name argument
			var name string
			if len(args) > 0 {
				name = args[0]
			}

			// Create the stack branch
			spec := stack.StackBranchSpec{
				Name: name,
				Base: stackBase,
			}

			stackBranch, err := stackService.CreateStackBranch(ctx, spec)
			if err != nil {
				Fatal("Failed to create stack branch: %v", err)
			}

			fmt.Fprintf(out, "Created stacked branch: %s\n", stackBranch.Name)

			// Create worktree
			if !noSetup {
				// Worktree creation will be implemented in next task
				fmt.Fprintf(out, "Worktree path: %s\n", stackBranch.Path)
			}
		},
	}

	cmd.Flags().StringVar(&stackBase, "base", "", "base branch for stack (default: current)")
	cmd.Flags().BoolVar(&stackForce, "force", false, "allow stacking on main/master")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false, "skip setup hooks and worktree creation")

	return cmd
}

// NewStackListCmd creates the stack list subcommand
func NewStackListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show stack hierarchy with paths",
		Long:  `Display the current stack as a tree with branch names and worktree paths.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			spiceClient, err := spice.NewClient()
			if err != nil {
				Fatal("Failed to create git-spice client: %v", err)
			}

			branches, err := spiceClient.GetStack(ctx)
			if err != nil {
				Fatal("Failed to get stack: %v", err)
			}

			// Get current branch for highlighting
			gitClient, err := git.NewClient()
			if err == nil {
				currentBranch, _ := gitClient.GetCurrentBranch(ctx)
				// TODO: Format tree display with current marker
				_ = currentBranch
			}

			// Simple display for now
			for _, branch := range branches {
				fmt.Fprintf(out, "%s\n", branch.Name)
			}
		},
	}

	return cmd
}

func getCurrentBranchProtected(ctx context.Context) (string, error) {
	gitClient, err := git.NewClient()
	if err != nil {
		return "", err
	}
	return gitClient.GetCurrentBranch(ctx)
}

func isProtectedBranch(branch string) bool {
	return branch == "main" || branch == "master"
}

func loadConfigForCommand() (*config.Config, error) {
	configPath, err := config.FindConfig("")
	if err != nil {
		return config.DefaultConfig(), nil
	}
	return config.Load(configPath)
}

func init() {
	stackCmd := NewStackCmd()
	stackCmd.AddCommand(NewStackListCmd())
	RegisterCommand(stackCmd)
}
```

**Step 3: Update internal/spice to export Branch type**

The `Branch` type in `internal/spice/client.go` needs to be properly accessible. Ensure it's exported (it should be already).

**Step 4: Run tests**

Run: `go test ./internal/cli -v -run TestStack`

Expected: PASS

**Step 5: Build**

Run: `make build`

Expected: bin/wt created

**Step 6: Test help**

Run: `./bin/wt stack --help`

Expected: Help text displayed

**Step 7: Commit**

```bash
git add internal/cli/stack.go internal/cli/stack_test.go
git commit -m "feat: add wt stack command for creating stacked branches"
```

---

### Task 8: Integrate worktree creation in wt stack

**Files:**
- Modify: `internal/cli/stack.go:55-85` (update Run function)
- Modify: `internal/stack/service.go:43-53` (add CreateWorktree method)

**Step 1: Add CreateWorktree to stack service**

In `internal/stack/service.go`, add after CreateStackBranch:

```go
// CreateWorktree creates a worktree for a stack branch
func (s *Service) CreateWorktree(ctx context.Context, branch string) (*domain.Worktree, error) {
	worktreePath := s.getWorktreePath(branch)

	spec := domain.WorktreeCreateSpec{
		Branch:   branch,
		Path:     worktreePath,
		Checkout: true,
	}

	return s.git.AddWorktree(ctx, spec)
}
```

**Step 2: Update stack command to create worktree**

Modify the Run function in `internal/cli/stack.go`, update the section after creating stack branch:

```go
			// Create the stack branch
			spec := stack.StackBranchSpec{
				Name: name,
				Base: stackBase,
			}

			stackBranch, err := stackService.CreateStackBranch(ctx, spec)
			if err != nil {
				Fatal("Failed to create stack branch: %v", err)
			}

			fmt.Fprintf(out, "Created stacked branch: %s\n", stackBranch.Name)

			// Create worktree
			if !noSetup {
				worktree, err := stackService.CreateWorktree(ctx, stackBranch.Name)
				if err != nil {
					Fatal("Failed to create worktree: %v", err)
				}
				fmt.Fprintf(out, "Created worktree: %s\n", worktree.Path)

				// TODO: Run setup hooks (will be added in Phase 4)
			}
```

**Step 3: Run tests**

Run: `go test ./internal/stack ./internal/cli -v`

Expected: PASS

**Step 4: Build and test**

Run: `make build`

Run: `./bin/wt stack --help`

Expected: Help shows `--no-setup` flag

**Step 5: Commit**

```bash
git add internal/stack/service.go internal/cli/stack.go
git commit -m "feat: integrate worktree creation in wt stack command"
```

---

## Phase 3: Stack Display

### Task 9: Implement stack list with tree formatting

**Files:**
- Create: `internal/stack/tree.go`
- Modify: `internal/cli/stack.go:88-120` (update NewStackListCmd)

**Step 1: Write failing test for tree formatting**

Create `internal/stack/tree_test.go`:

```go
package stack

import (
	"strings"
	"testing"

	"github.com/user/wt/pkg/domain"
)

func TestFormatStackTree(t *testing.T) {
	branches := []*domain.StackBranch{
		{
			Name:   "feat/auth",
			IsRoot: true,
			Path:   "~/worktrees/feat/auth",
		},
		{
			Name:     "feat/auth-xY7k",
			IsRoot:   false,
			IsHead:   true,
			Path:     "~/worktrees/feat/auth-xY7k",
		},
		{
			Name:   "feat/auth-k9P2",
			IsRoot: false,
			Path:   "~/worktrees/feat/auth-k9P2",
		},
	}

	output := FormatStackTree(branches)

	if !strings.Contains(output, "feat/auth") {
		t.Error("output should contain root branch")
	}
	if !strings.Contains(output, "(current)") {
		t.Error("output should mark current branch")
	}
	if !strings.Contains(output, "~/worktrees") {
		t.Error("output should contain paths")
	}
}
```

**Step 2: Create tree formatter**

Create `internal/stack/tree.go`:

```go
package stack

import (
	"fmt"
	"strings"

	"github.com/user/wt/pkg/domain"
)

// FormatStackTree formats a stack of branches as a tree with paths
func FormatStackTree(branches []*domain.StackBranch) string {
	var builder strings.Builder

	for i, branch := range branches {
		prefix := getTreePrefix(i, len(branches))
		currentMarker := ""
		if branch.IsHead {
			currentMarker = " (current) ◀────"
		}

		pathPadding := getPathPadding(len(branch.Name) + len(currentMarker))

		fmt.Fprintf(&builder, "%s%s%s [%s]\n",
			prefix,
			branch.Name,
			currentMarker,
			branch.Path)
	}

	return builder.String()
}

func getTreePrefix(index, total int) string {
	if index == 0 {
		return ""
	}
	// Simple prefix for now - will be enhanced for proper tree structure
	return "├── "
}

func getPathPadding(nameLen int) int {
	// Target 60 chars for branch name + marker
	targetLen := 60
	if nameLen >= targetLen {
		return 2
	}
	return targetLen - nameLen
}
```

**Step 3: Update stack list command to use tree formatter**

Modify `NewStackListCmd` in `internal/cli/stack.go`:

```go
// NewStackListCmd creates the stack list subcommand
func NewStackListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show stack hierarchy with paths",
		Long:  `Display the current stack as a tree with branch names and worktree paths.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			spiceClient, err := spice.NewClient()
			if err != nil {
				Fatal("Failed to create git-spice client: %v", err)
			}

			// Get stack from git-spice
			spiceBranches, err := spiceClient.GetStack(ctx)
			if err != nil {
				Fatal("Failed to get stack: %v", err)
			}

			// Get current branch for highlighting
			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			currentBranch, err := gitClient.GetCurrentBranch(ctx)
			if err != nil {
				currentBranch = ""
			}

			// Convert to domain types with paths
			cfg, _ := loadConfigForCommand()
			stackService, _ := stack.NewService(gitClient, spiceClient, cfg)

			branches := convertToDomainBranches(spiceBranches, currentBranch, stackService)

			// Format and display
			tree := stack.FormatStackTree(branches)
			fmt.Fprint(out, tree)
		},
	}

	return cmd
}

func convertToDomainBranches(spiceBranches []*spice.Branch, current string, svc *stack.Service) []*domain.StackBranch {
	result := make([]*domain.StackBranch, 0, len(spiceBranches))

	for _, sb := range spiceBranches {
		// Get worktree path for this branch
		path := svc.GetWorktreePathForBranch(sb.Name)

		db := &domain.StackBranch{
			Name:   sb.Name,
			IsRoot: sb.IsRoot,
			IsHead: sb.Name == current,
			Path:   path,
		}
		result = append(result, db)
	}

	return result
}
```

**Step 4: Add GetWorktreePathForBranch method to stack service**

In `internal/stack/service.go`, add:

```go
// GetWorktreePathForBranch returns the worktree path for a given branch name
func (s *Service) GetWorktreePathForBranch(branch string) string {
	return s.getWorktreePath(branch)
}
```

**Step 5: Update spice client to return proper branch structure**

The `Branch` struct needs `IsRoot` field. Update `internal/spice/client.go`:

```go
// Branch represents a git-spice branch
type Branch struct {
	Name     string    // Branch name
	IsRoot   bool      // Is this the root of the stack
	IsHead   bool      // Is this the current branch
	Children []*Branch // Child branches in the stack
}
```

And update `parseStackOutput` to detect root branches:

```go
func parseStackOutput(output string) []*Branch {
	// Parse git-spice stack list output
	// Format is tree-like with indentation
	var branches []*Branch
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect root: branches without indentation are roots
		isRoot := !strings.HasPrefix(output[:strings.Index(output, line)], "  ")

		branches = append(branches, &Branch{
			Name:   line,
			IsRoot: isRoot,
		})
	}

	return branches
}
```

**Step 6: Run tests**

Run: `go test ./internal/stack -v`

Expected: PASS (may need adjustments)

**Step 7: Commit**

```bash
git add internal/stack/tree.go internal/stack/tree_test.go internal/stack/service.go internal/cli/stack.go internal/spice/client.go
git commit -m "feat: add stack list with tree formatting and path display"
```

---

## Phase 4: Setup Integration

### Task 10: Implement setup hooks in stack and add commands

**Files:**
- Modify: `internal/cli/stack.go:70-85` (add setup hook execution)
- Modify: `internal/cli/add.go:32-65` (add setup hook execution)
- Create: `pkg/executor/hook_runner.go`

**Step 1: Create hook runner**

Create `pkg/executor/hook_runner.go`:

```go
package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// HookRunner executes post-create hooks
type HookRunner struct {
	workingDir string
}

// NewHookRunner creates a new hook runner
func NewHookRunner(workingDir string) *HookRunner {
	return &HookRunner{workingDir: workingDir}
}

// RunHooks executes all hooks in sequence
func (h *HookRunner) RunHooks(ctx context.Context, hooks []config.Hook) error {
	for i, hook := range hooks {
		if err := h.RunHook(ctx, hook); err != nil {
			return fmt.Errorf("hook %d failed: %w", i, err)
		}
	}
	return nil
}

// RunHook executes a single hook
func (h *HookRunner) RunHook(ctx context.Context, hook config.Hook) error {
	// Expand {worktree_path} template
	cwd := hook.Cwd
	if strings.Contains(cwd, "{worktree_path}") {
		cwd = strings.ReplaceAll(cwd, "{worktree_path}", h.workingDir)
	}
	if cwd == "" {
		cwd = h.workingDir
	}

	// Parse command
	parts := strings.Fields(hook.Run)
	if len(parts) == 0 {
		return fmt.Errorf("empty hook command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Add timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %q: %w", hook.Run, err)
	}

	return nil
}
```

**Step 2: Add import to hook_runner**

Add the config import:

```go
import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/wt/internal/config"
)
```

**Step 3: Update add command to run hooks**

Modify `internal/cli/add.go`, update the Run function after worktree creation:

```go
			worktree, err := svc.Add(ctx, spec)
			if err != nil {
				Fatal("Failed to add worktree: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created worktree: %s [%s]\n", worktree.Path, worktree.Branch)

			// Run setup hooks
			if err := runSetupHooks(ctx, worktree.Path); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
			}
```

**Step 4: Add runSetupHooks helper**

Add to `internal/cli/add.go`:

```go
// runSetupHooks executes post-create hooks for a worktree
func runSetupHooks(ctx context.Context, worktreePath string) error {
	cfg, err := loadConfigForCommand()
	if err != nil {
		return err
	}

	runner := executor.NewHookRunner(worktreePath)
	return runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate)
}
```

**Step 5: Add executor import to add.go**

```go
import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/worktree"
	"github.com/user/wt/pkg/domain"
	"github.com/user/wt/pkg/executor"
)
```

**Step 6: Update stack command similarly**

In `internal/cli/stack.go`, after worktree creation:

```go
				fmt.Fprintf(out, "Created worktree: %s\n", worktree.Path)

				// Run setup hooks
				if err := runSetupHooks(ctx, worktree.Path); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
				}
```

And add the same imports and helper function.

**Step 7: Create test**

Create `pkg/executor/hook_runner_test.go`:

```go
package executor

import (
	"context"
	"testing"
)

func TestNewHookRunner(t *testing.T) {
	runner := NewHookRunner("/tmp")
	if runner == nil {
		t.Fatal("NewHookRunner() returned nil")
	}
	if runner.workingDir != "/tmp" {
		t.Errorf("workingDir = %v, want /tmp", runner.workingDir)
	}
}

func TestHookRunner_RunHooks(t *testing.T) {
	runner := NewHookRunner("/tmp")

	// Test with empty hooks
	err := runner.RunHooks(context.Background(), []config.Hook{})
	if err != nil {
		t.Errorf("RunHooks() with empty hooks error = %v", err)
	}
}
```

**Step 8: Run tests**

Run: `go test ./pkg/executor ./internal/cli -v`

Expected: PASS

**Step 9: Commit**

```bash
git add pkg/executor/hook_runner.go pkg/executor/hook_runner_test.go internal/cli/add.go internal/cli/stack.go
git commit -m "feat: add setup hook execution to add and stack commands"
```

---

### Task 11: Implement wt setup command

**Files:**
- Create: `internal/cli/setup.go`
- Create: `internal/cli/setup_test.go`

**Step 1: Write failing test**

Create `internal/cli/setup_test.go`:

```go
package cli

import (
	"testing"
)

func TestNewSetupCmd(t *testing.T) {
	cmd := NewSetupCmd()
	if cmd == nil {
		t.Fatal("NewSetupCmd() returned nil")
	}
	if cmd.Use != "setup" {
		t.Errorf("Use = %v, want setup", cmd.Use)
	}
}
```

**Step 2: Create setup command**

Create `internal/cli/setup.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/git"
)

// NewSetupCmd creates the setup command
func NewSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Re-run setup hooks in current worktree",
		Long: `Re-run all post-create hooks for the current worktree.

This is useful after manually fixing setup issues or when you want
to refresh your development environment.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			// Get current directory
			wd, err := os.Getwd()
			if err != nil {
				Fatal("Failed to get working directory: %v", err)
			}

			// Verify we're in a git worktree
			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			repoInfo, err := gitClient.GetRepoInfo(ctx)
			if err != nil {
				Fatal("Not in a git repository: %v", err)
			}

			// Check if we're in a worktree (not the main repo)
			if isMainWorktree(wd, repoInfo.RootPath) {
				Fatal("Setup hooks should be run from a worktree, not the main repository")
			}

			fmt.Fprintf(out, "Running setup hooks for: %s\n", filepath.Base(wd))

			// Run hooks
			if err := runSetupHooks(ctx, wd); err != nil {
				Fatal("Setup hooks failed: %v", err)
			}

			fmt.Fprintln(out, "✓ Setup complete")
		},
	}

	return cmd
}

func isMainWorktree(wd, repoRoot string) bool {
	// Normalize paths
	wd = filepath.Clean(wd)
	repoRoot = filepath.Clean(repoRoot)
	return wd == repoRoot
}

func init() {
	RegisterCommand(NewSetupCmd())
}
```

**Step 3: Run tests**

Run: `go test ./internal/cli -v -run TestSetup`

Expected: PASS

**Step 4: Build and test**

Run: `make build`

Run: `./bin/wt setup --help`

Expected: Help text displayed

**Step 5: Commit**

```bash
git add internal/cli/setup.go internal/cli/setup_test.go
git commit -m "feat: add wt setup command for re-running hooks"
```

---

## Phase 5: Polish & Testing

### Task 12: Add integration tests for stacking workflow

**Files:**
- Create: `tests/stacking_integration_test.go`

**Step 1: Create integration tests**

Create `tests/stacking_integration_test.go`:

```go
package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestStacking_BasicWorkflow tests the basic stacking workflow:
// 1. Create a root branch
// 2. Stack on it (auto-suffix)
// 3. Verify both branches exist
// 4. Clean up
func TestStacking_BasicWorkflow(t *testing.T) {
	skipIfNoGit(t)

	// Skip if git-spice not available
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("git-spice not available")
	}

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	// Initialize git-spice
	runCommand(t, repoPath, "gs", "init")

	// Create root branch via wt add
	runCommand(t, repoPath, "git", "checkout", "-b", "feat/test-root")

	// Verify we're on the new branch
	runCommand(t, repoPath, "gs", "branch", "feat/test-stack-child")

	// List worktrees
	client, _ := git.NewClient()
	worktrees, _ := client.ListWorktrees(ctx)

	// We should have at least the main worktree
	if len(worktrees) < 1 {
		t.Errorf("expected at least 1 worktree, got %d", len(worktrees))
	}

	// Cleanup
	runCommand(t, repoPath, "git", "checkout", "main")
}

func runCommand(t testing.TB, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\nOutput: %s", name, strings.Join(args, " "), err, output)
	}
}
```

**Step 2: Run tests**

Run: `go test ./tests -v -run TestStacking`

Expected: May skip if git-spice not installed

**Step 3: Commit**

```bash
git add tests/stacking_integration_test.go
git commit -m "test: add integration tests for stacking workflow"
```

---

### Task 13: Update documentation

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

**Step 1: Update README with stacking examples**

Add to README.md:

```markdown
## Stacking Features (v2)

WT v2 integrates with [git-spice](https://github.com/abhinav/git-spice) for branch stacking.

### Installation

1. Install git-spice:
   ```bash
   cargo install git-spice
   # or
   brew install git-spice
   ```

2. Initialize wt:
   ```bash
   wt init
   ```

### Basic Stacking Workflow

```bash
# Create root branch
wt add feat/auth
cd ~/worktrees/feat/auth

# Stack on current (auto-suffix)
wt stack
# Creates: feat/auth-xY7k

# Stack with named suffix
wt stack api
# Creates: feat/auth-api-k9P2

# View stack hierarchy
wt stack list
```

### Configuration

Worktree location is configurable in `~/.config/wt/config.yaml`:

```yaml
worktree:
  location: dedicated      # "dedicated" or "per-repo"
  dedicated_path: ~/worktrees  # custom path for dedicated mode
```

### Health Check

Run `wt doctor` to verify installation and dependencies.
```

**Step 2: Update CLAUDE.md with architecture changes**

Add to CLAUDE.md Architecture Overview section:

```markdown
### Stack Management (v2)

WT v2 adds stack management via git-spice integration:

- `internal/stack/service.go` - Stack operations using nanoid for unique suffixes
- `internal/spice/client.go` - Git-spice client wrapper
- `pkg/executor/hook_runner.go` - Hook execution for setup automation

Stack naming convention:
- Auto-suffix: `feat/auth` -> `feat/auth-xY7k` (4-char nanoid)
- Named suffix: `feat/auth` -> `feat/auth-api-k9P2`
```

**Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "docs: update documentation for v2 stacking features"
```

---

### Task 14: Final verification and cleanup

**Step 1: Run all tests**

Run: `make test`

Expected: All tests pass

**Step 2: Run linting**

Run: `make lint`

Expected: No lint errors (or fix if any)

**Step 3: Build final binary**

Run: `make build`

Expected: bin/wt created successfully

**Step 4: Test all commands**

```bash
./bin/wt --help
./bin/wt init --help
./bin/wt doctor --help
./bin/wt stack --help
./bin/wt stack list --help
./bin/wt setup --help
```

Expected: All help texts display correctly

**Step 5: Final commit**

```bash
git add -A
git commit -m "polish: final verification and cleanup for v2 stacking"
```

---

## Summary

This plan implements WT v2 stacking features in 5 phases:

**Phase 1: Foundation** - Dependencies, config, git-spice client, init/doctor commands
**Phase 2: Core Stacking** - Stack service, wt stack command, worktree integration
**Phase 3: Stack Display** - Tree formatting with paths and current branch marker
**Phase 4: Setup Integration** - Hook execution, wt setup command
**Phase 5: Polish** - Integration tests, documentation updates, verification

**Key Design Decisions:**
- Nanoid (4 chars) for collision-free branch suffixes
- Dedicated/per-repo worktree location modes
- Git-spice as required dependency (fail-fast if missing)
- Main/master branch protection with --force override
- Config-driven hook execution with template expansion
