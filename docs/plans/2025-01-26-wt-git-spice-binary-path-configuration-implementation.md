# Git-Spice Binary Path Configuration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add configurable git-spice binary path with init-time detection, explicit runtime validation, and clear error messages.

**Architecture:** Config-only approach (no env vars, no runtime auto-detect). `wt init` detects git-spice and populates `spice.binary_path`. Stack commands fail fast if not configured. `wt doctor` reports git-spice status.

**Tech Stack:** Go 1.22, Cobra CLI, YAML config (gopkg.in/yaml.v3), testing with standard `testing` package.

---

## Task 1: Add SpiceConfig to Config Structure

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go` (create if needed)

**Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
package config

import (
	"testing"
)

func TestSpiceConfig_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Spice.BinaryPath != "" {
		t.Errorf("expected empty BinaryPath, got %q", cfg.Spice.BinaryPath)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -v -run TestSpiceConfig`
Expected: FAIL with "undefined type Config has no field or method Spice"

**Step 3: Add SpiceConfig struct and field**

In `internal/config/config.go`, add after `WorktreeConfig` (around line 50):

```go
// SpiceConfig contains git-spice specific settings
type SpiceConfig struct {
	BinaryPath string `yaml:"binary_path"` // Path to git-spice binary
}
```

Add `Spice` field to `Config` struct (around line 12):

```go
type Config struct {
	Global    GlobalConfig     `yaml:"global"`
	Hooks     HooksConfig      `yaml:"hooks"`
	Tmux      TmuxConfig       `yaml:"tmux"`
	Worktree  WorktreeConfig   `yaml:"worktree"`
	Spice     SpiceConfig      `yaml:"spice"`      // NEW
	Overrides []OverrideConfig `yaml:"project_overrides,omitempty"`
}
```

Update `DefaultConfig()` to initialize Spice (around line 72):

```go
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
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
		Spice: SpiceConfig{
			BinaryPath: "", // Empty means not configured
		},
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -v -run TestSpiceConfig`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add SpiceConfig for git-spice binary path

Add spice.binary_path configuration field.
Empty by default, populated by wt init.
"
```

---

## Task 2: Add verifyGitSpice Function

**Files:**
- Modify: `internal/spice/client.go`
- Test: `internal/spice/client_test.go`

**Step 1: Write the failing test**

Add to `internal/spice/client_test.go`:

```go
package spice

import (
	"testing"
)

func TestVerifyGitSpice_ValidBinary(t *testing.T) {
	// Skip if git-spice not available
	path, err := findGitSpice()
	if err != nil {
		t.Skip("git-spice not available")
	}

	if err := verifyGitSpice(path); err != nil {
		t.Errorf("verifyGitSpice failed: %v", err)
	}
}

