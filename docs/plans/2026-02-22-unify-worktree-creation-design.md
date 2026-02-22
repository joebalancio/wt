# Design: Unify wt add and wt stack Worktree Creation

**Issue:** wt-ido
**Date:** 2026-02-22
**Status:** Approved

## Problem Statement

`wt stack` bypasses the `worktree.Service` layer, causing it to miss critical safety features and user-facing options that `wt add` provides:

| Feature | `wt add` | `wt stack` |
|---------|----------|------------|
| Worktree nesting check | ✅ | ❌ |
| Path collision detection | ✅ | ❌ |
| `--path` flag | ✅ | ❌ |
| `--track` flag | ✅ | ❌ |
| `--no-checkout` flag | ✅ | ❌ |

## Solution Overview

Make `stack.Service` delegate worktree creation to `worktree.Service` instead of calling `git.AddWorktree()` directly.

**Current Flow:**
```
wt stack → stack.Service.CreateWorktree() → git.AddWorktree()
                    ↑ (duplicates path resolution logic)
```

**Proposed Flow:**
```
wt stack → stack.Service.CreateWorktree() → worktree.Service.Add() → git.AddWorktree()
                                                    ↓
                                         (path resolution, validation, collision check)
```

## Design Decisions

### D1: Feature Scope - Full Parity
Add all three flags (`--path`, `--no-checkout`, `--track`) to `wt stack` for consistency with `wt add`.

### D2: Interactive Picker - Deferred
Keep current auto-suffix behavior (`wt stack` with no args = `currentBranch-xY7k`). Interactive picker is a separate feature (wt-13z).

### D3: Test Strategy - Stub Implementation
Use minimal stub implementation in tests rather than creating a full interface for `worktree.Service`.

## Architecture

### Service Layer Changes (internal/stack/service.go)

**Updated Struct:**
```go
type Service struct {
    git         git.GitClient
    spice       SpiceClient
    cfg         *config.Config
    worktreeSvc *worktree.Service  // NEW
}
```

**Updated Constructor:**
```go
func NewService(gitClient git.GitClient, spiceClient SpiceClient, cfg *config.Config, worktreeSvc *worktree.Service) (*Service, error) {
    if worktreeSvc == nil {
        return nil, fmt.Errorf("worktreeSvc cannot be nil")
    }
    // ... existing validation ...
}
```

**Updated BranchSpec:**
```go
type BranchSpec struct {
    Name       string  // Named suffix (e.g., "api")
    Base       string  // Base branch (defaults to current)
    Path       string  // Custom worktree path (NEW)
    Track      string  // Remote branch to track (NEW)
    NoCheckout bool    // Skip checkout (NEW)
}
```

**Refactored CreateWorktree():**
```go
func (s *Service) CreateWorktree(ctx context.Context, spec BranchSpec) (*domain.Worktree, error) {
    wtSpec := domain.WorktreeCreateSpec{
        Branch:   spec.BranchName,
        Path:     spec.Path,        // empty = auto-resolve
        Checkout: !spec.NoCheckout,
    }
    if spec.Track != "" {
        wtSpec.Track = &spec.Track
    }
    return s.worktreeSvc.Add(ctx, wtSpec)
}
```

**Removed:** `getWorktreePath()` - path resolution now handled by `worktree.Service.ResolvePath()`

### CLI Layer Changes (internal/cli/stack.go)

**New Flags:**
```go
var (
    stackBase    string
    stackForce   bool
    noSetup      bool
    path         string  // --path: custom worktree path
    track        string  // --track: remote branch to track
    noCheckout   bool    // --no-checkout: skip checkout
)
```

**Worktree Nesting Check** (at start of `runStackCommand()`):
```go
inWorktree, mainRepoRoot, err := gitClient.IsInWorktree(ctx)
if inWorktree {
    Fatal(`cannot stack from inside another worktree

Current location: %s
Main repository:  %s

Run this command from the main repository instead:
  cd %s && wt stack %s`, currentPath, mainRepoRoot, mainRepoRoot, name)
}
```

**Updated initStackService():**
```go
func initStackService(gitClient *git.Client, cfg *config.Config) (*stack.Service, *worktree.Service) {
    worktreeSvc, err := worktree.NewService(gitClient, cfg)
    if err != nil {
        Fatal("Failed to create worktree service: %v", err)
    }

    spiceClient, err := spice.NewClient(cfg)
    if err != nil {
        Fatal("Failed to create spice client: %v", err)
    }

    stackService, err := stack.NewService(gitClient, spiceClient, cfg, worktreeSvc)
    if err != nil {
        Fatal("Failed to create stack service: %v", err)
    }

    return stackService, worktreeSvc
}
```

## Testing Strategy

### Unit Tests

| File | Test | Description |
|------|------|-------------|
| `stack/service_test.go` | `TestService_CreateWorktree_UsesWorktreeService` | Verify worktree service is called |
| `stack/service_test.go` | `TestService_CreateWorktree_PassesFlags` | Verify flags are passed through |
| `cli/stack_test.go` | `TestStackCommand_NestingCheck` | Verify error from worktree |
| `cli/stack_test.go` | `TestStackCommand_Flags` | Verify flag parsing |

### Test Stub Pattern

```go
// stubWorktreeService implements minimal worktree.Service behavior for tests
type stubWorktreeService struct {
    addFunc func(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
}

func (s *stubWorktreeService) Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
    if s.addFunc != nil {
        return s.addFunc(ctx, spec)
    }
    return &domain.Worktree{Path: "/stub/path", Branch: spec.Branch}, nil
}
```

## Implementation Steps

### Phase 1: Service Layer Refactoring
1. Add `worktreeSvc` field to `Service` struct
2. Update `NewService()` to accept worktree service parameter
3. Update `BranchSpec` with new fields
4. Refactor `CreateWorktree()` to use `worktreeSvc.Add()`
5. Remove `getWorktreePath()` method
6. Create worktree service stub for tests

### Phase 2: CLI Layer Updates
1. Add new flags (`--path`, `--track`, `--no-checkout`)
2. Add worktree nesting check
3. Update `initStackService()` to create worktree service
4. Pass flag values to `BranchSpec`

### Phase 3: Verification
1. Update existing tests
2. Add new tests for flags and nesting check
3. Run `make check` (fmt + lint + test)
4. Update documentation

## Files Changed

```
internal/stack/service.go        # Service refactoring
internal/stack/service_test.go   # Updated tests
internal/cli/stack.go            # CLI flags + nesting check
CLAUDE.md                        # Documentation
```

## Acceptance Criteria

1. `wt stack` fails with helpful error when run from inside a worktree
2. `wt stack --path /custom/path api` creates worktree at specified path
3. `wt stack --no-checkout api` creates worktree without checkout
4. `wt stack --track origin/api api` tracks remote branch
5. Path collision detection works for stack commands
6. All existing tests pass
7. `make check` passes

## Out of Scope

- Interactive picker for `wt stack` (see wt-13z)
- `--run` flag (could be added separately if needed)
