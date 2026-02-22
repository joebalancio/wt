# Squash-Merge Detection for wt remove

**Issue:** wt-cm1
**Date:** 2026-02-22

## Problem

Users frequently need to use `wt remove --force` after completing the normal PR workflow (squash merge → cleanup). This is confusing and reduces trust in the tool's safety checks.

### Root Cause

The `IsBranchMerged()` function uses `git merge-base --is-ancestor <branch> <default>` which:
- Works for regular merge commits ✓
- Fails for squash merges ✗ (original commits are not ancestors of squash commit)

## Solution

Implement two-tier detection strategy:

```
wt remove <branch>
        │
        ▼
┌─────────────────────────┐
│ gh pr view <branch>     │
│ --json state            │
└───────────┬─────────────┘
            │
      ┌─────┴───────────┐
    SUCCESS           ERROR
    (PR found)      (no PR / network)
      │                 │
      ▼                 ▼
 state="MERGED"?   ┌─────────────────┐
      │            │ git cherry -v   │
      │            │ <default> <branch>
      │            └────────┬────────┘
      │                     │
      │               ┌─────┴─────┐
      │             ALL "-"     HAS "+"
      │               │           │
      └───────────────┼───────────┘
                      │
                ┌─────┴─────┐
              MERGED     NOT MERGED
                │           │
                ▼           ▼
           Allow      Require --force
           remove
```

### Detection Methods

| Method | Merge | Squash | Rebase | Notes |
|--------|-------|--------|--------|-------|
| `gh pr view` | ✓ | ✓ | ✓ | GitHub's source of truth |
| `git cherry` | ✓ | ✓ | ✓* | Patch ID comparison |
| ~~`git merge-base`~~ | ✓ | ✗ | ✗ | Removed - lowest coverage |

*Rebase: works if commits preserved, may fail if rewritten

### Why git cherry > git merge-base

- `git cherry` compares patch IDs (changes), not commit hashes
- Detects squash merges because the *changes* exist in main
- Strict: only considers merged if 100% of patches applied

## Implementation

### New Files

**`internal/git/gh.go`** - GitHub CLI client:
```go
type GhClient struct {
    ghPath string
}

func NewGhClient() (*GhClient, error)
func (c *GhClient) IsBranchPRMerged(ctx context.Context, branch string) (bool, error)
func (c *GhClient) IsAvailable() bool
```

### Modified Files

**`internal/git/worktree.go`**:
- Add `ghClient *GhClient` to `Client` struct
- Add `IsBranchCherryMerged(ctx, branch) (bool, error)` method
- Update `IsBranchMerged()` to use gh → cherry fallback

**`internal/cli/doctor.go`**:
- Add gh CLI validation (installed + authenticated)

**`internal/cli/init.go`**:
- Add gh CLI validation with warning

## Tests

Following testing pyramid:

### Unit Tests (Most)
- `internal/git/gh_test.go` - PR state JSON parsing
- `internal/git/cherry_test.go` - Cherry output parsing

### Integration Tests (Fewer)
- `internal/git/gh_integration_test.go` - Real gh CLI calls (skipped if no token)

### E2E Tests (Fewest)
- `tests/remove_squash_merge_test.go` - Full workflow tests

## Acceptance Criteria

- [ ] `wt remove` succeeds on squash-merged branches without `--force`
- [ ] `wt remove` still requires `--force` for truly unmerged branches
- [ ] `wt doctor` reports gh CLI status (installed + authenticated)
- [ ] `wt init` validates gh CLI and warns if missing
- [ ] Unit tests cover parsing logic for both gh and cherry outputs
- [ ] Integration tests pass with real gh CLI
- [ ] Backward compatible: existing `--force` behavior unchanged

## Dependencies

- `gh` CLI required (validated in `wt init` and `wt doctor`)
- No plans to support non-GitHub remotes at this time

## Follow-up

- Refactor shared validation logic between `init` and `doctor` into reusable component
