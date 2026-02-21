# Interactive Picker Integration Design

**Date:** 2026-02-21
**Issue:** wt-eex
**Status:** Approved

## Overview

Add interactive TUI pickers to `wt add` and `wt remove` commands when running from the main repository with no arguments. Uses **charmbracelet/huh** as the selection library.

## Triggers

| Command | Trigger Condition | Behavior |
|---------|-------------------|----------|
| `wt add` | No args + from main repo + not in worktree | Show branch picker → optionally base branch picker |
| `wt remove` | No args + from main repo | Show worktree picker |

## Non-Goals (YAGNI)

- No `wt select` command (use `git worktree list`)
- No configuration option for enabling/disabling
- No `--fzf` flag (auto-detect is cleaner)
- No preview panes (can add later if needed)

## Library Selection

**Chosen:** [charmbracelet/huh](https://github.com/charmbracelet/huh)

### Why huh?

1. **Purpose-built for selection** - `huh.NewSelect()` is exactly what we need
2. **Minimal boilerplate** - 5-10 lines of code vs 50+ with raw Bubble Tea
3. **Built-in accessibility** - Screen reader support out of the box
4. **Integrates with Bubble Tea** - Can embed in larger TUI if needed later
5. **Active maintenance** - Part of the Charm ecosystem, well-supported

### Alternatives Considered

| Library | Stars | Status | Reason Rejected |
|---------|-------|--------|-----------------|
| AlecAivazis/survey | ~4k | **Deprecated** | Author recommends Bubble Tea |
| rivo/tview | ~10k | Active | Overkill for simple selection |
| c-bata/go-prompt | ~5k | Low activity | REPL-focused, not forms |
| pterm/pterm | ~4.5k | Active | Less polished select component |

## Architecture

### New Package: `internal/picker`

```
internal/picker/
├── picker.go       # Core picker functionality using huh
└── picker_test.go  # Unit tests
```

### Core Components

**1. `Picker` struct** - Wraps huh with wt-specific logic
```go
type Picker struct {
    gitClient *git.Client
}

func NewPicker(gitClient *git.Client) *Picker

// SelectWorktree shows a picker for existing worktrees
func (p *Picker) SelectWorktree(ctx context.Context) (path string, err error)

// SelectBranch shows a picker for branches (existing or new)
func (p *Picker) SelectBranch(ctx context.Context) (branch string, baseBranch string, isNew bool, err error)
```

**2. TTY Detection** - Only show picker in interactive terminal
```go
func IsTerminal() bool {
    return term.IsTerminal(int(os.Stdout.Fd()))
}
```

**3. Integration Points**

| File | Change |
|------|--------|
| `internal/cli/add.go` | If no args + TTY + main repo → call `picker.SelectBranch()` |
| `internal/cli/remove.go` | If no args + TTY + main repo → call `picker.SelectWorktree()` |

## Flows

### Branch Picker Flow (for `wt add`)

```
1. Get all branches (local + remote)
2. Add "Create new branch" option
3. User selects:
   - Existing branch → return (branch, "", false)
   - Create new → prompt for name → prompt for base branch → return (name, base, true)
```

### Worktree Picker Flow (for `wt remove`)

```
1. List worktrees via gitClient
2. Format as "branch → /path/to/worktree"
3. User selects → return path
```

## Implementation Details

### File 1: `internal/picker/picker.go`

```go
package picker

import (
    "context"
    "fmt"
    "os"

    "github.com/charmbracelet/huh"
    "github.com/joebalancio/wt/internal/git"
    "golang.org/x/term"
)

const newBranchOption = "Create new branch"

type Picker struct {
    gitClient *git.Client
}

func NewPicker(gitClient *git.Client) *Picker {
    return &Picker{gitClient: gitClient}
}

func IsTerminal() bool {
    return term.IsTerminal(int(os.Stdout.Fd()))
}

// SelectWorktree presents a picker for selecting a worktree to remove.
func (p *Picker) SelectWorktree(ctx context.Context) (string, error) {
    worktrees, err := p.gitClient.ListWorktrees(ctx)
    if err != nil {
        return "", fmt.Errorf("list worktrees: %w", err)
    }

    if len(worktrees) == 0 {
        return "", fmt.Errorf("no worktrees found")
    }

    var options []huh.Option[string]
    for _, wt := range worktrees {
        // Skip bare/main worktree (the repo itself)
        if wt.Branch == "" {
            continue
        }
        label := fmt.Sprintf("%s -> %s", wt.Branch, wt.Path)
        options = append(options, huh.NewOption(label, wt.Path))
    }

    if len(options) == 0 {
        return "", fmt.Errorf("no removable worktrees found")
    }

    var selected string
    err = huh.NewSelect[string]().
        Title("Select worktree to remove:").
        Options(options...).
        Value(&selected).
        Run()

    return selected, err
}

// SelectBranchResult contains the result of branch selection.
type SelectBranchResult struct {
    Branch     string
    BaseBranch string
    IsNew      bool
}

// SelectBranch presents a picker for selecting or creating a branch.
func (p *Picker) SelectBranch(ctx context.Context) (SelectBranchResult, error) {
    branches, err := p.gitClient.ListAllBranches(ctx)
    if err != nil {
        return SelectBranchResult{}, fmt.Errorf("list branches: %w", err)
    }

    // Build options: new branch option first, then existing branches
    options := []huh.Option[string]{
        huh.NewOption(newBranchOption, newBranchOption),
    }
    for _, b := range branches {
        options = append(options, huh.NewOption(b, b))
    }

    var selected string
    err = huh.NewSelect[string]().
        Title("Select or create a branch:").
        Options(options...).
        Value(&selected).
        Run()

    if err != nil {
        return SelectBranchResult{}, err
    }

    // User chose to create new branch
    if selected == newBranchOption {
        return p.promptNewBranch(ctx, branches)
    }

    // User selected existing branch
    return SelectBranchResult{
        Branch: selected,
        IsNew:  false,
    }, nil
}

func (p *Picker) promptNewBranch(ctx context.Context, existingBranches []string) (SelectBranchResult, error) {
    var branchName string
    err := huh.NewInput().
        Title("Enter new branch name:").
        Value(&branchName).
        Validate(func(s string) error {
            if s == "" {
                return fmt.Errorf("branch name cannot be empty")
            }
            for _, b := range existingBranches {
                if b == s {
                    return fmt.Errorf("branch %q already exists", s)
                }
            }
            return nil
        }).
        Run()

    if err != nil {
        return SelectBranchResult{}, err
    }

    // Prompt for base branch
    var baseBranch string
    baseOptions := make([]huh.Option[string], len(existingBranches))
    for i, b := range existingBranches {
        baseOptions[i] = huh.NewOption(b, b)
    }

    err = huh.NewSelect[string]().
        Title("Select base branch:").
        Options(baseOptions...).
        Value(&baseBranch).
        Run()

    if err != nil {
        return SelectBranchResult{}, err
    }

    return SelectBranchResult{
        Branch:     branchName,
        BaseBranch: baseBranch,
        IsNew:      true,
    }, nil
}
```

### File 2: `internal/git/branch.go` (new file)

Add these methods to the git client:

```go
// ListAllBranches returns all local and remote branches (deduplicated).
func (c *Client) ListAllBranches(ctx context.Context) ([]string, error)

// DefaultBranch returns the default branch (main/master).
func (c *Client) DefaultBranch(ctx context.Context) (string, error)
```

### File 3: `internal/cli/add.go` changes

- Change `Args: cobra.ExactArgs(1)` to `Args: cobra.MaximumNArgs(1)`
- Add picker integration when `branch == ""`

### File 4: `internal/cli/remove.go` changes

- Add picker integration when `path == ""` and running from main repo

## Dependencies

```go
require (
    github.com/charmbracelet/huh v0.6.0  // Form/prompt library
    golang.org/x/term v0.21.0            // TTY detection
)
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| User presses Ctrl+C | `huh` returns `ErrUserAborted` -> exit silently (code 0) |
| No worktrees exist | Return error with message "no worktrees found" |
| No TTY (piped input) | Error: "branch/path argument required when not in interactive terminal" |
| Branch name empty | Validation error in huh form |
| Branch already exists | Validation error when creating new branch |

## Testing Strategy

### Unit tests (`internal/picker/picker_test.go`)
- Mock `git.Client` interface for testing picker logic
- Test branch/worktree formatting
- Test validation logic

### Integration tests (`tests/picker_integration_test.go`)
- Skip if not in TTY environment
- Test full flow with real git repo

### Existing test updates
- `internal/cli/add_test.go` - Add test for `MaximumNArgs(1)` change
- `internal/cli/remove_test.go` - Add test for picker path

## File Summary

| File | Action |
|------|--------|
| `go.mod` | Add `charmbracelet/huh`, `golang.org/x/term` |
| `internal/picker/picker.go` | **New** - Picker implementation |
| `internal/picker/picker_test.go` | **New** - Unit tests |
| `internal/git/branch.go` | **New** - Branch listing methods |
| `internal/git/client_interface.go` | Add `BranchLister` interface |
| `internal/cli/add.go` | Modify - Add picker integration |
| `internal/cli/remove.go` | Modify - Add picker integration |
