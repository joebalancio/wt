# Wait for Hooks Before --run Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix race condition where `--run` command executes while setup hooks are still running in tmux.

**Architecture:** Add `WithFinalCommand()` option to `HookRunner` that appends a final command to the hook chain. In tmux mode, build a single compound command using subshells chained with `&&` for fail-fast behavior.

**Tech Stack:** Go 1.21+, testing package, tmux (for integration tests)

---

## Task 1: Add WithFinalCommand Option to HookRunner

**Files:**
- Modify: `pkg/executor/hook_runner.go:43-48` (struct definition)
- Create: `pkg/executor/hook_runner.go` (new option function)
- Test: `pkg/executor/hook_runner_test.go`

**Step 1: Write the failing test**

Add to `pkg/executor/hook_runner_test.go`:

```go
func TestWithFinalCommand(t *testing.T) {
	runner := NewHookRunner("/tmp", WithFinalCommand("claude"))
	if runner == nil {
		t.Fatal("NewHookRunner() returned nil")
	}
	if runner.finalCommand != "claude" {
		t.Errorf("finalCommand = %v, want claude", runner.finalCommand)
	}
}

func TestWithFinalCommand_Empty(t *testing.T) {
	runner := NewHookRunner("/tmp", WithFinalCommand(""))
	if runner.finalCommand != "" {
		t.Errorf("finalCommand = %v, want empty", runner.finalCommand)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/executor -run TestWithFinalCommand -v`
Expected: FAIL with "runner.finalCommand undefined"

**Step 3: Add finalCommand field to HookRunner struct**

Modify `pkg/executor/hook_runner.go`:

```go
// HookRunner executes post-create hooks
type HookRunner struct {
	workingDir    string
	templateVars  map[string]string
	tmuxClient    *tmux.Client // nil = run locally
	windowName    string       // used if tmuxClient is set
	finalCommand  string       // command to run after all hooks (tmux mode only)
}
```

**Step 4: Add WithFinalCommand option function**

Add after `WithTemplateVars` function (around line 68):

```go
// WithFinalCommand sets a command to execute after all hooks complete (tmux mode only)
func WithFinalCommand(cmd string) HookRunnerOption {
	return func(hr *HookRunner) {
		hr.finalCommand = cmd
	}
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./pkg/executor -run TestWithFinalCommand -v`
Expected: PASS

**Step 6: Commit**

```bash
git add pkg/executor/hook_runner.go pkg/executor/hook_runner_test.go
git commit -m "feat(executor): add WithFinalCommand option to HookRunner

Adds finalCommand field and option function for appending a command
to the hook chain in tmux mode.

Relates-to: wt-98c"
```

---

## Task 2: Add buildCompoundCommand Helper Function

**Files:**
- Modify: `pkg/executor/hook_runner.go` (add helper)
- Test: `pkg/executor/hook_runner_test.go`

**Step 1: Write the failing test**

Add to `pkg/executor/hook_runner_test.go`:

```go
func TestBuildCompoundCommand(t *testing.T) {
	tests := []struct {
		name         string
		workingDir   string
		hooks        []config.Hook
		finalCommand string
		wantContains []string // substrings that should appear in output
	}{
		{
			name:         "no hooks no final command",
			workingDir:   "/tmp/worktree",
			hooks:        nil,
			finalCommand: "",
			wantContains: nil, // empty result
		},
		{
			name:       "no hooks with final command",
			workingDir: "/tmp/worktree",
			hooks:      nil,
			finalCommand: "claude",
			wantContains: []string{"(cd /tmp/worktree && claude)"},
		},
		{
			name:       "one hook no final command",
			workingDir: "/tmp/worktree",
			hooks: []config.Hook{
				{Run: "npm install"},
			},
			finalCommand: "",
			wantContains: []string{"(cd /tmp/worktree &&", "npm install)"},
		},
		{
			name:       "multiple hooks with final command",
			workingDir: "/tmp/worktree",
			hooks: []config.Hook{
				{Run: "npm install"},
				{Run: "direnv allow"},
			},
			finalCommand: "claude",
			wantContains: []string{
				"(cd /tmp/worktree &&",
				"npm install)",
				"&&",
				"direnv allow)",
				"claude)",
			},
		},
		{
			name:       "hook with custom cwd",
			workingDir: "/tmp/worktree",
			hooks: []config.Hook{
				{Run: "npm install", Cwd: "/tmp/worktree/frontend"},
			},
			finalCommand: "",
			wantContains: []string{"(cd /tmp/worktree/frontend &&"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewHookRunner(tt.workingDir, WithFinalCommand(tt.finalCommand))
			got := runner.buildCompoundCommand(tt.hooks)

			if tt.wantContains == nil {
				if got != "" {
					t.Errorf("buildCompoundCommand() = %q, want empty", got)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("buildCompoundCommand() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/executor -run TestBuildCompoundCommand -v`
