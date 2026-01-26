# WT CLI Flatten Design

> **Status:** Proposed
> **Issue:** wt-new
> **Created:** 2025-01-26

## Problem

The CLI has an inconsistency where worktree commands (`add`, `list`, `remove`) are nested under a `worktree` group (`wt worktree add`), while documentation and user expectations show root-level commands (`wt add`, `wt list`, `wt remove`).

### Evidence

- `docs/architecture.md` keybindings reference: `Ctrl-A: wt add`, `Ctrl-D: wt remove`, `Ctrl-L: wt list`
- `docs/plans/2025-01-26-wt-tmux-integration.md` uses `wt add` throughout
- `docs/usage.md` shows nested `wt worktree add` (inconsistent with other docs)

## Goal

Flatten the CLI structure so all primary commands are at the root level, matching the documented API and providing a more consistent user experience.

## Current vs Target Structure

### Current
```
wt
├── worktree
│   ├── add <branch>
│   ├── list
│   └── remove <path>
├── stack
├── session
├── config
├── init
├── doctor
└── setup
```

### Target
```
wt
├── add <branch>
├── list
├── remove <path>
├── stack
├── session
├── config
├── init
├── doctor
└── setup
```

## Implementation Changes

### 1. Delete `internal/cli/worktree.go`
- Remove the entire file since `worktreeCmd` will no longer exist

### 2. Modify `internal/cli/add.go` (init function)
```go
// OLD:
func init() {
    worktreeCmd.AddCommand(NewAddCmd())
}

// NEW:
func init() {
    RegisterCommand(NewAddCmd())
}
```

### 3. Modify `internal/cli/list.go` (init function)
- Same pattern: change `worktreeCmd.AddCommand` to `RegisterCommand`

### 4. Modify `internal/cli/remove.go` (init function)
- Same pattern: change `worktreeCmd.AddCommand` to `RegisterCommand`

### 5. Modify `internal/cli/root.go`
- Remove the `Run` function that rewrites args to `"worktree list"`
- Without `Run`, Cobra will show help/usage by default when `wt` is called with no args

## File Summary

| File | Action | Lines Changed |
|------|--------|---------------|
| `internal/cli/worktree.go` | Delete | -14 lines |
| `internal/cli/add.go` | Modify init() | ~2 lines |
| `internal/cli/list.go` | Modify init() | ~2 lines |
| `internal/cli/remove.go` | Modify init() | ~2 lines |
| `internal/cli/root.go` | Remove Run function | ~-10 lines |

**Total:** ~22 lines changed across 5 files.

## Testing & Validation

### Test Updates Required

1. **Update existing tests** - Tests that mock `worktree add` need to call `add` directly
   - `internal/cli/add_test.go` - Update any command path assertions
   - `internal/cli/list_test.go` - Same
   - `internal/cli/remove_test.go` - Same

2. **Verify no-args behavior:**
   ```bash
   # Before: wt → shows worktree list
   # After: wt → shows help/usage
   wt
   # Expected: Usage message with all commands listed
   ```

3. **Verify commands work at root:**
   ```bash
   wt add test-branch        # Should work
   wt list                   # Should work
   wt remove /path/to/tree   # Should work
   ```

4. **Verify old commands don't work:**
   ```bash
   wt worktree add test      # Should error: "unknown command"
   ```

5. **Run full test suite:**
   ```bash
   make test
   make lint
   make build
   ```

### Documentation Updates

- `docs/usage.md` - Already mostly correct but verify
- `docs/architecture.md` - Update command tree diagram if present
- Any shell aliases/keybindings that reference `wt worktree *`

## Edge Cases & User Impact

### Breaking Change

Users with scripts/aliases using `wt worktree add` will need to update to `wt add`.

**Mitigation:** This is a pre-1.0 release, breaking changes are acceptable. Release notes should document the change clearly.

### Error Messages

If someone types `wt worktree add`, Cobra will show:
```
Error: unknown command "worktree" for "wt"
```

This is clear enough - users will understand the command has changed.

### Rollback

If needed, reverting is straightforward:
1. Restore `internal/cli/worktree.go`
2. Change init() functions back to `worktreeCmd.AddCommand()`
3. Restore `rootCmd.Run` in `root.go`

## Design Decisions

### Q: Should we keep backward compatibility for `wt worktree add`?

**A: No.** Full removal of the `worktree` group. This provides the cleanest API and matches the documented design. Dual registration would add maintenance overhead without clear benefit.

### Q: What should happen when someone runs `wt` with no arguments?

**A: Show help/usage.** This is standard CLI behavior. The current special-case (listing worktrees) is surprising and inconsistent.

### Q: Should we support `wt worktrees` as an alias for `wt list`?

**A: No.** Keep it simple with just `wt list`. Aliases add complexity for marginal benefit.

## Implementation Order

1. Modify `root.go` - Remove `Run` function
2. Modify `add.go`, `list.go`, `remove.go` - Change init() to use `RegisterCommand()`
3. Delete `worktree.go`
4. Update tests
5. Run full test suite
6. Update documentation

## Success Criteria

- [ ] `wt add`, `wt list`, `wt remove` work correctly
- [ ] `wt` (no args) shows help/usage
- [ ] `wt worktree add` returns "unknown command" error
- [ ] All tests pass
- [ ] Documentation updated
