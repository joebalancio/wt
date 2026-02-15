# Design: Enhanced `wt remove` Command

## Overview & Goals

**Problem:** The current `wt remove` only removes the worktree and closes the tmux window — it leaves the branch behind. This creates asymmetry with `wt add`, which creates both branch and worktree together.

**Goal:** Make `wt remove` the natural counterpart to `wt add` by removing the branch alongside the worktree. This aligns with the GitHub PR workflow where merges happen remotely, and local cleanup is the final step.

**Key changes:**

| Current Behavior | New Behavior |
|------------------|--------------|
| Removes worktree only | Removes worktree **and** branch |
| Requires path argument | Path optional (resolves from CWD) |
| `--force` only affects worktree removal | `--force` controls both worktree and branch deletion |

**Invariants:**
- `wt remove` only works on **worktree branches** — never on main/default branch or branches without worktrees
- Tmux window cleanup continues to work as-is

**Workflow alignment:**

| Command | When to use | Merge happens |
|---------|-------------|---------------|
| `wt remove` | PR merged on GitHub | On GitHub (already done) |
| `wt done` | No PR, local merge only | Locally via squash merge |

## Command Syntax

```bash
# From inside a worktree (CWD resolution):
wt remove                    # Removes worktree + branch + tmux

# From anywhere (explicit path):
wt remove /path/to/worktree  # Removes worktree + branch + tmux

# With force:
wt remove --force            # Force local deletion
wt remove --force=remote     # Force local + delete remote branch
```

**Path argument becomes optional:**

| Current | New |
|---------|-----|
| `cobra.ExactArgs(1)` | `cobra.MaximumNArgs(1)` |

**Resolution order:**

1. **Path provided** → Use it directly
2. **No path, CWD is worktree** → Resolve worktree + branch from CWD
3. **No path, CWD not worktree** → Error: "not in a worktree, provide path"

**Arguments:**

| Arg | Required? | Default | Description |
|-----|-----------|---------|-------------|
| `path` | No | CWD | Worktree path to remove |
| `--force` | No | `false` | Force deletion (see Force Levels) |

## `--force` Flag Design

**The force levels:**

| Flag | Worktree (dirty) | Branch (unmerged) | Remote Branch |
|------|------------------|-------------------|---------------|
| *(none)* | ❌ Fail | ❌ Fail | ❌ Skip |
| `--force` | ✅ Remove | ✅ Delete | ❌ Skip |
| `--force=remote` | ✅ Remove | ✅ Delete | ✅ Delete |
| `--force=all` | ✅ Remove | ✅ Delete | ✅ Delete |

**Boolean shorthand:**

`--force` and `--force=true` are equivalent to `--force` (local force only).

**Default behavior (no --force):**

```bash
wt remove
```

1. Check if worktree is dirty → if yes, error
2. Check if branch is unmerged → if yes, error
3. Remove worktree, delete branch, close tmux

**Remote branch handling:**

Only `--force=remote` or `--force=all` triggers `git push origin --delete <branch>`.

**Remote branch already deleted:**

If the remote branch is already gone (e.g., GitHub auto-deleted after PR merge):

```bash
$ wt remove --force=remote
Removing worktree...
Deleting local branch 'feat-auth'...
Warning: remote branch 'origin/feat-auth' not found (may already be deleted)
✓ Done
```

This fails gracefully with a warning rather than an error.

## Path Resolution (CWD)

**Resolution logic:**

```
┌─────────────────────────────────────┐
│         wt remove [path]            │
└─────────────────┬───────────────────┘
                  │
                  ▼
        ┌─────────────────┐
        │  Path provided? │
        └────────┬────────┘
           Yes   │   No
         ┌───────┴───────┐
         ▼               ▼
    Use path      Is CWD in worktree?
                        │
              ┌─────────┴─────────┐
              Yes                  No
              ▼                    ▼
      Resolve branch         Error: "not in a
      from worktree          worktree, provide path"
```

**Detecting CWD is a worktree:**

```go
func resolveWorktreeFromCWD(ctx context.Context, gitClient *git.Client) (*domain.Worktree, error) {
    worktrees, err := gitClient.ListWorktrees(ctx)
    if err != nil {
        return nil, err
    }

    cwd, _ := os.Getwd()

    for _, wt := range worktrees {
        // Check if CWD is inside this worktree's path
        if strings.HasPrefix(cwd, wt.Path) {
            return wt, nil
        }
    }

    return nil, errors.New("not in a worktree")
}
```

## Safety Checks

**Goal:** Prevent accidental deletion of non-worktree branches.

**Protected branches:**