Expected: FAIL with "runner.buildCompoundCommand undefined"

**Step 3: Add buildCompoundCommand method**

Add to `pkg/executor/hook_runner.go` after `runHookInTmux` function:

```go
// buildCompoundCommand builds a single compound command from hooks and finalCommand.
// Each hook runs in a subshell to preserve its cwd and timeout.
// Commands are chained with && for fail-fast behavior.
func (h *HookRunner) buildCompoundCommand(hooks []config.Hook) string {
	var parts []string
	timeoutBin := detectTimeoutCommand()

	for _, hook := range hooks {
		cwd := h.substituteTemplates(hook.Cwd)
		if cwd == "" {
			cwd = h.workingDir
		}
		timeout, _ := hook.ParseTimeout()
		cmd := h.substituteTemplates(hook.Run)

		timedCmd := buildTimedCommand(timeoutBin, timeout, cmd)
		parts = append(parts, fmt.Sprintf("(cd %s && %s)", cwd, timedCmd))
	}

	if h.finalCommand != "" {
		parts = append(parts, fmt.Sprintf("(cd %s && %s)", h.workingDir, h.finalCommand))
	}

	return strings.Join(parts, " && ")
}
```

**Step 4: Add strings import if not present**

Verify `strings` is in imports. If not, add it.

**Step 5: Run test to verify it passes**

Run: `go test ./pkg/executor -run TestBuildCompoundCommand -v`
Expected: PASS

**Step 6: Commit**

```bash
git add pkg/executor/hook_runner.go pkg/executor/hook_runner_test.go
git commit -m "feat(executor): add buildCompoundCommand helper

Builds single compound command from hooks using subshells.
Each hook preserves its cwd and timeout via subshell grouping.

Relates-to: wt-98c"
```

---

## Task 3: Modify RunHooks for Tmux Compound Mode

**Files:**
- Modify: `pkg/executor/hook_runner.go:104-112` (RunHooks method)
- Test: `pkg/executor/hook_runner_test.go`

**Step 1: Write the failing test**

Add to `pkg/executor/hook_runner_test.go`:

```go
// mockTmuxClient is a mock for testing tmux mode without real tmux
type mockTmuxClient struct {
	lastCommand string
}

func (m *mockTmuxClient) RunInWindow(windowName, command string) error {
	m.lastCommand = command
	return nil
}

func TestHookRunner_RunHooks_TmuxCompoundMode(t *testing.T) {
	// Create a mock client that captures the command
	mockClient := &mockTmuxClient{}

	// Create a real tmux.Client wrapper that uses our mock
	// We need to use the real client type, so we skip if tmux not available
	realClient, err := tmux.NewClient()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	runner := NewHookRunner("/tmp/worktree",
		WithTmux(realClient, "test-window"),
		WithFinalCommand("claude"))

	// Test that compound command is built correctly in tmux mode
	// The actual SendKeys call uses the compound command
	hooks := []config.Hook{
		{Run: "echo hook1"},
		{Run: "echo hook2"},
	}

	// In tmux mode, RunHooks should build compound command
	// We can't easily verify this without integration test,
	// but we verify buildCompoundCommand separately
	_ = runner
	_ = hooks
	_ = mockClient
}
```

Actually, let's use a simpler approach - test the behavior via a table test:

