# Design: Run Hooks Inside New Tmux Window

**Issue:** wt-4gg
**Date:** 2026-02-21
**Status:** Approved

## Problem

Currently, `on_worktree_create` hooks run in the current terminal before the tmux window is created. Users don't see hook output in their new workspace — it scrolls by in the original window and is lost when they switch.

## Solution

Reorder operations: create tmux window first, then run hooks inside it using `tmux run-shell`. Users see live output and end up in the new workspace ready to work.

## Flow Changes

### Current Flow (`wt add`)

```
1. git worktree add
2. runSetupHooks() ← in current terminal
3. createTmuxWindowForWorktree()
```

### New Flow (`wt add`)

```
1. git worktree add
2. createTmuxWindowForWorktree()
3. selectWindow()
4. runSetupHooksInWindow() ← hooks run inside new window
```

Same reordering applies to `wt stack`.

### Commands Not Affected

- `wt setup` — user already in worktree, current behavior is correct
- `wt add --no-tmux` — falls back to current behavior (hooks in terminal)
- Running outside tmux — same as `--no-tmux`

## Implementation

### 1. Tmux Client (`internal/tmux/session.go`)

Add method to run commands in a window:

```go
// RunInWindow runs a command in the specified window and blocks until completion.
// Output appears in the window's pane. Returns any error from the command.
func (c *Client) RunInWindow(windowName, command string) error
```

Implementation:
```bash
tmux run-shell -t <windowName> "<command>"
```

### 2. Config (`internal/config/config.go`)

Add timeout field to Hook struct:

```go
type Hook struct {
    Run     string `yaml:"run"`
    Cwd     string `yaml:"cwd,omitempty"`
    Timeout string `yaml:"timeout,omitempty"`  // e.g., "30s", "2m", "1h"
}
```

**Timeout rules:**
- Default: 30 seconds
- Must use explicit units (`s`, `m`, `h`)
- Bare numbers like `30` will fail to parse (falls back to default)

**Example:**
```yaml
hooks:
  on_worktree_create:
    - run: npm install
      timeout: 2m
    - run: code .
      timeout: 10s
    - run: echo "Ready!"
      # uses default 30s
```

### 3. HookRunner (`pkg/executor/hook_runner.go`)

Extend to support tmux execution:

```go
type HookRunner struct {
    workingDir   string
    templateVars map[string]string
    tmuxClient   *tmux.Client  // nil = run locally
    windowName   string        // used if tmuxClient is set
}

func NewHookRunner(workingDir string, tmuxClient *tmux.Client, windowName string, templateVars ...map[string]string) *HookRunner
```

**Timeout enforcement:**

- **Local execution:** Go process management with `cmd.Process.Kill()` on timeout
- **Tmux execution:** Shell `timeout`/`gtimeout` wrapper (best effort)

Detect available timeout command at initialization:

```go
var timeoutCmd string

func init() {
    if _, err := exec.LookPath("timeout"); err == nil {
        timeoutCmd = "timeout"
    } else if _, err := exec.LookPath("gtimeout"); err == nil {
        timeoutCmd = "gtimeout"  // macOS with coreutils
    }
}
```

### 4. CLI Changes

#### `internal/cli/add.go`

```go
func runAddCommand(cmd *cobra.Command, branch, base, path string, force bool, track string, noCheckout bool) {
    // ... existing setup ...

    wt, err := svc.Add(ctx, spec)
    if err != nil {
        Fatal("Failed to add worktree: %v", err)
    }

    fmt.Fprintf(cmd.OutOrStdout(), "Created worktree: %s [%s]\n", wt.Path, wt.Branch)

    if shouldCreateTmuxWindow(NoTmux()) {
        tmuxClient, _ := tmux.NewClient()
        windowName := tmux.GenerateWindowName(wt.Branch)

        if err := tmuxClient.CreateOrSelectWindow(windowName, wt.Path); err != nil {
            fmt.Fprint(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
        }

        tmuxClient.SelectWindow(windowName)

        if err := runSetupHooksInWindow(ctx, wt.Path, tmuxClient, windowName); err != nil {
            fmt.Fprint(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
        }
        // Stay in window regardless of success/failure
    } else {
        if err := runSetupHooks(ctx, wt.Path); err != nil {
            fmt.Fprint(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
        }
    }
}
```

#### `internal/cli/stack.go`

Same pattern in `createStackWorktree()`.

#### `internal/cli/root.go`

Add helper function:

```go
func runSetupHooksInWindow(ctx context.Context, worktreePath string, tmuxClient *tmux.Client, windowName string) error {
    cfg, err := loadConfigForCommand()
    if err != nil {
        return err
    }

    runner := executor.NewHookRunner(worktreePath, tmuxClient, windowName)
    return runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate)
}
```

### 5. Doctor and Init Checks (`internal/cli/doctor.go`, `internal/cli/init.go`)

Add check for timeout command availability:

```go
func checkTimeoutCommand() (path string, found bool) {
    if path, err := exec.LookPath("timeout"); err == nil {
        return path, true
    }
    if path, err := exec.LookPath("gtimeout"); err == nil {
        return path, true
    }
    return "", false
}
```

**Output in `wt doctor`:**
```
✓ git found: /usr/bin/git
✓ tmux found: /usr/bin/tmux
✓ timeout command: /usr/bin/timeout
```

If not found:
```
⚠ timeout/gtimeout not found — hook timeouts won't be enforced in tmux windows
  Install coreutils: brew install coreutils (macOS)
```

This is a soft dependency — everything works without it, just without timeout enforcement in tmux.

## Error Handling

- **On hook failure:** Stay in new window so user can debug immediately
- **On timeout:** Command is killed, error returned, remaining hooks don't run
- **Remaining hooks:** Stop on first failure (consistent with current behavior)
- **No worktree/window cleanup on failure:** User needs the workspace to debug

## Files Changed

| File | Change |
|------|--------|
| `internal/tmux/session.go` | Add `RunInWindow()` method |
| `internal/config/config.go` | Add `Timeout` field to `Hook` struct |
| `pkg/executor/hook_runner.go` | Add tmux execution mode, timeout handling |
| `internal/cli/add.go` | Reorder: window → hooks |
| `internal/cli/stack.go` | Same reordering |
| `internal/cli/root.go` | Add `runSetupHooksInWindow()` helper |
| `internal/cli/doctor.go` | Check for timeout command |
| `internal/cli/init.go` | Same check |

## Testing

- Unit tests for `RunInWindow()` method
- Unit tests for timeout parsing and enforcement
- Integration tests for `wt add` with hooks
- Integration tests for `wt stack` with hooks
- Test behavior when timeout command unavailable
- Test error handling (hook failure, timeout)
