# WT Open Command Design

**Status:** Design Phase
**Author:** WT Team
**Created:** 2025-01-25
**Related:** [TMUX Integration](./2025-01-25-wt-tmux-integration-design.md)

## Overview

`wt open` provides a fast way to navigate to existing worktrees by creating or switching to a tmux window. It complements `wt add` (which creates new worktrees) with a focused command for opening existing ones.

### Goals

1. **Fast navigation** - Quick access to existing worktrees via tmux windows
2. **Smart window reuse** - Switch to existing windows or create new ones
3. **Clear errors** - Helpful messages when worktrees don't exist
4. **Consistent UX** - Reused by `wt add` for window handling

### Non-Goals

- Creating new worktrees (use `wt add`)
- Interactive selection in v1 (deferred)
- Multi-server support (single tmux server only)

---

## Command Syntax

### Basic Usage

```bash
wt open <branch>
```

### Examples

```bash
# Open existing worktree
wt open feat-auth

# Open with setup
wt open feat-auth --setup

# Short form
wt o feat-auth
```

---

## Behavior

### With Argument

```bash
wt open feat-auth
```

**Steps:**
1. Find worktree for branch `feat-auth`
2. Check if in tmux (`$TMUX` env var)
3. Generate window name: `feat-auth` (or abbreviated)
4. Check if window exists in current session
5. If exists: switch to window, update CWD
6. If not exists: create new window with worktree CWD

**Success output:**
```
Switched to window: feat-auth
~/worktrees/my-repo/feat-auth
```

### No Argument

```bash
wt open
```

**Behavior:** List worktrees and suggest usage

**Output:**
```
Available worktrees:
  feat-auth      ~/worktrees/my-repo/feat-auth
  feat-api       ~/worktrees/my-repo/feat-api
  main           ~/worktrees/my-repo/main

Usage: wt open <branch>
  wt open feat-auth
  wt open feat-api
```

### Not in Tmux

```bash
# $TMUX is not set
wt open feat-auth
```

**Error:**
```
Error: Not in tmux session

wt open requires tmux. Start tmux first:
  tmux new-session -s my-repo
```

**Exit code:** 1

---

## Branch Resolution

### Exact Match

```bash
wt open feat-auth
# → Finds worktree with branch "feat-auth"
```

### Fuzzy Match (Future)

```bash
wt open auth
# → Could match "feat-auth" or "auth-provider"
# → Error: Ambiguous match (v1)
# → Interactive selector (v2)
```

**v1 behavior:** Exact match only

---

## Tmux Integration

### Window Reuse

```go
func openWorktreeWindow(worktree *Worktree) error {
    if !isInTmux() {
        return errors.New("not in tmux session")
    }

    windowName := generateWindowName(worktree.Branch)

    if windowExists(windowName) {
        // Switch to existing window
        return switchToWindow(windowName, worktree.Path)
    }

    // Create new window
    return createWindow(windowName, worktree.Path)
}
```

### Switch Behavior

```bash
# Window "feat-auth" already exists
wt open feat-auth
# → tmux select-window -t feat-auth
# → tmux send-keys -t feat-auth "cd ~/worktrees/my-repo/feat-auth" Enter
```

### Create Behavior

```bash
# Window "feat-auth" doesn't exist
wt open feat-auth
# → tmux new-window -c ~/worktrees/my-repo/feat-auth -n feat-auth
```

---

## Flags

| Flag | Type | Description |
|------|------|-------------|
| `--setup` | bool | Run setup hooks after opening |
| `--cmd` | string | Send command to window after opening |
| `--no-tmux` | bool | Don't use tmux (future: print path or cd) |

### Examples

```bash
# Open with setup
wt open feat-auth --setup

# Open and run vim
wt open feat-auth --cmd vim

# Open without tmux (future)
wt open feat-auth --no-tmux
# → cd ~/worktrees/my-repo/feat-auth
```

---

## Error Handling

### Worktree Not Found

```bash
wt open feat-xyz
```

**Error:**
```
Error: Worktree 'feat-xyz' not found

Available worktrees:
  feat-auth      ~/worktrees/my-repo/feat-auth
  feat-api       ~/worktrees/my-repo/feat-api
  main           ~/worktrees/my-repo/main
```

**Exit code:** 1

### Ambiguous Match (Future)

```bash
wt open auth
```

**Error (v1):**
```
Error: Ambiguous match for 'auth'

Did you mean:
  feat-auth      ~/worktrees/my-repo/feat-auth
  auth-provider  ~/worktrees/my-repo/auth-provider

Be more specific or use exact branch name.
```

**Exit code:** 1

### Not in Tmux

```bash
wt open feat-auth
```

**Error:**
```
Error: Not in tmux session

wt open requires tmux. Start tmux first:
  tmux new-session -s my-repo

Or use --no-tmux to print the worktree path.
```

**Exit code:** 1

---

## Integration with wt add

### Shared Logic

`wt add` reuses the window opening logic from `wt open`:

```go
// In wt add command
func addWorktree(spec WorktreeCreateSpec) error {
    // 1. Create worktree
    worktree, err := git.AddWorktree(spec)
    if err != nil {
        return err
    }

    // 2. Open in tmux (reuses wt open logic)
    if shouldOpenInTmux() {
        if err := openWorktreeWindow(worktree); err != nil {
            // Log warning but don't fail
            log.Warnf("Failed to open tmux window: %v", err)
        }
    }

    return nil
}
```