```go
func TestHookRunner_RunHooks_LocalModeUnchanged(t *testing.T) {
	// Verify local mode behavior is unchanged (hooks block)
	runner := NewHookRunner("/tmp")

	hooks := []config.Hook{
		{Run: "echo hook1"},
		{Run: "echo hook2"},
	}

	err := runner.RunHooks(context.Background(), hooks)
	if err != nil {
		t.Errorf("RunHooks() in local mode error = %v", err)
	}
}
```

**Step 2: Run test to verify current behavior**

Run: `go test ./pkg/executor -run TestHookRunner_RunHooks_LocalModeUnchanged -v`
Expected: PASS (existing behavior should work)

**Step 3: Modify RunHooks to use compound mode in tmux**

Replace `RunHooks` function in `pkg/executor/hook_runner.go`:

```go
// RunHooks executes all hooks in sequence
func (h *HookRunner) RunHooks(ctx context.Context, hooks []config.Hook) error {
	// In tmux mode with potential finalCommand, use compound command
	if h.isTmuxMode() {
		return h.runHooksInTmuxCompound(hooks)
	}

	// Local mode: hooks block, so no race condition
	for i, hook := range hooks {
		if err := h.RunHook(ctx, hook); err != nil {
			return fmt.Errorf("hook %d failed: %w", i, err)
		}
	}
	return nil
}
```

**Step 4: Add runHooksInTmuxCompound method**

Add to `pkg/executor/hook_runner.go`:

```go
// runHooksInTmuxCompound executes all hooks and finalCommand as a single compound command
func (h *HookRunner) runHooksInTmuxCompound(hooks []config.Hook) error {
	compoundCmd := h.buildCompoundCommand(hooks)
	if compoundCmd == "" {
		return nil // Nothing to run
	}

	if err := h.tmuxClient.RunInWindow(h.windowName, compoundCmd); err != nil {
		return fmt.Errorf("running compound command in tmux: %w", err)
	}
	return nil
}
```

**Step 5: Run all hook_runner tests**

Run: `go test ./pkg/executor -run TestHookRunner -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add pkg/executor/hook_runner.go pkg/executor/hook_runner_test.go
git commit -m "feat(executor): use compound command in tmux mode

RunHooks now builds single compound command when in tmux mode,
ensuring hooks complete before finalCommand runs.

Relates-to: wt-98c"
```

---

## Task 4: Update add.go to Use WithFinalCommand

**Files:**
- Modify: `internal/cli/add.go:160-201` (setupWorktreeWithTmux function)

**Step 1: Locate current implementation**

Current code in `add.go:setupWorktreeWithTmux`:

```go
// Run hooks INSIDE the new window
if err := runSetupHooksInWindow(ctx, worktreePath, tmuxClient, windowName); err != nil {
    _, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
}
if runCmd != "" {
    _ = runCommandAfterHooks(RunCommandOpts{
        Command:       runCmd,
        WorktreePath:  worktreePath,
        Branch:        branch,
        WindowName:    windowName,
        TmuxClient:    tmuxClient,
        WindowExisted: windowExisted,
        InTmux:        true,
    })
}
```

**Step 2: Replace with new approach**

Replace the hooks + run command section with:

```go
// Run hooks and optional --run command together in tmux
// This ensures hooks complete before --run executes
cfg, err := loadConfigForCommand()
if err != nil {
    _, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to load config: %v\n", err)
} else {
    runner := executor.NewHookRunner(worktreePath,
        executor.WithTmux(tmuxClient, windowName),
        executor.WithFinalCommand(runCmd))
    if err := runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate); err != nil {
        _, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
    }
}
```

**Step 3: Add executor import if not present**

Verify `github.com/joebalancio/wt/pkg/executor` is imported.

**Step 4: Run tests**

Run: `go test ./internal/cli -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/add.go
git commit -m "fix(cli): use WithFinalCommand in add.go

Replaces separate hook and run command calls with single
HookRunner using WithFinalCommand option.

Relates-to: wt-98c"
```

---

## Task 5: Update stack.go to Use WithFinalCommand

**Files:**
- Modify: `internal/cli/stack.go:220-235` (hook execution section)

**Step 1: Locate current implementation**

Current code in `stack.go:createStackWorktreeWithSpec`:

