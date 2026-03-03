# Design Plan: Wait for Hooks to Complete Before --run

**Issue**: wt-98c
**Date**: 2026-02-28
**Status**: Draft

## Problem Statement

When using `wt add <branch> --run "<command>"`, the command executes while setup hooks are still running in the tmux window. This is a race condition.

### Root Cause

The execution flow in `internal/cli/add.go:setupWorktreeWithTmux()`:

1. `runSetupHooksInWindow()` sends hook commands to tmux via `SendKeys` (fire-and-forget)
2. Immediately after, `runCommandAfterHooks()` sends the `--run` command via `SendKeys` (also fire-and-forget)

Both operations use tmux `send-keys`, which queues commands but doesn't wait for completion.

### Code Path

```
add.go:187  → runSetupHooksInWindow()
    → root.go:96 → runner.RunHooks()
        → hook_runner.go:136 → runHookInTmux()
            → session.go:461 → RunInWindow() → SendKeys() [FIRE-AND-FORGET]

add.go:191  → runCommandAfterHooks() [IMMEDIATELY AFTER]
    → root.go:173 → runCommandInTmuxWindow() → SendKeys() [FIRE-AND-FORGET]
```

## Solution Design

### Chosen Approach: WithFinalCommand Option

Add a new option to `HookRunner` that appends a final command to the hook chain. In tmux mode, build a single compound command using subshells.

### Key Decision: Fail-Fast Behavior

If any hook fails, `--run` is skipped entirely. This ensures `--run` only executes when setup is successful. Uses `&&` chaining.

### Key Decision: Subshells for Per-Hook Context

Each hook runs in a subshell to preserve per-hook `cwd` and timeout:

```bash
(cd /path && timeout 300s npm install) && \
(cd /path && timeout 300s direnv allow) && \
(cd /path && claude)
```

### Trade-offs

| Trade-off | Decision | Rationale |
|-----------|----------|-----------|
| Env var propagation | Accept isolation | Hooks rarely export vars for other hooks |
| Path quoting | Assume normal paths | wt controls path generation (YAGNI) |

## Implementation Details

### HookRunner Changes (`pkg/executor/hook_runner.go`)

```go
type HookRunner struct {
    workingDir    string
    templateVars  map[string]string
    tmuxClient    *tmux.Client
    windowName    string
    finalCommand  string  // NEW: command to run after all hooks
}

// WithFinalCommand sets a command to execute after all hooks complete
func WithFinalCommand(cmd string) HookRunnerOption {
    return func(hr *HookRunner) {
        hr.finalCommand = cmd
    }
}
```

### Modified RunHooks() Behavior

```go
func (h *HookRunner) RunHooks(ctx context.Context, hooks []config.Hook) error {
    if h.isTmuxMode() {
        return h.runHooksInTmuxCompound(hooks)
    }
    // Local mode unchanged - hooks already block
    for i, hook := range hooks {
        if err := h.RunHook(ctx, hook); err != nil {
            return fmt.Errorf("hook %d failed: %w", i, err)
        }
    }
    return nil
}

func (h *HookRunner) runHooksInTmuxCompound(hooks []config.Hook) error {
    var parts []string
    for _, hook := range hooks {
        cwd := h.substituteTemplates(hook.Cwd)
        if cwd == "" {
            cwd = h.workingDir
        }
        timeout, _ := hook.ParseTimeout()
        cmd := h.substituteTemplates(hook.Run)

        timeoutBin := detectTimeoutCommand()
        timedCmd := buildTimedCommand(timeoutBin, timeout, cmd)

        // Build subshell: (cd <cwd> && <timedCmd>)
        parts = append(parts, fmt.Sprintf("(cd %s && %s)", cwd, timedCmd))
    }

    if h.finalCommand != "" {
        parts = append(parts, fmt.Sprintf("(cd %s && %s)", h.workingDir, h.finalCommand))
    }

    compoundCmd := strings.Join(parts, " && ")
    return h.tmuxClient.RunInWindow(h.windowName, compoundCmd)
}
```

### CLI Changes (`internal/cli/add.go`)

Before:
```go
if err := runSetupHooksInWindow(ctx, worktreePath, tmuxClient, windowName); err != nil {
    // ...
}
if runCmd != "" {
    _ = runCommandAfterHooks(RunCommandOpts{...})
}
```

After:
```go
runner := executor.NewHookRunner(worktreePath,
    executor.WithTmux(tmuxClient, windowName),
    executor.WithFinalCommand(runCmd))
if err := runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate); err != nil {
    // ...
}
```

## Files Changed

| File | Change |
|------|--------|
| `pkg/executor/hook_runner.go` | Add `finalCommand` field, `WithFinalCommand()` option, `runHooksInTmuxCompound()` method |
| `pkg/executor/hook_runner_test.go` | New unit tests for compound command building |
| `internal/cli/add.go` | Use `WithFinalCommand()` instead of separate `runCommandAfterHooks()` call |
| `internal/cli/stack.go` | Same change as add.go |
| `internal/cli/root.go` | Simplify or remove unused helper functions |

## Testing Strategy

### Unit Tests (`pkg/executor/hook_runner_test.go`)

| Test Case | Input | Expected |
|-----------|-------|----------|
| No hooks, no run | `hooks=[]`, `finalCommand=""` | No error, no SendKeys call |
| No hooks, with run | `hooks=[]`, `finalCommand="claude"` | `(cd /path && claude)` |
| One hook, no run | `hooks=[npm install]` | `(cd /path && timeout 300s npm install)` |
| Multiple hooks, with run | 2 hooks + run | Subshells chained with `&&` |
| Hook with custom cwd | `cwd: "{worktree_path}/frontend"` | Subshell with correct path |

### Integration Tests (`tests/hooks_run_integration_test.go`)

| Test Case | Setup | Verify |
|-----------|-------|--------|
| Run executes after hooks | Configure slow hook (sleep 2) + run command | Run output appears AFTER hook output |
| Hook failure skips run | Hook exits 1 + run command | Run command never executes |
| Outside tmux mode | Run locally | Existing blocking behavior preserved |

### TDD Approach

1. **Red**: Write failing test that verifies `--run` waits for hooks
2. **Green**: Implement `WithFinalCommand` and compound command building
3. **Refactor**: Clean up duplicate code in add.go/stack.go

## Acceptance Criteria

1. When `--run` is specified, it MUST execute after all setup hooks complete
2. Hook output and `--run` output MUST NOT interleave
3. If hooks fail, `--run` should be skipped
4. Behavior must work for both `wt add` and `wt stack`

## Backward Compatibility

- No `--run` specified → identical behavior (hooks run separately)
- With `--run` → new compound command behavior
- **Documented change**: Environment variables exported by hooks don't propagate to `--run` command (subshell isolation)
