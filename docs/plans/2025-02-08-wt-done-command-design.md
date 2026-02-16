# wt done Command Design

**Date:** 2025-02-08
**Status:** Design Validated
**Scope:** Feature completion workflow automation

## Overview

`wt done <branch>` automates the cleanup workflow after feature work is complete. It performs three operations in sequence: squash merge the feature branch into the current branch, remove the associated worktree, and delete the feature branch.

## Motivation

- **Workflow efficiency**: The manual sequence (git merge → wt remove → git branch -d) is repetitive and error-prone
- **Atomic operation**: All cleanup steps in one command reduces chance of leaving orphaned worktrees
- **Hook integration**: Enables automation (tests, notifications) at the exact moment of feature completion

## Command Interface

```bash
# Basic usage
wt done feat/config

# With flags
wt done feat/api --dry-run    # Preview without changes
wt done feat/auth --force     # Skip dirty worktree checks
```

**Arguments:**
- `<branch>` - Feature branch to squash merge into current branch

**Flags:**
- `--force` - Proceed even if worktree has uncommitted changes
- `--dry-run` - Show what would happen without making changes

## Execution Flow

```
┌─────────────────────────────────────────────────────────┐
│ 1. Squash Merge                                         │
│    git merge --squash <branch>                          │
│    git commit -m "feat(...): ..."                       │
└────────────────────┬────────────────────────────────────┘
                     │
                     v
┌─────────────────────────────────────────────────────────┐
│ 2. OnWorktreeDone Hooks (worktree exists)               │
│    - Run tests against merged state                     │
│    - Scripts requiring file access                      │
│    - Pre-cleanup validation                             │
└────────────────────┬────────────────────────────────────┘
                     │
                     v
┌─────────────────────────────────────────────────────────┐
│ 3. Remove Worktree                                      │
│    wt remove <worktree_path>                            │
│    (includes tmux window cleanup)                       │
└────────────────────┬────────────────────────────────────┘
                     │
                     v
┌─────────────────────────────────────────────────────────┐
│ 4. Delete Branch                                        │
│    git branch -D <branch>                               │
└────────────────────┬────────────────────────────────────┘
                     │
                     v
┌─────────────────────────────────────────────────────────┐
│ 5. OnWorktreeRemove Hooks (worktree gone)               │
│    - Team notifications (Slack, email)                  │
│    - Issue tracker updates (close ticket)               │
│    - Deployment triggers                                │
└─────────────────────────────────────────────────────────┘
```

## Hook Configuration

```yaml
hooks:
  # Runs after merge, before cleanup (worktree exists)
  on_worktree_done:
    - run: pytest tests/
      cwd: "{worktree_path}"
    - run: npm run lint
      cwd: "{worktree_path}"
    - run: make verify
      cwd: "{worktree_path}"

  # Runs after everything is complete (worktree gone)
  on_worktree_remove:
    - run: gh issue close $ISSUE_ID --comment "Merged via wt done"
    - run: slack-notify "Merged {branch} to main"
    - run: trigger-deployment {branch}
```

**Hook Template Variables:**
- `{worktree_path}` - Absolute path to worktree (OnWorktreeDone only)
- `{branch}` - Feature branch name

## Implementation Structure

```
internal/cli/
└── done.go                    # Cobra command definition

internal/worktree/
└── service.go                 # Add Done() method
    ├── Done(ctx, sourceBranch, force) error
    ├── squashMerge()          # Git merge operations
    ├── runDoneHooks()         # OnWorktreeDone hooks
    └── runRemoveHooks()       # OnWorktreeRemove hooks

internal/config/
└── config.go                  # Add OnWorktreeDone to HooksConfig

pkg/executor/
└── hook_runner.go             # Reuse existing hook runner
```

## Service Layer Signature

```go
// Done merges, removes, and cleans up a feature branch
func (s *Service) Done(ctx context.Context, sourceBranch string, force bool) error {
    // 1. Squash merge into current branch
    if err := s.squashMerge(ctx, sourceBranch); err != nil {
        return fmt.Errorf("squash merge: %w", err)
    }

    // 2. Get worktree path before cleanup
    worktreePath, err := s.resolveWorktreePath(ctx, sourceBranch)
    if err != nil {
        return fmt.Errorf("resolve worktree: %w", err)
    }

    // 3. Run OnWorktreeDone hooks (worktree exists)
    if err := s.runDoneHooks(ctx, worktreePath); err != nil {
        return fmt.Errorf("done hooks: %w", err)
    }

    // 4. Remove worktree
    if err := s.Remove(ctx, worktreePath, force); err != nil {
        return fmt.Errorf("remove worktree: %w", err)
    }

    // 5. Delete branch
    if err := s.gitClient.DeleteBranch(ctx, sourceBranch, true); err != nil {
        return fmt.Errorf("delete branch: %w", err)
    }

    // 6. Run OnWorktreeRemove hooks (cleanup complete)
    if err := s.runRemoveHooks(ctx, sourceBranch); err != nil {
        return fmt.Errorf("remove hooks: %w", err)
    }

    return nil
}
```

## Error Handling

- **Atomic failure**: Errors stop execution immediately (no rollback)
- **Hook failures**: Logged as warnings but don't block cleanup
- **Merge conflicts**: Fatal error - user must resolve manually
- **Dirty worktree**: Fatal error unless `--force` is set

## Configuration Changes

```go
// HooksConfig defines hook configurations
type HooksConfig struct {
    OnWorktreeCreate []Hook `yaml:"on_worktree_create"`
    OnWorktreeDone   []Hook `yaml:"on_worktree_done,omitempty"`      // NEW
    OnWorktreeRemove []Hook `yaml:"on_worktree_remove,omitempty"`
}
```

## Testing Strategy

**Unit Tests:**
- `done_test.go` - Mock git client, verify call sequence
- Hook runner tests with different config scenarios

**Integration Tests:**
- `tests/done_integration_test.go` - Real git operations in temp repo
- Verify: merge, hooks, worktree removal, branch deletion

**Test Cases:**
- Successful workflow (clean worktree)
- Dirty worktree (fails without --force)
- Merge conflict (fails, leaves branch intact)
- Hook failures (logged but continue)
- Dry-run mode (no changes made)

## Alignment with Existing Patterns

**Command Pattern:** Follows `add.go` / `remove.go` structure
- Cobra command with exact args
- Service layer orchestration
- Simple output messages

**Hook Symmetry:**
- `OnWorktreeCreate` → worktree appears
- `OnWorktreeDone` → integration complete, files accessible
- `OnWorktreeRemove` → worktree gone, notifications

**Tmux Integration:** Reuses `remove.go` window cleanup logic

## Future Considerations

**Potential Enhancements:**
- `--target` flag to specify merge target (vs. current branch)
- `--keep-branch` flag to skip branch deletion
- Interactive mode with confirmation prompts
- Undo/rollback capability

**Not in Scope:**
- Auto-detection of parent branch
- Pull request creation
- Remote branch operations