func TestVerifyGitSpice_InvalidBinary(t *testing.T) {
	err := verifyGitSpice("/bin/ls")
	if err == nil {
		t.Error("expected error for /bin/ls, got nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/spice -v -run TestVerifyGitSpice`
Expected: FAIL with "undefined function verifyGitSpice"

**Step 3: Implement verifyGitSpice**

Add to `internal/spice/client.go` after the `findGitSpice` function:

```go
// verifyGitSpice checks that the path is actually git-spice
func verifyGitSpice(path string) error {
	cmd := exec.Command(path, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run --version: %w", err)
	}
	if !strings.Contains(string(output), "git-spice") {
		return fmt.Errorf("version output doesn't contain 'git-spice'")
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/spice -v -run TestVerifyGitSpice`
Expected: PASS (with possible skip if git-spice not installed)

**Step 5: Commit**

```bash
git add internal/spice/client.go internal/spice/client_test.go
git commit -m "feat(spice): add verifyGitSpice function

Check that binary is actually git-spice by running --version
and checking output contains 'git-spice' string.
"
```

---

## Task 3: Add detectGitSpice Function

**Files:**
- Modify: `internal/spice/client.go`
- Test: `internal/spice/client_test.go`

**Step 1: Write the failing test**

Add to `internal/spice/client_test.go`:

```go
func TestDetectGitSpice_PrefersGitSpiceCommand(t *testing.T) {
	// This test verifies the precedence: git-spice > gs
	path, err := detectGitSpice()
	if err != nil {
		t.Skip("git-spice not available")
	}

	// Should not be just "gs" if "git-spice" exists
	if filepath.Base(path) == "gs" {
		// Check if git-spice also exists
		if _, err := exec.LookPath("git-spice"); err == nil {
			t.Error("prefer 'git-spice' over 'gs', but got 'gs'")
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/spice -v -run TestDetectGitSpice`
Expected: FAIL with "undefined function detectGitSpice"

**Step 3: Implement detectGitSpice**

Add to `internal/spice/client.go` (this replaces the existing `findGitSpice`):

```go
// detectGitSpice locates git-spice binary
// Tries "git-spice" first (most specific), then "gs" with verification
func detectGitSpice() (string, error) {
	// Try "git-spice" first
	if path, err := exec.LookPath("git-spice"); err == nil {
		if err := verifyGitSpice(path); err == nil {
			return path, nil
		}
	}

	// Try "gs" with verification
	if path, err := exec.LookPath("gs"); err == nil {
		if err := verifyGitSpice(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("git-spice not found in PATH (tried git-spice and gs)")
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/spice -v -run TestDetectGitSpice`
Expected: PASS (with possible skip)

**Step 5: Commit**

```bash
git add internal/spice/client.go internal/spice/client_test.go
git commit -m "feat(spice): add detectGitSpice function

Tries 'git-spice' command first, then 'gs' with verification.
Returns error if neither is found or if verification fails.
"
```

---

## Task 4: Modify spice.NewClient to Accept Config

**Files:**
- Modify: `internal/spice/client.go`
- Modify: `internal/stack/service.go`
- Test: `internal/spice/client_test.go`
- Test: `internal/stack/service_test.go`

**Step 1: Write the failing test**

Add to `internal/spice/client_test.go`:

```go
func TestNewClient_RequiresConfig(t *testing.T) {
	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: "",
		},
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Error("expected error when BinaryPath is empty")
	}
}

func TestNewClient_ValidatesPath(t *testing.T) {
	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: "/nonexistent/path",
		},
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/spice -v -run TestNewClient`
Expected: FAIL with "not enough arguments" or function signature mismatch

**Step 3: Update NewClient signature and implementation**

Replace `NewClient` function in `internal/spice/client.go`:

```go
// NewClient creates a new git-spice client with explicit config
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// No auto-detection at runtime
	if cfg.Spice.BinaryPath == "" {
		return nil, fmt.Errorf("git-spice not configured. Run 'wt init' to set up git-spice integration")
	}

	// Verify the configured path exists and is executable
	if _, err := exec.LookPath(cfg.Spice.BinaryPath); err != nil {
		return nil, fmt.Errorf("git-spice binary not found at %s: %w", cfg.Spice.BinaryPath, err)
	}

	// Verify it's actually git-spice
	if err := verifyGitSpice(cfg.Spice.BinaryPath); err != nil {
		return nil, fmt.Errorf("%s is not git-spice: %w", cfg.Spice.BinaryPath, err)
	}

	return &Client{gsPath: cfg.Spice.BinaryPath}, nil
}
```

**Step 4: Update stack service to pass config**

Modify `internal/stack/service.go` NewService function to accept and store config:

```go
type Service struct {
	git    git.GitClient
	spice  *spice.Client
	cfg    *config.Config  // ADD THIS FIELD
}

func NewService(gitClient git.GitClient, spiceClient *spice.Client, cfg *config.Config) (*Service, error) {
	if gitClient == nil {
		return nil, fmt.Errorf("gitClient cannot be nil")
	}
	if spiceClient == nil {
		return nil, fmt.Errorf("spiceClient cannot be nil")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &Service{
		git:   gitClient,
		spice: spiceClient,
		cfg:   cfg,  // ADD THIS LINE
	}, nil
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/spice ./internal/stack -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/spice/client.go internal/spice/client_test.go internal/stack/service.go internal/stack/service_test.go
git commit -m "refactor(spice): NewClient requires config, validates path

- NewClient now accepts *config.Config parameter
- Returns error if BinaryPath is empty
- Validates configured path exists and is git-spice
- Updated stack service to accept and store config
"
```

---

## Task 5: Update CLI Commands to Pass Config

**Files:**
- Modify: `internal/cli/stack.go`
- Modify: `internal/cli/init.go`

**Step 1: Modify stack.go to load config and pass to spice client**

In `internal/cli/stack.go`, update stack command initialization:

Find where `spice.NewClient()` is called and update:

```go
cfg, err := loadConfigForCommand()
if err != nil {
    Fatal("Failed to load config: %v", err)
}

spiceClient, err := spice.NewClient(cfg)
if err != nil {
    Fatal("Failed to create spice client: %v", err)
}

stackSvc, err := stack.NewService(gitClient, spiceClient, cfg)
if err != nil {
    Fatal("Failed to create stack service: %v", err)
}
```

**Step 2: Update init.go to use new config structure**

In `internal/cli/init.go`, after creating default config, add spice detection:

Find where config is created and add:

```go
// Detect git-spice and add to config
gitSpicePath, err := detectGitSpice()
if err != nil {
    fmt.Fprintf(cmd.ErrOrStderr(), "Warning: git-spice not found: %v\n", err)
    fmt.Fprintf(cmd.ErrOrStderr(), "Stacking features will not work.\n")
    fmt.Fprintf(cmd.ErrOrStderr(), "Install git-spice: cargo install git-spice\n")
    fmt.Fprintf(cmd.ErrOrStderr(), "Then re-run: wt init\n")
} else {
    cfg.Spice.BinaryPath = gitSpicePath
    fmt.Fprintf(cmd.OutOrStdout(), "Detected git-spice at: %s\n", gitSpicePath)
}
```

Add the import for detectGitSpice:
```go
import (
    // ... existing imports ...
    "github.com/joebalancio/wt/internal/spice"
)
```

**Step 3: Build to verify changes**

Run: `make build`
Expected: Success

**Step 4: Commit**

```bash
git add internal/cli/stack.go internal/cli/init.go
git commit -m "feat(cli): update commands to use new config structure

- stack.go: Load config and pass to spice client
- init.go: Detect git-spice and populate spice.binary_path
"
```

---

## Task 6: Add Git-Spice Check to wt doctor

**Files:**
- Modify: `internal/cli/doctor.go`

**Step 1: Read current doctor implementation**

Run: `cat internal/cli/doctor.go`
Note: Understand current structure, find where to add git-spice section

**Step 2: Add git-spice check to doctor output**

In `internal/cli/doctor.go`, add after existing checks:

```go
// Check git-spice
fmt.Fprintln(cmd.OutOrStdout(), "\nGit-Spice:")
if cfg.Spice.BinaryPath == "" {
    fmt.Fprintln(cmd.OutOrStdout(), "  Status: Not configured")
    fmt.Fprintln(cmd.OutOrStdout(), "  Fix: Run 'wt init' to detect and configure git-spice")
} else {
    client, err := spice.NewClient(cfg)
    if err != nil {
        fmt.Fprintf(cmd.OutOrStdout(), "  Status: Error\n")
        fmt.Fprintf(cmd.OutOrStdout(), "  Configured: %s\n", cfg.Spice.BinaryPath)
        fmt.Fprintf(cmd.OutOrStdout(), "  Error: %v\n", err)
    } else {
        version, _ := client.GetVersion(context.Background())
        fmt.Fprintf(cmd.OutOrStdout(), "  Status: OK\n")
        fmt.Fprintf(cmd.OutOrStdout(), "  Path: %s\n", cfg.Spice.BinaryPath)
        fmt.Fprintf(cmd.OutOrStdout(), "  Version: %s\n", version)
    }
}
```

Add context import if not present:
```go
import (
    "context"
    // ... existing imports ...
)
```

**Step 3: Build and test manually**

Run: `make build`
Run: `./bin/wt doctor`
Expected: Shows Git-Spice section with status

**Step 4: Commit**

```bash
git add internal/cli/doctor.go
git commit -m "feat(doctor): add git-spice health check

Reports git-spice status: OK/Error/Not configured
Shows path and version when available
Provides fix suggestions for error states
"
```

---

## Task 7: Add Early Validation to wt stack

**Files:**
- Modify: `internal/cli/stack.go`

**Step 1: Add early check in stack command**

In `internal/cli/stack.go`, add validation at start of Run function:

```go
Run: func(cmd *cobra.Command, args []string) {
    // Early check before any operations
    cfg, err := loadConfigForCommand()
    if err != nil {
        Fatal("Failed to load config: %v", err)
    }

    if cfg.Spice.BinaryPath == "" {
        Fatal(`git-spice is not configured.

Stacking features require git-spice.

To fix:
  1. Install git-spice: cargo install git-spice
  2. Run: wt init

This will detect and configure git-spice automatically.`)
    }

    // Verify it works
    client, err := spice.NewClient(cfg)
    if err != nil {
        Fatal("git-spice configuration error: %v\n\nRun 'wt doctor' for details.", err)
    }

    // ... continue with existing stack operations ...
},
```

**Step 2: Build and test error case**

Run: `make build`
Test with empty config (should error):
```bash
mv ~/.config/wt/config.yaml ~/.config/wt/config.yaml.bak
./bin/wt stack 2>&1 | head -10
mv ~/.config/wt/config.yaml.bak ~/.config/wt/config.yaml
```
Expected: Error message about git-spice not configured

**Step 3: Commit**

```bash
git add internal/cli/stack.go
git commit -m "feat(stack): add early git-spice validation

Fails fast with helpful error if git-spice not configured.
Clear error messages guide users to wt init.
"
```

---

## Task 8: Update Example Config

**Files:**
- Modify: `configs/example.yaml`

**Step 1: Add spice section to example config**

In `configs/example.yaml`, add:

```yaml
spice:
  # Path to git-spice binary. Populated by 'wt init'.
  # Leave empty to disable stacking features.
  binary_path: ""  # Example: "/home/linuxbrew/.linuxbrew/bin/git-spice"
```

**Step 2: Verify YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('configs/example.yaml'))"` or use `yamllint`
Expected: No parsing errors

**Step 3: Commit**

```bash
git add configs/example.yaml
git commit -m "docs(example): add spice.binary_path to example config

Document the spice configuration section with explanation
of how wt init populates this field.
"
```

---

## Task 9: Update README Documentation

**Files:**
- Modify: `README.md`

**Step 1: Update Stacking Features section**

In README.md, update the installation section to mention `wt init`:

```markdown
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

   This detects git-spice and configures the `spice.binary_path` setting.
```

**Step 2: Add troubleshooting section**

Add to README.md:

```markdown
### Troubleshooting

**git-spice not found:**

If `wt stack` reports that git-spice is not configured:

1. Install git-spice: `cargo install git-spice`
2. Run `wt init` to detect and configure it
3. Verify with `wt doctor`

**Wrong git-spice path:**

If you've moved git-spice, re-run `wt init` to update the configuration.

**Ghostscript conflicts:**

If your system has Ghostscript installed, `wt init` will detect the correct
git-spice binary and avoid conflicts.
```

**Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): document git-spice setup and troubleshooting

Add wt init step to installation.
Add troubleshooting section for common issues.
"
```

---

## Task 10: Add Integration Tests

**Files:**
- Modify: `tests/stacking_integration_test.go`

**Step 1: Update test helper to detect git-spice**

Replace `gitSpiceCommand()` function with:

```go
// gitSpiceCommand returns the detected git-spice command
// Uses the same detection logic as wt init
func gitSpiceCommand() (string, error) {
	return detectGitSpice()
}
```

Add import:
```go
import (
    // ... existing imports ...
    "github.com/joebalancio/wt/internal/spice"
)
```

**Step 2: Run integration tests**

Run: `go test ./tests -v -run TestStacking`
Expected: Tests skip if git-spice not available

**Step 3: Commit**

```bash
git add tests/stacking_integration_test.go
git commit -m "test(integration): use detectGitSpice in tests

Tests now use the same detection logic as wt init.
More consistent behavior across codebase.
"
```

---

## Task 11: Full Test Suite and Validation

**Step 1: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 2: Run linter**

Run: `make lint`
Expected: No linter errors

**Step 3: Build binary**

Run: `make build`
Expected: Binary builds successfully

**Step 4: Manual smoke test**

```bash
# Test wt init creates config with spice path
./bin/wt init

# Test wt doctor shows git-spice status
./bin/wt doctor

# Test wt stack errors appropriately
./bin/wt stack
```

**Step 5: Commit**

```bash
git add -A
git commit -m "test: validate git-spice configuration implementation

- All unit tests pass
- Integration tests updated
- Manual smoke testing successful
"
```

---

## Task 12: Remove Old Auto-Detection Code

**Files:**
- Modify: `internal/spice/client.go`

**Step 1: Remove old findGitSpice function**

Remove the old `findGitSpice()` function that was used before (it's now replaced by `detectGitSpice()`)

**Step 2: Verify no references remain**

Run: `grep -r "findGitSpice" internal/`
Expected: No results (or only in test files)

**Step 3: Run tests**

Run: `go test ./...`
Expected: All tests pass

**Step 4: Commit**

```bash
git add internal/spice/client.go
git commit -m "refactor(spice): remove old findGitSpice function

Replaced by detectGitSpice() with clearer naming.
"
```

---

## Summary

This implementation plan:
1. ✅ Adds `SpiceConfig` to configuration structure
2. ✅ Implements `detectGitSpice()` and `verifyGitSpice()` functions
3. ✅ Modifies `spice.NewClient()` to require config and validate
4. ✅ Updates `wt init` to detect and populate `spice.binary_path`
5. ✅ Adds git-spice health check to `wt doctor`
6. ✅ Adds early validation to `wt stack`
7. ✅ Updates documentation and example config
8. ✅ Updates integration tests
9. ✅ Removes old auto-detection code

**Total tasks: 12**
**Estimated time: 2-3 hours**
**Breaking change:** Yes - users must run `wt init` after upgrade
