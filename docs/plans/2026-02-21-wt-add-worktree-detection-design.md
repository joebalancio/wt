# wt add: Prevent Nested Worktree Creation

**Issue:** wt-4rm
**Date:** 2026-02-21
**Status:** Approved

## Problem

Running `wt add <new-branch>` from inside a worktree directory creates a nested worktree at `<worktree>/.worktrees/...` instead of at the main repo's worktree location.

**Example:**
```bash
# From inside /home/user/repo/.worktrees/feature/foo
wt add feature/bar

# Creates: /home/user/repo/.worktrees/feature/foo/.worktrees/feature/bar
# Should: Block with error - user must run from main repo
```

## Root Cause

`GetRepoInfo()` uses `git rev-parse --show-toplevel` which returns the **current worktree's path** when inside a worktree, not the main repository root. This causes `ResolvePath()` to compute incorrect paths.

## Solution

Block `wt add` when run from inside a worktree with a clear error message directing the user to the main repository.

## Design

### Approach: Add `IsInWorktree()` method to git client

This provides a reusable building block for detecting worktree context.

### 1. Git Client Interface

**New method on `GitClient` interface:**

```go
IsInWorktree(ctx context.Context) (inWorktree bool, mainRepoRoot string, err error)
```

**Implementation in `internal/git/worktree.go`:**

```go
func (c *Client) IsInWorktree(ctx context.Context) (bool, string, error) {
    // Get current toplevel
    toplevel, err := c.getOutput(ctx, "rev-parse", "--show-toplevel")
    if err != nil {
        return false, "", fmt.Errorf("getting toplevel: %w", err)
    }

    // Get common git dir
    gitCommonDir, err := c.getOutput(ctx, "rev-parse", "--git-common-dir")
    if err != nil {
        return false, "", fmt.Errorf("getting git-common-dir: %w", err)
    }

    // Main repo root is parent of .git directory
    mainRepoRoot := filepath.Dir(gitCommonDir)

    // If toplevel differs from main repo root, we're in a worktree
    inWorktree := toplevel != mainRepoRoot
    return inWorktree, mainRepoRoot, nil
}
```

### 2. CLI Layer Integration

**Changes to `internal/cli/add.go`:**

Add validation at the start of the `add` command:

```go
func runAdd(cmd *cobra.Command, args []string) error {
    // ... existing flag parsing ...

    // Check if we're inside a worktree
    gitClient, err := git.NewClient()
    if err != nil {
        return err
    }

    inWorktree, mainRepoRoot, err := gitClient.IsInWorktree(cmd.Context())
    if err != nil {
        return fmt.Errorf("checking worktree context: %w", err)
    }

    if inWorktree {
        return fmt.Errorf(`cannot add worktree from inside another worktree

Current location: %s
Main repository:  %s

Run this command from the main repository instead:
  cd %s && wt add %s`,
            getCurrentToplevel(),
            mainRepoRoot,
            mainRepoRoot,
            branch)
    }

    // ... rest of existing add logic ...
}
```

### 3. Testing Strategy

**Unit tests for `IsInWorktree` in `internal/git/worktree_test.go`:**

- Test in main repository → returns `false`
- Test in worktree → returns `true` with correct main repo root
- Test bare repository edge case

**Integration test in `tests/add_integration_test.go`:**

- Verify error message when running `wt add` from inside a worktree
- Assert error contains "cannot add worktree from inside"
- Assert error shows main repo path

### 4. Edge Cases

| Case | Behavior |
|------|----------|
| Bare repository | `--show-toplevel` returns empty; return error |
| Submodule | Works correctly - detects worktree vs parent repo |
| Detached HEAD | No impact - worktree detection works independently |
| No git repo | Both commands fail; return wrapped error |

## Files to Modify

1. `internal/git/worktree.go` - Add `IsInWorktree()` implementation and interface method
2. `internal/cli/add.go` - Add validation check
3. `internal/git/worktree_test.go` - Add unit tests
4. `internal/worktree/service_test.go` - Update mocks
5. `tests/add_integration_test.go` - Add integration test
