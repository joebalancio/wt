# Project-Specific Hooks Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable project-specific hook overrides that match worktree paths using glob patterns, appending to global hooks for all hook types.

**Architecture:** Modify `HookRunner` to accept config and worktree path, then iterate over `ProjectOverrides` to find matching patterns. Matching override hooks are appended to global hooks. Glob pattern validation is added to `ValidateSchema()`.

**Tech Stack:** Go 1.21+, `filepath.Match` for glob patterns, existing hook infrastructure

---

## Task 1: Add Glob Pattern Validation to Config

**Files:**
- Modify: `internal/config/config.go:132-141` (ValidateSchema method)
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test for glob validation**

Add to `internal/config/config_test.go`:

```go
func TestConfig_ValidateSchema_GlobPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"valid simple", "feature-*", false},
		{"valid with path", "**/feature-*", false},
		{"valid with single char", "project-?", false},
		{"valid with char class", "project-[abc]*", false},
		{"invalid unclosed bracket", "project-[", true},
		{"invalid bad escape", "project-\\", true},
		{"empty pattern", "", false}, // empty is valid (matches nothing)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Overrides = []OverrideConfig{
				{Match: tt.pattern},
			}

			err := cfg.ValidateSchema()
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSchema() expected error for pattern %q, got nil", tt.pattern)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateSchema() unexpected error for pattern %q: %v", tt.pattern, err)
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestConfig_ValidateSchema_GlobPattern -v`

Expected: FAIL - "no error returned for invalid pattern"

**Step 3: Implement glob validation**