### Consistent Behavior

| Scenario | `wt add` | `wt open` |
|----------|----------|-----------|
| Worktree exists | Error (already exists) | Switch to window |
| Worktree doesn't exist | Creates worktree | Error (not found) |
| Window exists | Switches to window | Switches to window |
| Not in tmux | Creates worktree only | Error |

---

## Command Reference

### wt open

```
Open an existing worktree in a new tmux window

Usage:
  wt open <branch>
  wt open [flags]
  wt o <branch>    # short form

Arguments:
  <branch>    Branch name of existing worktree (exact match)

Flags:
  --setup      Run setup hooks after opening
  --cmd <cmd>  Send command to window after opening
  --no-tmux    Don't use tmux (prints path)

Examples:
  # Open existing worktree
  wt open feat-auth

  # Open and run setup
  wt open feat-auth --setup

  # Open and run vim
  wt open feat-auth --cmd vim

  # No argument: list worktrees
  wt open
```

---

## Implementation

### Package Structure

```
internal/cli/
├── open.go           # wt open command
├── add.go            # wt add command (uses open logic)
└── tmux/
    └── window.go      # Shared window management
```

### Core Functions

```go
// Command implementation
func NewOpenCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:     "open <branch>",
        Aliases: []string{"o"},
        Short:   "Open existing worktree in tmux",
        Run:     runOpen,
    }

    cmd.Flags().BoolVar(&opts.Setup, "setup", false, "Run setup hooks")
    cmd.Flags().StringVar(&opts.Cmd, "cmd", "", "Send command to window")
    cmd.Flags().BoolVar(&opts.NoTmux, "no-tmux", false, "Don't use tmux")

    return cmd
}

func runOpen(cmd *cobra.Command, args []string) {
    if len(args) == 0 {
        // List worktrees and suggest usage
        listWorktrees()
        fmt.Println("\nUsage: wt open <branch>")
        return
    }

    branch := args[0]

    // Find worktree
    worktree, err := findWorktree(branch)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Open in tmux
    if err := openWorktreeWindow(worktree); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    // Run setup if requested
    if opts.Setup {
        runSetupHooks(worktree.Path)
    }

    // Send command if requested
    if opts.Cmd != "" {
        sendKeysToWindow(opts.Cmd)
    }
}

// Core logic (shared with wt add)
func findWorktree(branch string) (*Worktree, error) {
    worktrees := git.ListWorktrees()

    // Exact match
    for _, wt := range worktrees {
        if wt.Branch == branch {
            return wt, nil
        }
    }

    // Build error with available worktrees
    var list []string
    for _, wt := range worktrees {
        list = append(list, fmt.Sprintf("  %s\t%s", wt.Branch, wt.Path))
    }

    return nil, fmt.Errorf("worktree '%s' not found\n\nAvailable worktrees:\n%s",
        branch, strings.Join(list, "\n"))
}

func openWorktreeWindow(worktree *Worktree) error {
    if !tmux.IsActive() {
        return errors.New("not in tmux session")
    }

    windowName := tmux.GenerateWindowName(worktree.Branch)

    if tmux.WindowExists(windowName) {
        return tmux.SwitchToWindow(windowName, worktree.Path)
    }

    return tmux.CreateWindow(windowName, worktree.Path)
}
```

---

## Implementation Phases

### Phase 1: Core Open
- [ ] Implement `wt open` command
- [ ] Add worktree lookup (exact match)
- [ ] Add tmux detection and error
- [ ] Implement window switching
- [ ] Implement window creation

### Phase 2: Window Management
- [ ] Extract shared logic to `internal/tmux/window.go`
- [ ] Update `wt add` to use shared logic
- [ ] Add window name generation
- [ ] Add window existence check

### Phase 3: Flags & Enhancements
- [ ] Add `--setup` flag
- [ ] Add `--cmd` flag
- [ ] Add `--no-tmux` flag
- [ ] Improve error messages

### Phase 4: Polish
- [ ] Add tests
- [ ] Update documentation
- [ ] Add completion scripts

---

## Open Questions

1. **Short alias:** Should `wt o` be documented or just an implementation detail?
   - **Decision:** Document it, it's useful

2. **Fuzzy match:** Implement in v1 or v2?
   - **Decision:** v2 - keep v1 simple with exact match only

3. **--no-tmux behavior:** Print path, try to cd, or error?
   - **Decision:** Error in v1, add path printing in v2

4. **Multiple matches:** Error or interactive?
   - **Decision:** Error in v1, interactive in v2

---

## Dependencies

### External Tools
```
tmux >= 1.9  (window creation/switching)
git >= 2.30  (worktree listing)
```

### Go Modules
```
github.com/spf13/cobra       # CLI framework
github.com/aidarkhanov/nanoid/v3  # Nanoid (future)
```

---

## References

- [TMUX Integration Design](./2025-01-25-wt-tmux-integration-design.md)
- [v2 Stacking Design](./2025-01-25-wt-v2-stacking-design.md)
- [Original Bash Script](~/projects/home/bin/wt)