| Check | Error Message |
|-------|---------------|
| CWD is main repo (not worktree) | `not in a worktree, provide path` |
| Target branch is main/default | `cannot remove default branch 'main'` |
| Branch has no worktree | `branch 'feat-orphan' has no associated worktree` |
| Detached HEAD in worktree | `cannot remove: detached HEAD` |

**Implementation order:**

```go
func (s *Service) Remove(ctx context.Context, path string, force ForceLevel) error {
    // 1. Resolve path (from arg or CWD)
    worktree, err := s.resolveWorktree(path)
    if err != nil {
        return err  // "not in a worktree" or "branch has no worktree"
    }

    // 2. Check it's not the default branch
    repoInfo, _ := s.git.GetRepoInfo(ctx)
    if worktree.Branch == repoInfo.DefaultBranch {
        return fmt.Errorf("cannot remove default branch %q", worktree.Branch)
    }

    // 3. Check for detached HEAD
    if worktree.Head == "detached" {
        return fmt.Errorf("cannot remove: detached HEAD")
    }

    // 4. Proceed with removal...
}
```

## Execution Flow

**Full removal sequence:**

```
wt remove --force=remote
         │
         ▼
┌─────────────────────────────────────┐
│ 1. Resolve worktree (path or CWD)   │
│ 2. Run safety checks                │
└─────────────────┬───────────────────┘
                  ▼
┌─────────────────────────────────────┐
│ 3. Check dirty worktree             │
│    - If dirty & no --force: ERROR   │
│    - If dirty & --force: continue   │
└─────────────────┬───────────────────┘
                  ▼
┌─────────────────────────────────────┐
│ 4. Check unmerged branch            │
│    - If unmerged & no --force: ERROR│
│    - If unmerged & --force: continue│
└─────────────────┬───────────────────┘
                  ▼
┌─────────────────────────────────────┐
│ 5. Remove worktree (git worktree)   │
│ 6. Delete local branch (git -D)     │
│ 7. Close tmux window                │
└─────────────────┬───────────────────┘
                  ▼
         ┌────────────────┐
         │ --force=remote?│
         └────────┬───────┘
             Yes  │  No
            ┌─────┴─────┐
            ▼           ▼
┌───────────────────┐   (done)
│ 8. Delete remote  │
│    (warn if gone) │
└───────────────────┘
```

**Ordering rationale:**

1. **Worktree first** — physically removes the directory
2. **Branch second** — now safe to delete (no worktree referencing it)
3. **Tmux third** — cleanup UI
4. **Remote last** — optional, may not exist

## Error Messages & Output

**Success output:**

```bash
$ wt remove
Removed worktree: /home/user/worktrees/feat-auth
Deleted branch: feat-auth
Closed tmux window: feat-auth

$ wt remove --force=remote
Removed worktree: /home/user/worktrees/feat-auth
Deleted branch: feat-auth
Deleted remote branch: origin/feat-auth
Closed tmux window: feat-auth
```

**Error messages:**

| Scenario | Message |
|----------|---------|
| Not in worktree, no path | `Error: not in a worktree. Provide a path: wt remove <path>` |
| Branch is default | `Error: cannot remove default branch "main"` |
| Detached HEAD | `Error: cannot remove: detached HEAD (no branch)` |
| Dirty worktree, no force | `Error: worktree has uncommitted changes. Use --force to remove anyway` |
| Unmerged branch, no force | `Error: branch "feat-auth" is not merged. Use --force to delete anyway` |
| Invalid force value | `Error: invalid --force value "foo". Use: true, remote, or all` |

**Warning (non-fatal):**

```bash
Warning: remote branch "origin/feat-auth" not found (may already be deleted)
```

**Dry-run output (existing `--dry-run` flag):**

```bash
$ wt remove --dry-run
Would remove worktree: /home/user/worktrees/feat-auth
Would delete branch: feat-auth
Would close tmux window: feat-auth
```

## Implementation Notes

**Files to modify:**

| File | Changes |
|------|---------|
| `internal/cli/remove.go` | Update command to accept optional path, add `--force` parsing |
| `internal/worktree/service.go` | Add `ForceLevel` type, update `Remove()` method |
| `internal/git/worktree.go` | Add `IsBranchMerged()`, `DeleteRemoteBranch()`, `RemoteBranchExists()` |

**New types:**

```go
type ForceLevel int

const (
    ForceNone ForceLevel = iota
    ForceLocal            // --force or --force=true
    ForceRemote           // --force=remote or --force=all
)
```

**Backward compatibility:**

- Existing `wt remove --force <path>` continues to work (force local only)
- Path argument remains supported for explicit usage
