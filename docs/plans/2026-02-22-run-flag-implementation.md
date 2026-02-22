# `--run` Flag Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--run <command>` flag to `wt add` and `wt stack` commands that executes a user-specified command after hooks complete.

**Architecture:** Add helper functions in root.go for template expansion and command execution. Modify add.go and stack.go to add the flag and call helpers after hook completion. In tmux, use fire-and-forget SendKeys; outside tmux, use syscall.Exec for process replacement.

**Tech Stack:** Go 1.21+, syscall.Exec, existing tmux client patterns

---

## Task 1: Create Unit Tests for Template Expansion

**Files:**
- Create: `internal/cli/run_command_test.go`

**Step 1: Write the failing tests**

```go
package cli

import (
	"testing"
)

func TestExpandRunTemplate(t *testing.T) {
	tests := []struct {
		name         string
		command      string
		worktreePath string
		want         string
	}{
		{
			name:         "expands worktree_path",
			command:      "cd {worktree_path} && claude",
			worktreePath: "/home/user/worktrees/feat-auth",
			want:         "cd /home/user/worktrees/feat-auth && claude",
		},
		{
			name:         "empty_command",
			command:      "",
			worktreePath: "/path/to/worktree",
			want:         "",
		},
		{
			name:         "no_templates",
			command:      "claude",
			worktreePath: "/path",
			want:         "claude",
		},
		{
			name:         "unknown_template_passthrough",
			command:      "cd {branch} && claude",
			worktreePath: "/path",
			want:         "cd {branch} && claude",
		},
		{
			name:         "multiple_worktree_path",
			command:      "cd {worktree_path} && echo {worktree_path}",
			worktreePath: "/home/user/wt",
			want:         "cd /home/user/wt && echo /home/user/wt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandRunTemplate(tt.command, tt.worktreePath)
			if got != tt.want {
				t.Errorf("expandRunTemplate(%q, %q) = %q, want %q", tt.command, tt.worktreePath, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestExpandRunTemplate -v`
Expected: FAIL - `expandRunTemplate` undefined

**Step 3: Commit the test file**

```bash
git add internal/cli/run_command_test.go
git commit -m "test: add tests for expandRunTemplate function"
```

---

## Task 2: Implement Template Expansion

**Files:**
- Modify: `internal/cli/root.go` (add helper function)
- Test: `internal/cli/run_command_test.go`

**Step 1: Write the implementation**

Add to `internal/cli/root.go` after the `Warn` function:

```go
// expandRunTemplate expands template variables in a run command.
// Only {worktree_path} is supported. Unknown templates pass through unchanged.
func expandRunTemplate(command, worktreePath string) string {
	return strings.ReplaceAll(command, "{worktree_path}", worktreePath)
}
```

