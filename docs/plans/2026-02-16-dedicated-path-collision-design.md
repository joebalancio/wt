# Dedicated Mode Path Collision Fix

**Date:** 2026-02-16
**Issue:** wt-kie
**Status:** Approved

## Problem

When `worktree.location: dedicated` (current default), worktree paths use only the branch name:

```
~/worktrees/feature/auth   # same path for any repo with this branch
```

This causes collisions when multiple repos have branches with the same name.

### Collision Scenario

```bash
# User has two repos
~/projects/repo-a (branch: feature/auth)
~/projects/repo-b (branch: feature/auth)

# Both would resolve to the SAME path:
~/worktrees/feature/auth   # COLLISION!
```

### Code Evidence

**internal/worktree/service.go:92-102:**
```go
if s.cfg.Worktree.IsDedicated() {
    dedicatedPath := s.cfg.Worktree.GetDedicatedPath()
    return filepath.Join(dedicatedPath, branch), nil  // Only branch name!
}
```

**internal/stack/service.go:152-153** has the same issue.

## Solution

Two-part fix with collision detection as a safety net:

| Change | What | Why |
|--------|------|-----|
| **1. New default** | Switch default from `dedicated` to `per-repo` | Safer out-of-the-box, no collisions possible |
| **2. Namespace dedicated** | Include repo directory name: `~/worktrees/<repo>/<branch>` | Fixes collisions for users who opt into dedicated |
| **3. Collision detection** | Error with helpful message when collision detected | Safety net for edge cases |

## Implementation Details

### 1. Change Default to Per-Repo

**File:** `internal/config/config.go`

```go
// Current behavior: dedicated is default
func (w *WorktreeConfig) IsDedicated() bool {
    return w.Location == "dedicated" || w.Location == ""  // empty = dedicated
}

// New behavior: per-repo is default
func (w *WorktreeConfig) IsDedicated() bool {
    return w.Location == "dedicated"  // empty = per-repo (not dedicated)
}
```

### 2. Namespace Dedicated Mode Paths

**Files:** `internal/worktree/service.go`, `internal/stack/service.go`

```go
if s.cfg.Worktree.IsDedicated() {
    dedicatedPath := s.cfg.Worktree.GetDedicatedPath()
    repoInfo, err := s.git.GetRepoInfo(ctx)
    if err != nil {
        return "", err
    }
    repoName := filepath.Base(repoInfo.RootPath)
    return filepath.Join(dedicatedPath, repoName, branch), nil
}
```

**Path examples:**

| Repo Path | Repo Name | Branch | Worktree Path |
|-----------|-----------|--------|---------------|
| `/home/user/work/api` | `api` | `feature/auth` | `~/worktrees/api/feature/auth` |
| `/home/user/projects/web` | `web` | `main` | `~/worktrees/web/main` |

### 3. Collision Detection

**File:** `internal/worktree/service.go`

Add collision check when resolving paths in dedicated mode:

```go
func (s *Service) checkCollision(targetPath, currentRepoRoot string) error {
    if _, err := os.Stat(targetPath); os.IsNotExist(err) {
        return nil  // Path doesn't exist, no collision
    }

    // Path exists - check if it's from the same repo
    // Read the worktree's git dir to find its origin repo
    // If different repo, return error with helpful message

    return fmt.Errorf(`path collision: %s already exists from another repo

Options:
  --path <explicit-path>  # specify a different path
  wt config set worktree.location per-repo  # use per-repo mode`, targetPath)
}
```

### Per-Repo Mode: Unchanged

Per-repo mode is not modified - it already works correctly:

```go
filepath.Join(repoInfo.RootPath, ".worktrees", branch)
// Example: /home/user/projects/my-repo/.worktrees/feature/auth
```

## Migration & Rollback

### Breaking Change?

**No** - this is a default change, not a breaking change for existing users.

| Scenario | Impact |
|----------|--------|
| User has config with `location: dedicated` | **None** - explicit config wins |
| User has no config (fresh install) | **Changed** - now defaults to per-repo |

### Existing Worktrees

Worktrees created before this change continue to work:

- `wt list` still finds them (reads from git, not our path logic)
- `wt remove` still removes them (takes explicit path)
- Only `wt add` behavior changes for new worktrees

### Rollback

If issues arise, rollback is simple:

1. Revert the default change in `IsDedicated()`
2. Users can also override via config: `wt config set worktree.location dedicated`

## Testing Strategy

### Unit Tests

| Test | Location |
|------|----------|
| `IsDedicated_defaultIsEmpty` | `internal/config/config_test.go` |
| `ResolvePath_Dedicated_addsRepoName` | `internal/worktree/service_test.go` |
| `ResolvePath_PerRepo_unchanged` | `internal/worktree/service_test.go` |
| `getWorktreePath_Dedicated_addsRepoName` | `internal/stack/service_test.go` |

### Integration Tests

| Test | File |
|------|------|
| `TestDedicatedMode_Namespacing` | `tests/dedicated_namespacing_test.go` |
| `TestDedicatedMode_CollisionDetection` | `tests/dedicated_namespacing_test.go` |
| `TestDefaultIsPerRepo` | `tests/integration_test.go` (add to existing) |

Use existing `setupTestRepo()` helper pattern.

## Files Changed

```
internal/config/config.go           # IsDedicated() logic
internal/worktree/service.go        # ResolvePath() + collision detection
internal/stack/service.go           # getWorktreePath()

internal/config/config_test.go      # new unit tests
internal/worktree/service_test.go   # new unit tests
internal/stack/service_test.go      # new unit tests

tests/dedicated_namespacing_test.go # new integration tests
tests/integration_test.go           # add default test
```

## Estimated Scope

- **~50-80 lines** of production code
- **~100-150 lines** of test code
- **Low risk** - isolated changes, backward compatible