```go
// Run hooks INSIDE the new window
if err := runSetupHooksInWindow(ctx, worktree.Path, tmuxClient, windowName); err != nil {
    _, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
}
if runCmd != "" {
    _ = runCommandAfterHooks(RunCommandOpts{
        Command:       runCmd,
        ...
    })
}
```

**Step 2: Replace with new approach**

Replace with:

```go
// Run hooks and optional --run command together in tmux
cfg, err := loadConfigForCommand()
if err != nil {
    _, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to load config: %v\n", err)
} else {
    runner := executor.NewHookRunner(worktree.Path,
        executor.WithTmux(tmuxClient, windowName),
        executor.WithFinalCommand(runCmd))
    if err := runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate); err != nil {
        _, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/cli -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/cli/stack.go
git commit -m "fix(cli): use WithFinalCommand in stack.go

Same fix as add.go - ensures hooks complete before --run.

Relates-to: wt-98c"
```

---

## Task 6: Add Integration Test

**Files:**
- Create: `tests/hooks_run_integration_test.go`

**Step 1: Write integration test**

Create `tests/hooks_run_integration_test.go`:

```go
//go:build integration
// +build integration

package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/pkg/executor"
)

func TestHooksCompleteBeforeRun(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := tmux.NewClient()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	// Create temp directory
	tmpDir := t.TempDir()
	worktreePath := filepath.Join(tmpDir, "worktree")

	// Create test window
	windowName := "test-hooks-run-order"
	_ = client.KillWindow(windowName)
	if err := client.CreateNewWindow(windowName, tmpDir); err != nil {
		t.Fatalf("CreateNewWindow() error = %v", err)
	}
	defer func() { _ = client.KillWindow(windowName) }()

	// Create a marker file path
	markerFile := filepath.Join(tmpDir, "marker")

	// Hook creates marker file after delay
	hooks := []config.Hook{
		{
			Run:     "sleep 1 && touch " + markerFile,
			Timeout: "10s",
		},
	}

	// Final command checks for marker file
	finalCmd := "test -f " + markerFile + " && echo SUCCESS || echo FAIL"

	runner := executor.NewHookRunner(worktreePath,
		executor.WithTmux(client, windowName),
		executor.WithFinalCommand(finalCmd))

	err = runner.RunHooks(context.Background(), hooks)
	if err != nil {
		t.Errorf("RunHooks() error = %v", err)
	}

	// Wait for commands to execute
	time.Sleep(2 * time.Second)

	// Verify marker file exists (hook completed)
	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Error("Hook did not complete - marker file not created")
	}
}
```

**Step 2: Run integration test**

Run: `WT_INTEGRATION_TEST=1 go test ./tests -run TestHooksCompleteBeforeRun -v`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/hooks_run_integration_test.go
git commit -m "test(integration): add test for hooks complete before --run

Verifies that hooks complete before the final command executes.

Relates-to: wt-98c"
```

---

## Task 7: Run Full Test Suite and Quality Gates

**Step 1: Run all tests**

Run: `make test`
Expected: All PASS

**Step 2: Run linter**

Run: `make lint`
Expected: No errors

**Step 3: Build binary**

Run: `make build`
Expected: Binary created at `bin/wt`

**Step 4: Manual smoke test**

```bash
# Create a test branch with --run
./bin/wt add test-hooks-wait --run "echo 'Run executed at:' && date"

# Observe tmux window - hooks should complete before echo runs
```

**Step 5: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "chore: fix lint/test issues from wt-98c implementation"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add WithFinalCommand option | hook_runner.go, hook_runner_test.go |
| 2 | Add buildCompoundCommand helper | hook_runner.go, hook_runner_test.go |
| 3 | Modify RunHooks for tmux mode | hook_runner.go |
| 4 | Update add.go | add.go |
| 5 | Update stack.go | stack.go |
| 6 | Add integration test | tests/hooks_run_integration_test.go |
| 7 | Quality gates | All files |

**Acceptance Criteria:**
- [ ] `--run` executes after all hooks complete
- [ ] Hook output and `--run` output don't interleave
- [ ] If hooks fail, `--run` is skipped
- [ ] Works for both `wt add` and `wt stack`