Add import for `"strings"` if not already present.

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/cli -run TestExpandRunTemplate -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat: add expandRunTemplate helper for --run command"
```

---

## Task 3: Create Tests for ShouldSkipRun

**Files:**
- Modify: `internal/cli/run_command_test.go`

**Step 1: Write the failing tests**

Add to `internal/cli/run_command_test.go`:

```go
func TestShouldSkipRun(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		windowExisted bool
		want          bool
	}{
		{
			name:          "empty_command_skips",
			command:       "",
			windowExisted: false,
			want:          true,
		},
		{
			name:          "window_existed_skips",
			command:       "claude",
			windowExisted: true,
			want:          true,
		},
		{
			name:          "new_window_with_command_runs",
			command:       "claude",
			windowExisted: false,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipRun(tt.command, tt.windowExisted)
			if got != tt.want {
				t.Errorf("shouldSkipRun(%q, %v) = %v, want %v", tt.command, tt.windowExisted, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestShouldSkipRun -v`
Expected: FAIL - `shouldSkipRun` undefined

**Step 3: Commit the test additions**

```bash
git add internal/cli/run_command_test.go
git commit -m "test: add tests for shouldSkipRun function"
```

---

## Task 4: Implement ShouldSkipRun

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/run_command_test.go`

**Step 1: Write the implementation**

Add to `internal/cli/root.go` after `expandRunTemplate`:

```go
// shouldSkipRun returns true if the --run command should be skipped.
// Skips if command is empty or window already existed (don't interrupt).
func shouldSkipRun(command string, windowExisted bool) bool {
	return command == "" || windowExisted
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/cli -run TestShouldSkipRun -v`
Expected: PASS

**Step 3: Run all tests to verify no regressions**

Run: `go test ./internal/cli -v`
Expected: PASS (all tests)

**Step 4: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat: add shouldSkipRun helper for --run flag logic"
```

---

## Task 5: Create Tests for ExecReplace

**Files:**
- Modify: `internal/cli/run_command_test.go`

**Step 1: Write the failing tests**

Add to `internal/cli/run_command_test.go`:

```go
import (
	"os"
	"os/exec"
	"testing"
)

func TestBuildShellCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{
			name:    "simple_command",
			command: "claude",
			want:    []string{"sh", "-c", "claude"},
		},
		{
			name:    "command_with_args",
			command: "claude --prompt 'fix bug'",
			want:    []string{"sh", "-c", "claude --prompt 'fix bug'"},
		},
		{
			name:    "command_with_pipe",
			command: "echo hello | cat",
			want:    []string{"sh", "-c", "echo hello | cat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildShellCommand(tt.command)
			if !equalSlices(got, tt.want) {
				t.Errorf("buildShellCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestBuildShellCommand -v`
Expected: FAIL - `buildShellCommand` undefined

**Step 3: Commit the test additions**

```bash
git add internal/cli/run_command_test.go
git commit -m "test: add tests for buildShellCommand function"
```

---

## Task 6: Implement BuildShellCommand

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/run_command_test.go`

**Step 1: Write the implementation**

Add to `internal/cli/root.go`:

```go
// buildShellCommand builds the arguments for sh -c execution
func buildShellCommand(command string) []string {
	return []string{"sh", "-c", command}
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/cli -run TestBuildShellCommand -v`
Expected: PASS

**Step 3: Run all tests**

Run: `go test ./internal/cli -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat: add buildShellCommand helper for --run execution"
```

---

## Task 7: Create RunCommandAfterHooks Helper

**Files:**
- Modify: `internal/cli/root.go`

**Step 1: Add the helper function and types**

Add to `internal/cli/root.go` after the existing helper functions:

```go
import (
	"os"
	"os/exec"
	"syscall"
)

// RunCommandOpts contains options for running the --run command
type RunCommandOpts struct {
	Command       string
	WorktreePath  string
	WindowName    string
	TmuxClient    *tmux.Client
	WindowExisted bool
	InTmux        bool
}

// runCommandAfterHooks executes the --run command in the appropriate context.
// In tmux: fire-and-forget via SendKeys.
// Outside tmux: exec replacement (process is replaced, never returns).
func runCommandAfterHooks(opts RunCommandOpts) error {
	if shouldSkipRun(opts.Command, opts.WindowExisted) {
		if opts.Command != "" && opts.WindowExisted {
			// Inform user why we skipped
			fmt.Printf("--run skipped: window '%s' already exists\n", opts.WindowName)
		}
		return nil
	}

	// Expand template
	cmd := expandRunTemplate(opts.Command, opts.WorktreePath)

	if opts.InTmux && opts.TmuxClient != nil {
		// Fire-and-forget in tmux window
		return runCommandInTmuxWindow(opts.TmuxClient, opts.WindowName, cmd, opts.WorktreePath)
	}

	// Exec replacement outside tmux (never returns on success)
	return execReplace(opts.WorktreePath, cmd)
}

// runCommandInTmuxWindow sends a command to a tmux window
func runCommandInTmuxWindow(tmuxClient *tmux.Client, windowName, command, worktreePath string) error {
	// Build full command with cd
	fullCmd := fmt.Sprintf("cd %s && %s", worktreePath, command)

	// Send to tmux (fire-and-forget)
	if err := tmuxClient.RunInWindow(windowName, fullCmd); err != nil {
		Warn("Failed to run command in tmux: %v", err)
		return err
	}
	return nil
}

// execReplace replaces the current process with the command.
// This is used when running outside tmux. On success, this never returns.
func execReplace(worktreePath, command string) error {
	// Find sh executable
	shPath, err := exec.LookPath("sh")
	if err != nil {
		return fmt.Errorf("sh not found: %w", err)
	}

	// Change to worktree directory
	if err := os.Chdir(worktreePath); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	// Build args for sh -c
	args := buildShellCommand(command)

	// Replace process (never returns on success)
	return syscall.Exec(shPath, args, os.Environ())
}
```

**Step 2: Run tests to verify no regressions**

Run: `go test ./internal/cli -v`
Expected: PASS

**Step 3: Run linter**

Run: `make lint`
Expected: No errors

**Step 4: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat: add runCommandAfterHooks, runCommandInTmuxWindow, execReplace helpers"
```

---

## Task 8: Add --run Flag to wt add

**Files:**
- Modify: `internal/cli/add.go`

**Step 1: Add the flag variable**

In `NewAddCmd()`, add `run` to the existing variable block (around line 17):

```go
var (
	base       string
	path       string
	force      bool
	track      string
	noCheckout bool
	run        string  // ADD THIS LINE
)
```

**Step 2: Add the flag definition**

After the existing flags (around line 46), add:

```go
cmd.Flags().StringVar(&run, "run", "", "command to run after hooks (e.g., 'claude')")
```

**Step 3: Pass run to runAddCommand**

Modify the Run function to pass `run`:

```go
Run: func(cmd *cobra.Command, args []string) {
	branch := ""
	if len(args) > 0 {
		branch = args[0]
	}
	runAddCommand(cmd, branch, base, path, force, track, noCheckout, run)
},
```

**Step 4: Update runAddCommand signature**

Update the function signature to accept `run`:

```go
func runAddCommand(cmd *cobra.Command, branch, base, path string, force bool, track string, noCheckout bool, run string) {
```

**Step 5: Pass run to setupWorktreeWithTmux**

Update the call at the end of `runAddCommand`:

```go
setupWorktreeWithTmux(ctx, cmd, wt.Branch, wt.Path, run)
```

**Step 6: Run tests to verify compilation**

Run: `go build ./...`
Expected: Success

**Step 7: Commit**

```bash
git add internal/cli/add.go
git commit -m "feat: add --run flag to wt add command"
```

---

## Task 9: Update setupWorktreeWithTmux for --run

**Files:**
- Modify: `internal/cli/add.go`

**Step 1: Update function signature**

Change `setupWorktreeWithTmux` to accept `runCmd`:

```go
func setupWorktreeWithTmux(ctx context.Context, cmd *cobra.Command, branch, worktreePath, runCmd string) {
```

**Step 2: Add window existence tracking**

Modify the tmux path to track if window was newly created:

```go
func setupWorktreeWithTmux(ctx context.Context, cmd *cobra.Command, branch, worktreePath, runCmd string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		// Not in tmux or --no-tmux: run hooks locally
		if err := runSetupHooks(ctx, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}

		// Run --run command (exec replacement)
		if runCmd != "" {
			if err := runCommandAfterHooks(RunCommandOpts{
				Command:       runCmd,
				WorktreePath:  worktreePath,
				InTmux:        false,
			}); err != nil {
				Fatal("Failed to run command: %v", err)
			}
		}
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		// Fall back to local hooks if tmux unavailable
		if err := runSetupHooks(ctx, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}

		// Run --run command (exec replacement)
		if runCmd != "" {
			if err := runCommandAfterHooks(RunCommandOpts{
				Command:       runCmd,
				WorktreePath:  worktreePath,
				InTmux:        false,
			}); err != nil {
				Fatal("Failed to run command: %v", err)
			}
		}
		return
	}

	windowName := tmux.GenerateWindowName(branch)

	// Check if window exists BEFORE creating
	windowExisted, _ := tmuxClient.WindowExists(windowName)

	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
		// Still try to run hooks locally
		if err := runSetupHooks(ctx, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}
		return
	}

	// Select the window so user sees it
	_ = tmuxClient.SelectWindow(windowName)

	// Run hooks INSIDE the new window
	if err := runSetupHooksInWindow(ctx, worktreePath, tmuxClient, windowName); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
	}

	// Run --run command in tmux window
	if runCmd != "" {
		_ = runCommandAfterHooks(RunCommandOpts{
			Command:       runCmd,
			WorktreePath:  worktreePath,
			WindowName:    windowName,
			TmuxClient:    tmuxClient,
			WindowExisted: windowExisted,
			InTmux:        true,
		})
	}
}
```

**Step 3: Run tests to verify compilation**

Run: `go build ./...`
Expected: Success

**Step 4: Run existing tests**

Run: `go test ./internal/cli -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/add.go
git commit -m "feat: integrate --run flag with setupWorktreeWithTmux"
```

---

## Task 10: Add --run Flag to wt stack

**Files:**
- Modify: `internal/cli/stack.go`

**Step 1: Add the flag variable**

In `NewStackCmd()`, add `run` to the existing variable block:

```go
var (
	stackBase  string
	stackForce bool
	noSetup    bool
	run        string  // ADD THIS LINE
)
```

**Step 2: Add the flag definition**

After the existing flags, add:

```go
cmd.Flags().StringVar(&run, "run", "", "command to run after hooks (e.g., 'claude')")
```

**Step 3: Pass run to runStackCommand**

Modify the Run function:

```go
Run: func(cmd *cobra.Command, args []string) {
	runStackCommand(cmd, args, stackBase, stackForce, noSetup, run)
},
```

**Step 4: Update runStackCommand signature**

```go
func runStackCommand(cmd *cobra.Command, args []string, stackBase string, stackForce bool, noSetup bool, run string) {
```

**Step 5: Pass run to createStackWorktree**

```go
createStackWorktree(ctx, cmd, stackService, stackBranch.Name, run)
```

**Step 6: Update createStackWorktree signature and implementation**

```go
func createStackWorktree(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName string, runCmd string) {
	out := cmd.OutOrStdout()

	worktree, err := stackService.CreateWorktree(ctx, branchName)
	if err != nil {
		Fatal("Failed to create worktree: %v", err)
	}
	if _, err := fmt.Fprintf(out, "Created worktree: %s\n", worktree.Path); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// NEW ORDER: Create tmux window BEFORE running hooks
	if shouldCreateTmuxWindow(NoTmux()) {
		tmuxClient, err := tmux.NewClient()
		if err != nil {
			// Fall back to local hooks
			if err := runSetupHooks(ctx, worktree.Path); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
			}

			// Run --run command (exec replacement)
			if runCmd != "" {
				if err := runCommandAfterHooks(RunCommandOpts{
					Command:       runCmd,
					WorktreePath:  worktree.Path,
					InTmux:        false,
				}); err != nil {
					Fatal("Failed to run command: %v", err)
				}
			}
			return
		}

		// Get stack level for window naming
		stackLevel := getStackLevel(ctx, stackService, branchName)
		windowName := tmux.GenerateStackWindowName(branchName, stackLevel)

		// Check if window exists BEFORE creating
		windowExisted, _ := tmuxClient.WindowExists(windowName)

		if err := tmuxClient.CreateOrSelectWindow(windowName, worktree.Path); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
			if err := runSetupHooks(ctx, worktree.Path); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
			}
			return
		}

		// Select the window
		_ = tmuxClient.SelectWindow(windowName)

		// Run hooks INSIDE the new window
		if err := runSetupHooksInWindow(ctx, worktree.Path, tmuxClient, windowName); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}

		// Run --run command in tmux window
		if runCmd != "" {
			_ = runCommandAfterHooks(RunCommandOpts{
				Command:       runCmd,
				WorktreePath:  worktree.Path,
				WindowName:    windowName,
				TmuxClient:    tmuxClient,
				WindowExisted: windowExisted,
				InTmux:        true,
			})
		}
	} else {
		// Not in tmux or --no-tmux: run hooks locally
		if err := runSetupHooks(ctx, worktree.Path); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}

		// Run --run command (exec replacement)
		if runCmd != "" {
			if err := runCommandAfterHooks(RunCommandOpts{
				Command:       runCmd,
				WorktreePath:  worktree.Path,
				InTmux:        false,
			}); err != nil {
				Fatal("Failed to run command: %v", err)
			}
		}
	}
}
```

**Step 7: Run tests to verify compilation**

Run: `go build ./...`
Expected: Success

**Step 8: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 9: Commit**

```bash
git add internal/cli/stack.go
git commit -m "feat: add --run flag to wt stack command"
```

---

## Task 11: Add Integration Tests

**Files:**
- Create: `tests/run_flag_integration_test.go`

**Step 1: Write integration tests**

```go
//go:build integration
// +build integration

package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/cli"
)

func TestAddRunFlag_TemplateExpansion(t *testing.T) {
	// Unit test via exported function if possible, or test via CLI output
	// This tests that the template expansion works end-to-end
	tests := []struct {
		name         string
		command      string
		worktreePath string
		want         string
	}{
		{
			name:         "expands_worktree_path",
			command:      "echo {worktree_path}",
			worktreePath: "/tmp/test-wt",
			want:         "echo /tmp/test-wt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cli.ExpandRunTemplate(tt.command, tt.worktreePath)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAddRunFlag_EmptyCommand(t *testing.T) {
	// Test that empty --run is a no-op
	// This should create worktree normally without error
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")

	// Setup test repo
	setupTestRepo(t, repoDir)

	// The command should succeed without running anything
	// (Full integration test would require tmux or exec mock)
}
```

**Step 2: Run integration tests**

Run: `go test ./tests -tags=integration -v -run TestAddRunFlag`
Expected: PASS (or skip if not in integration environment)

**Step 3: Commit**

```bash
git add tests/run_flag_integration_test.go
git commit -m "test: add integration tests for --run flag"
```

---

## Task 12: Update Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `internal/cli/add.go` (Long description)
- Modify: `internal/cli/stack.go` (Long description)

**Step 1: Add to CLAUDE.md**

Add after the existing `wt add` section:

```markdown
### wt add --run flag

Run a command after worktree creation and hooks complete.

```bash
# Start Claude Code in new worktree
wt add feat/auth --run "claude"

# Run with template
wt add feat/api --run "cd {worktree_path} && claude"

# Works with --no-tmux (exec's into command)
wt add feat/ui --no-tmux --run "claude"
```

**Behavior:**
- In tmux: Command sent to new window (fire-and-forget)
- Outside tmux: wt replaces itself with the command
- If window already exists: `--run` is skipped with message
```

**Step 2: Update add.go Long description**

```go
Long: `Add a new worktree for the specified branch.

If the branch does not exist, it will be created from the base branch (default: current HEAD).
If the branch already exists, a worktree will be added for that existing branch.

Use --run to execute a command after hooks complete:
  wt add feat/auth --run "claude"

Template variables:
  {worktree_path} - Path to the new worktree`,
```

**Step 3: Update stack.go Long description**

```go
Long: `Create a new stacked branch on top of the current branch.

If no name is provided, generates an auto-suffix (4 chars).
If a name is provided, appends it with a 4-char suffix.

Examples:
  wt stack              # Creates: currentBranch-xY7k
  wt stack api          # Creates: currentBranch-api-k9P2
  wt stack api --run "claude"  # Run command after setup

Template variables for --run:
  {worktree_path} - Path to the new worktree`,
```

**Step 4: Commit**

```bash
git add CLAUDE.md internal/cli/add.go internal/cli/stack.go
git commit -m "docs: update documentation for --run flag"
```

---

## Task 13: Run Full Test Suite and Lint

**Files:**
- All modified files

**Step 1: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Run linter**

Run: `make lint`
Expected: No errors

**Step 3: Run make check**

Run: `make check`
Expected: All checks pass

**Step 4: Build binary**

Run: `make build`
Expected: Success

**Step 5: Verify help output**

Run: `./bin/wt add --help`
Expected: Shows `--run` flag with description

Run: `./bin/wt stack --help`
Expected: Shows `--run` flag with description

---

## Task 14: Final Commit and Push

**Step 1: Review all changes**

Run: `git status && git diff --stat`
Expected: All files committed

**Step 2: Run bd sync**

Run: `bd sync --from-main`
Expected: Sync successful

**Step 3: Final commit (if any remaining changes)**

```bash
git add .
git commit -m "feat: complete --run flag implementation for wt add and wt stack"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create unit tests for template expansion | `internal/cli/run_command_test.go` |
| 2 | Implement expandRunTemplate | `internal/cli/root.go` |
| 3 | Create tests for shouldSkipRun | `internal/cli/run_command_test.go` |
| 4 | Implement shouldSkipRun | `internal/cli/root.go` |
| 5 | Create tests for buildShellCommand | `internal/cli/run_command_test.go` |
| 6 | Implement buildShellCommand | `internal/cli/root.go` |
| 7 | Create runCommandAfterHooks helper | `internal/cli/root.go` |
| 8 | Add --run flag to wt add | `internal/cli/add.go` |
| 9 | Update setupWorktreeWithTmux | `internal/cli/add.go` |
| 10 | Add --run flag to wt stack | `internal/cli/stack.go` |
| 11 | Add integration tests | `tests/run_flag_integration_test.go` |
| 12 | Update documentation | `CLAUDE.md`, `add.go`, `stack.go` |
| 13 | Run full test suite | All files |
| 14 | Final commit and push | All files |