Modify `ValidateSchema` in `internal/config/config.go`:

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

	// Validate glob patterns in project_overrides
	for i, override := range c.Overrides {
		if override.Match == "" {
			continue // empty pattern is valid (matches nothing)
		}
		// filepath.Match returns error for malformed patterns
		// We test with empty string to validate pattern syntax
		if _, err := filepath.Match(override.Match, ""); err != nil {
			return fmt.Errorf("invalid glob pattern in project_overrides[%d].match %q: %w",
				i, override.Match, err)
		}
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestConfig_ValidateSchema_GlobPattern -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add glob pattern validation for project_overrides"
```

---

## Task 2: Add Hook Merging Helper to Config

**Files:**
- Modify: `internal/config/config.go` (add new method)
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test for hook merging**

Add to `internal/config/config_test.go`:

```go
func TestConfig_GetHooksForPath(t *testing.T) {
	cfg := &Config{
		Hooks: HooksConfig{
			OnWorktreeCreate: []Hook{
				{Run: "global-create"},
			},
			OnWorktreeRemove: []Hook{
				{Run: "global-remove"},
			},
		},
		Overrides: []OverrideConfig{
			{
				Match: "**/rust/**",
				Hooks: HooksConfig{
					OnWorktreeCreate: []Hook{
						{Run: "rust-create"},
					},
				},
			},
			{
				Match: "**/shared/**",
				Hooks: HooksConfig{
					OnWorktreeCreate: []Hook{
						{Run: "shared-create"},
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		hookType string
		want     []string
	}{
		{
			name:     "no match returns global hooks",
			path:     "/home/user/worktrees/python-project",
			hookType: "create",
			want:     []string{"global-create"},
		},
		{
			name:     "single match appends override",
			path:     "/home/user/worktrees/rust/feature-auth",
			hookType: "create",
			want:     []string{"global-create", "rust-create"},
		},
		{
			name:     "multiple matches append all",
			path:     "/home/user/worktrees/rust/shared/lib",
			hookType: "create",
			want:     []string{"global-create", "rust-create", "shared-create"},
		},
		{
			name:     "remove hooks with no override",
			path:     "/home/user/worktrees/rust/feature-auth",
			hookType: "remove",
			want:     []string{"global-remove"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []Hook
			switch tt.hookType {
			case "create":
				got = cfg.GetHooksForPath(tt.path, cfg.Hooks.OnWorktreeCreate, func(o OverrideConfig) []Hook {
					return o.Hooks.OnWorktreeCreate
				})
			case "remove":
				got = cfg.GetHooksForPath(tt.path, cfg.Hooks.OnWorktreeRemove, func(o OverrideConfig) []Hook {
					return o.Hooks.OnWorktreeRemove
				})
			}

			if len(got) != len(tt.want) {
				t.Errorf("GetHooksForPath() returned %d hooks, want %d", len(got), len(tt.want))
				return
			}
			for i, hook := range got {
				if hook.Run != tt.want[i] {
					t.Errorf("GetHooksForPath()[%d].Run = %q, want %q", i, hook.Run, tt.want[i])
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestConfig_GetHooksForPath -v`

Expected: FAIL - "Config.GetHooksForPath undefined"

**Step 3: Implement the hook merging method**

Add to `internal/config/config.go`:

```go
// HookExtractor is a function that extracts hooks of a specific type from an OverrideConfig
type HookExtractor func(OverrideConfig) []Hook

// GetHooksForPath returns hooks for a given path, combining global hooks with
// any matching override hooks. All matching overrides are applied in order.
func (c *Config) GetHooksForPath(path string, globalHooks []Hook, extract HookExtractor) []Hook {
	// Start with global hooks
	result := make([]Hook, len(globalHooks))
	copy(result, globalHooks)

	// Append hooks from all matching overrides
	for _, override := range c.Overrides {
		matched, err := filepath.Match(override.Match, path)
		if err != nil {
			// Log warning but continue - pattern was validated at load time
			// so this should only happen with runtime path issues
			continue
		}
		if matched {
			overrideHooks := extract(override)
			result = append(result, overrideHooks...)
		}
	}

	return result
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestConfig_GetHooksForPath -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add GetHooksForPath for hook merging"
```

---

## Task 3: Update HookRunner to Use Merged Hooks

**Files:**
- Modify: `pkg/executor/hook_runner.go`
- Test: `pkg/executor/hook_runner_test.go`

**Step 1: Write the failing test for RunHooksWithConfig**

Add to `pkg/executor/hook_runner_test.go`:

```go
func TestHookRunner_RunHooksWithConfig(t *testing.T) {
	// Create a temp file to track hook execution
	tmpDir := t.TempDir()
	markerFile := filepath.Join(tmpDir, "marker.txt")

	cfg := &config.Config{
		Hooks: config.HooksConfig{
			OnWorktreeCreate: []config.Hook{
				{Run: "echo global >> " + markerFile},
			},
		},
		Overrides: []config.OverrideConfig{
			{
				Match: "**/rust/**",
				Hooks: config.HooksConfig{
					OnWorktreeCreate: []config.Hook{
						{Run: "echo override >> " + markerFile},
					},
				},
			},
		},
	}

	runner := NewHookRunner("/home/user/worktrees/rust/feature")

	// Get merged hooks
	hooks := cfg.GetHooksForPath("/home/user/worktrees/rust/feature",
		cfg.Hooks.OnWorktreeCreate,
		func(o config.OverrideConfig) []config.Hook {
			return o.Hooks.OnWorktreeCreate
		})

	err := runner.RunHooks(context.Background(), hooks)
	if err != nil {
		t.Fatalf("RunHooks() error = %v", err)
	}

	// Verify both hooks ran
	data, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("failed to read marker file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "global") {
		t.Error("global hook did not run")
	}
	if !strings.Contains(content, "override") {
		t.Error("override hook did not run")
	}
}
```

Add imports:
```go
import (
	"os"
	"path/filepath"
	"strings"
)
```

**Step 2: Run test to verify it passes (no new code needed)**

Run: `go test ./pkg/executor -run TestHookRunner_RunHooksWithConfig -v`

Expected: PASS - the GetHooksForPath method handles this

**Step 3: Commit**

```bash
git add pkg/executor/hook_runner_test.go
git commit -m "test(executor): add test for merged hooks execution"
```

---

## Task 4: Integrate Hook Merging into runSetupHooks (add.go)

**Files:**
- Modify: `internal/cli/add.go:104-112` (runSetupHooks function)
- Test: `tests/project_overrides_integration_test.go`

**Step 1: Write the integration test**

Create `tests/project_overrides_integration_test.go`:

```go
package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
)

// TestIntegration_ProjectOverrides tests that project_overrides work correctly:
// 1. Global hooks run first
// 2. Matching override hooks are appended
// 3. Multiple matching overrides all apply
func TestIntegration_ProjectOverrides(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	// Create a marker file to track hook execution order
	markerFile := filepath.Join(repoPath, "hook-order.txt")

	// Create config with global hooks and overrides
	cfg := config.DefaultConfig()
	cfg.Hooks.OnWorktreeCreate = []config.Hook{
		{Run: "echo global >> " + markerFile},
	}
	cfg.Overrides = []config.OverrideConfig{
		{
			Match: "**/feature-**",
			Hooks: config.HooksConfig{
				OnWorktreeCreate: []config.Hook{
					{Run: "echo feature-override >> " + markerFile},
				},
			},
		},
	}

	// Create git client and service
	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a feature worktree (should match override)
	featureBranch := "feature-test-overrides"
	featurePath := filepath.Join(repoPath, "feature-test-overrides")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Run hooks manually using the merged hook logic
	runner := service.GetHookRunner(featurePath)
	hooks := cfg.GetHooksForPath(featurePath, cfg.Hooks.OnWorktreeCreate,
		func(o config.OverrideConfig) []config.Hook {
			return o.Hooks.OnWorktreeCreate
		})
	if err := runner.RunHooks(ctx, hooks); err != nil {
		t.Fatalf("failed to run hooks: %v", err)
	}

	// Verify hook execution order
	data, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("failed to read marker file: %v", err)
	}

	content := string(data)
	expected := "global\nfeature-override\n"
	if content != expected {
		t.Errorf("hook execution order = %q, want %q", content, expected)
	}

	// Cleanup
	service.Remove(ctx, featurePath, false)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestIntegration_ProjectOverrides -v`

Expected: FAIL - service doesn't expose GetHookRunner

**Step 3: Update runSetupHooks in add.go**

Modify `internal/cli/add.go`:

```go
// runSetupHooks executes post-create hooks for a worktree
func runSetupHooks(ctx context.Context, worktreePath string) error {
	cfg, err := loadConfigForCommand()
	if err != nil {
		return err
	}

	runner := executor.NewHookRunner(worktreePath)

	// Get merged hooks (global + matching overrides)
	hooks := cfg.GetHooksForPath(worktreePath, cfg.Hooks.OnWorktreeCreate,
		func(o config.OverrideConfig) []config.Hook {
			return o.Hooks.OnWorktreeCreate
		})

	return runner.RunHooks(ctx, hooks)
}
```

**Step 4: Run all add-related tests**

Run: `go test ./internal/cli -run TestAdd -v && go test ./tests -run TestIntegration -v`

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/cli/add.go tests/project_overrides_integration_test.go
git commit -m "feat(cli): integrate project overrides into runSetupHooks"
```

---

## Task 5: Integrate Hook Merging into Done Workflow

**Files:**
- Modify: `internal/worktree/service.go:154-164` (done hooks)
- Modify: `internal/worktree/service.go:177-188` (remove hooks)
- Test: `tests/done_hooks_integration_test.go`

**Step 1: Add test for done hooks with overrides**

Add to `tests/done_hooks_integration_test.go`:

```go
// TestIntegration_Done_WithProjectOverrides tests done hooks with project overrides
func TestIntegration_Done_WithProjectOverrides(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	markerFile := filepath.Join(repoPath, "done-override-marker.txt")

	// Create config with done hook override
	cfg := config.DefaultConfig()
	cfg.Hooks.OnWorktreeDone = []config.Hook{
		{Run: "echo global-done >> " + markerFile},
	}
	cfg.Overrides = []config.OverrideConfig{
		{
			Match: "**/feature-**",
			Hooks: config.HooksConfig{
				OnWorktreeDone: []config.Hook{
					{Run: "echo override-done >> " + markerFile},
				},
			},
		},
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	featureBranch := "feature-test-done-override"
	featurePath := filepath.Join(repoPath, "feature-test-done-override")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Make changes and commit
	featureFile := filepath.Join(featurePath, "feature.txt")
	os.WriteFile(featureFile, []byte("Feature\n"), 0o644)
	runGitCommand(t, featurePath, "add", "feature.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add feature")

	runGitCommand(t, repoPath, "checkout", "main")

	err = service.Done(ctx, featurePath, featureBranch, false)
	if err != nil {
		t.Fatalf("failed to complete worktree: %v", err)
	}

	// Verify both hooks ran
	data, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("marker file not found: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "global-done") {
		t.Error("global done hook did not run")
	}
	if !strings.Contains(content, "override-done") {
		t.Error("override done hook did not run")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./tests -run TestIntegration_Done_WithProjectOverrides -v`

Expected: FAIL - override hook not executed

**Step 3: Update Done method in service.go**

Modify `internal/worktree/service.go` around line 154:

Replace:
```go
	// Run done hooks with template variables
	if len(s.cfg.Hooks.OnWorktreeDone) > 0 {
		templateVars := map[string]string{
			"branch":         branch,
			"worktree_path":  worktreePath,
		}
		runner := executor.NewHookRunner(worktreePath, templateVars)
		if err := runner.RunHooks(ctx, s.cfg.Hooks.OnWorktreeDone); err != nil {
			// Log hook failures as warnings but don't block cleanup
			fmt.Fprintf(os.Stderr, "Warning: done hooks failed: %v\n", err)
		}
	}
```

With:
```go
	// Run done hooks with template variables
	templateVars := map[string]string{
		"branch":        branch,
		"worktree_path": worktreePath,
	}
	runner := executor.NewHookRunner(worktreePath, templateVars)
	hooks := s.cfg.GetHooksForPath(worktreePath, s.cfg.Hooks.OnWorktreeDone,
		func(o config.OverrideConfig) []config.Hook {
			return o.Hooks.OnWorktreeDone
		})
	if len(hooks) > 0 {
		if err := runner.RunHooks(ctx, hooks); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: done hooks failed: %v\n", err)
		}
	}
```

And similarly for remove hooks around line 177:

Replace:
```go
	// Run remove hooks with template variables
	if len(s.cfg.Hooks.OnWorktreeRemove) > 0 {
		templateVars := map[string]string{
			"branch":        branch,
			"worktree_path": worktreePath,
		}
		// Note: worktree is gone, so use empty working dir
		runner := executor.NewHookRunner("", templateVars)
		if err := runner.RunHooks(ctx, s.cfg.Hooks.OnWorktreeRemove); err != nil {
			// Log hook failures as warnings but don't fail
			fmt.Fprintf(os.Stderr, "Warning: remove hooks failed: %v\n", err)
		}
	}
```

With:
```go
	// Run remove hooks with template variables
	templateVars := map[string]string{
		"branch":        branch,
		"worktree_path": worktreePath,
	}
	// Note: worktree is gone, so use empty working dir
	runner := executor.NewHookRunner("", templateVars)
	hooks := s.cfg.GetHooksForPath(worktreePath, s.cfg.Hooks.OnWorktreeRemove,
		func(o config.OverrideConfig) []config.Hook {
			return o.Hooks.OnWorktreeRemove
		})
	if len(hooks) > 0 {
		if err := runner.RunHooks(ctx, hooks); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: remove hooks failed: %v\n", err)
		}
	}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./tests -run TestIntegration_Done -v`

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/worktree/service.go tests/done_hooks_integration_test.go
git commit -m "feat(worktree): integrate project overrides into done/remove hooks"
```

---

## Task 6: Add wt config validate Test for Glob Patterns

**Files:**
- Modify: `tests/config_validate_integration_test.go` (or create if needed)

**Step 1: Write integration test for wt config validate**

Create or add to `tests/config_validate_integration_test.go`:

```go
package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestIntegration_ConfigValidate_InvalidGlobPattern(t *testing.T) {
	// Create a temp config file with invalid glob pattern
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".wt.yaml")

	configContent := `
hooks:
  on_worktree_create:
    - run: "echo test"
project_overrides:
  - match: "**/[invalid"
    hooks:
      on_worktree_create:
        - run: "echo override"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// ValidateSchema should catch the invalid glob
	err = cfg.ValidateSchema()
	if err == nil {
		t.Error("ValidateSchema() expected error for invalid glob pattern, got nil")
	}
}

func TestIntegration_ConfigValidate_ValidGlobPattern(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".wt.yaml")

	configContent := `
hooks:
  on_worktree_create:
    - run: "echo test"
project_overrides:
  - match: "**/feature-*"
    hooks:
      on_worktree_create:
        - run: "echo override"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// ValidateSchema should pass for valid glob
	err = cfg.ValidateSchema()
	if err != nil {
		t.Errorf("ValidateSchema() unexpected error: %v", err)
	}
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./tests -run TestIntegration_ConfigValidate -v`

Expected: PASS

**Step 3: Commit**

```bash
git add tests/config_validate_integration_test.go
git commit -m "test: add integration tests for glob pattern validation"
```

---

## Task 7: Run Full Test Suite and Final Validation

**Step 1: Run all tests**

Run: `make test`

Expected: All PASS

**Step 2: Run linter**

Run: `make lint`

Expected: No errors

**Step 3: Run format check**

Run: `make fmt`

**Step 4: Build binary**

Run: `make build`

Expected: Binary created at `bin/wt`

**Step 5: Manual validation**

```bash
# Create a test config with overrides
cat > /tmp/test-wt.yaml << 'EOF'
hooks:
  on_worktree_create:
    - run: "echo global-hook"
project_overrides:
  - match: "**/rust/**"
    hooks:
      on_worktree_create:
        - run: "echo rust-override"
EOF

# Validate the config
./bin/wt config validate --config /tmp/test-wt.yaml
```

Expected: Config valid

```bash
# Test invalid glob
cat > /tmp/test-wt-bad.yaml << 'EOF'
project_overrides:
  - match: "**/[invalid"
    hooks:
      on_worktree_create:
        - run: "echo test"
EOF

./bin/wt config validate --config /tmp/test-wt-bad.yaml
```

Expected: Error about invalid glob pattern

**Step 6: Final commit**

```bash
git add -A
git status
git commit -m "feat: implement project-specific hooks with glob pattern matching

- Add glob pattern validation to config.ValidateSchema()
- Add GetHooksForPath() for merging global and override hooks
- Integrate hook merging into add, done, and remove workflows
- Add comprehensive integration tests

Closes: project-specific-hooks feature"
```

---

## Summary

| Task | Description | Files Changed |
|------|-------------|---------------|
| 1 | Glob validation in config | `config.go`, `config_test.go` |
| 2 | Hook merging helper | `config.go`, `config_test.go` |
| 3 | HookRunner test | `hook_runner_test.go` |
| 4 | Integrate into add.go | `add.go`, new integration test |
| 5 | Integrate into service.go | `service.go`, `done_hooks_integration_test.go` |
| 6 | Config validate tests | `config_validate_integration_test.go` |
| 7 | Final validation | All tests, lint, build |
