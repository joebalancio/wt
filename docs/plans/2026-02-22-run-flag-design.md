# Design: `--run` Flag for `wt add` and `wt stack`

**Date**: 2026-02-22
**Bead**: wt-cdw
**Status**: Approved

## Summary

Add a `--run <command>` flag to `wt add` and `wt stack` commands that executes a user-specified command after all `on_worktree_create` hooks complete.

## Core Behavior

| Context | Behavior |
|---------|----------|
| **In tmux** | Fire-and-forget: send command to the new tmux window via `SendKeys`, wt returns immediately |
| **Outside tmux** | Exec replacement: wt replaces its process with the command using `syscall.Exec` |

### Execution Order

```
wt add feat/auth --run "claude"
        │
        ▼
   1. Create worktree
        │
        ▼
   2. Create/select tmux window (if in tmux)
        │
        ▼
   3. Run on_worktree_create hooks
        │
        ▼
   4. Run --run command (if specified, if window was newly created)
```

### Key Decisions

- **Template support**: `{worktree_path}` only (follow-up bead for `{branch}`)
- **Command syntax**: Shell pass-through via `sh -c "<command>"`
- **Error handling**: Non-zero exit if command fails (--run is intentional)
- **Existing window**: Skip `--run` if window already existed (don't interrupt)
- **Commands affected**: Both `wt add` and `wt stack`

## Implementation

### Files to Modify

| File | Changes |
|------|---------|
| `internal/cli/add.go` | Add `--run` flag, pass to `setupWorktreeWithTmux`, handle exec replacement |
| `internal/cli/stack.go` | Add `--run` flag, same pattern as add.go |
| `internal/cli/root.go` | Add helper functions for run command execution |

### CLI Flag Addition

```go
// In NewAddCmd() and NewStackCmd()
var run string
cmd.Flags().StringVar(&run, "run", "",
    "command to run after hooks (e.g., 'claude')")
```

### Helper Function Signature (root.go)

```go
// runCommandAfterHooks executes the --run command in the appropriate context
// Returns true if a new window was created and command was sent
func runCommandAfterHooks(ctx context.Context, opts RunCommandOpts) error

type RunCommandOpts struct {
    Command       string
    WorktreePath  string
    WindowName    string
    TmuxClient    *tmux.Client
    WindowExisted bool   // if true, skip running
    InTmux        bool
}
```

### Exec Replacement Pattern (non-tmux case)

```go
func execReplace(worktreePath, command string) error {
    // Expand template
    cmd = strings.ReplaceAll(command, "{worktree_path}", worktreePath)

    // Find executable
    absPath, err := exec.LookPath("sh")
    if err != nil {
        return fmt.Errorf("command not found: %w", err)
    }

    // Change to worktree directory
    if err := os.Chdir(worktreePath); err != nil {
        return err
    }

    // Replace process
    return syscall.Exec(absPath, []string{"sh", "-c", cmd}, os.Environ())
}
```

## Error Handling

| Scenario | Handling |
|----------|----------|
| Empty `--run ""` | No-op, skip silently |
| Command not found | **Non-tmux**: Error + non-zero exit before exec<br>**Tmux**: Warn but don't fail (can't know if command exists in target shell) |
| Window already existed | **Print message**: `"--run skipped: window '{name}' already exists"` then continue |
| Tmux SendKeys fails | Warn to stderr, don't fail the add operation |

### Template Expansion

```go
func expandRunTemplate(command, worktreePath string) string {
    return strings.ReplaceAll(command, "{worktree_path}", worktreePath)
}
```

Only `{worktree_path}` is supported. Invalid/unknown templates are passed through unchanged (no error).

## Edge Cases

### No Tmux Available

If `--run` is specified but tmux isn't available (not in tmux session or `--no-tmux`):

1. Hooks run locally (blocking)
2. `--run` command executes via exec replacement
3. wt process is replaced - never returns

### Mixed Flags

```bash
wt add feat/auth --run "claude" --no-tmux
```

This is valid - creates worktree without tmux window, then exec's into claude in the worktree directory.

## Testing Strategy

### Unit Tests (Foundation)

**File**: `internal/cli/run_command_test.go`

| Test | Description |
|------|-------------|
| `TestExpandRunTemplate` | `{worktree_path}` → expanded path |
| `TestExpandRunTemplate/empty_command` | `""` → `""` (no-op) |
| `TestExpandRunTemplate/unknown_template` | `{unknown}` → `{unknown}` (passthrough) |
| `TestShouldSkipRun` | `windowExisted=true` → skip, `false` → run |
| `TestBuildShellCommand` | `claude` → `["sh", "-c", "claude"]` |
| `TestBuildShellCommand/with_quotes` | preserves quoting |

### Integration Tests (Middle)

**File**: `tests/run_flag_integration_test.go`

| Test | Scope |
|------|-------|
| `TestAddRunFlag_Tmux` | Full `wt add --run` flow in tmux (requires tmux) |
| `TestAddRunFlag_NoTmux` | Full flow outside tmux |
| `TestStackRunFlag` | `wt stack --run` basic flow |

### E2E/Manual Tests (Top)

```bash
# Smoke tests only - run before release
./scripts/test-run-flag.sh  # script that exercises key paths
```

### Test Distribution

```
        ╱╲
       ╱  ╲        E2E: 2-3 manual scenarios
      ╱────╲
     ╱      ╲      Integration: 3-4 tests
    ╱────────╲
   ╱          ╲    Unit: 8-10 tests (fast, no external deps)
  ╱────────────╲
```

## Documentation Updates

| File | Update |
|------|--------|
| `CLAUDE.md` | Add `--run` flag to `wt add` section |
| `internal/cli/add.go` | Update command `Long` description |
| `internal/cli/stack.go` | Update command `Long` description |

### CLAUDE.md Addition

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

## Follow-up Work

Create a bead to add `{branch}` template support to all hooks and `--run`:

```bash
bd create --title="Add {branch} template to hooks and --run" \
  --description="Extend template support to include {branch} variable in all hook commands and --run flag" \
  --type=feature --priority=3
```
