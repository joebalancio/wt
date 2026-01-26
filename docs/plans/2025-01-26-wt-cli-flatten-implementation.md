# WT CLI Flatten Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Flatten CLI structure by moving worktree commands (`add`, `list`, `remove`) from nested `wt worktree *` to root level `wt *`.

**Architecture:** Remove the `worktreeCmd` group entirely and register commands directly to root. Delete `worktree.go`, modify init() functions in `add.go`, `list.go`, `remove.go`, and remove special-case `Run` function in `root.go`.

**Tech Stack:**
- `github.com/spf13/cobra` - CLI command framework
- Go standard testing - `testing` package
- `make` - Build automation

---

## Task 1: Remove Special-Case `Run` Function from root.go

**Files:**
- Modify: `internal/cli/root.go:30-42`

**Step 1: Read current root.go to understand the Run function**

```bash
cat internal/cli/root.go
```

Expected: See `rootCmd.Run` function that rewrites args to `["worktree", "list"]`

**Step 2: Remove the Run function entirely**

Delete these lines from `internal/cli/root.go`:
```go
// Make `wt` (no args) equivalent to `wt worktree list`
rootCmd.Run = func(cmd *cobra.Command, _ []string) {
    cmd.SetArgs([]string{"worktree", "list"})
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

Also remove unused `os` import if no other usage.

**Step 3: Verify no-args behavior shows help**

```bash
make build
./bin/wt
```

Expected: Usage/help message showing all available commands (add, list, remove, stack, etc.)

**Step 4: Commit**

```bash
git add internal/cli/root.go
git commit -m "refactor(cli): remove special-case no-args behavior

Running 'wt' with no arguments now shows help/usage instead of
listing worktrees. This is standard CLI behavior.

Part of flattening worktree commands to root level.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Change add.go init() to Register at Root

**Files:**
- Modify: `internal/cli/add.go:92-95`

**Step 1: Read the init function in add.go**

```bash
tail -10 internal/cli/add.go
```

Expected: See `worktreeCmd.AddCommand(NewAddCmd())`

**Step 2: Modify init() to use RegisterCommand**

Replace the init function:
```go
// OLD:
func init() {
    // Register as a child of worktreeCmd
    worktreeCmd.AddCommand(NewAddCmd())
}

// NEW:
func init() {
    RegisterCommand(NewAddCmd())
}
```

**Step 3: Verify command is accessible at root**

```bash
make build
./bin/wt add --help
```

Expected: Help for `wt add` command showing flags like `--base`, `--path`, `--force`

**Step 4: Commit**

```bash
git add internal/cli/add.go
git commit -m "refactor(cli): register add command at root level

Change 'wt worktree add' to 'wt add'. Part of flattening CLI
structure to match documented design.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Change list.go init() to Register at Root

**Files:**
- Modify: `internal/cli/list.go:72-74`

**Step 1: Read the init function in list.go**

```bash
tail -5 internal/cli/list.go
```

Expected: See `worktreeCmd.AddCommand(NewListCmd())`

**Step 2: Modify init() to use RegisterCommand**

Replace the init function:
```go
// OLD:
func init() {
    worktreeCmd.AddCommand(NewListCmd())
}

// NEW:
func init() {
    RegisterCommand(NewListCmd())
}
```

**Step 3: Verify command is accessible at root**

```bash
make build
./bin/wt list --help
```

Expected: Help for `wt list` command showing flags like `--branches`, `--path`

**Step 4: Commit**

```bash
git add internal/cli/list.go
git commit -m "refactor(cli): register list command at root level

Change 'wt worktree list' to 'wt list'. Part of flattening CLI
structure to match documented design.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Change remove.go init() to Register at Root

**Files:**
- Modify: `internal/cli/remove.go:55-58`

**Step 1: Read the init function in remove.go**

```bash
tail -8 internal/cli/remove.go
```

Expected: See `worktreeCmd.AddCommand(NewRemoveCmd())`

**Step 2: Modify init() to use RegisterCommand**

Replace the init function:
```go
// OLD:
func init() {
    // Register as a child of worktreeCmd
    worktreeCmd.AddCommand(NewRemoveCmd())
}

// NEW:
func init() {
    RegisterCommand(NewRemoveCmd())
}
```

**Step 3: Verify command is accessible at root**

```bash
make build
./bin/wt remove --help
```

Expected: Help for `wt remove` command showing `--force` flag

**Step 4: Commit**

```bash
git add internal/cli/remove.go
git commit -m "refactor(cli): register remove command at root level

Change 'wt worktree remove' to 'wt remove'. Part of flattening CLI
structure to match documented design.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Delete worktree.go

**Files:**
- Delete: `internal/cli/worktree.go`

**Step 1: Verify no remaining references to worktreeCmd**

```bash
grep -r "worktreeCmd" internal/cli/*.go
```

Expected: No results (all references should be gone after Tasks 2-4)

**Step 2: Delete the file**

```bash
rm internal/cli/worktree.go
```

**Step 3: Verify build still works**

```bash
make build
```

Expected: Clean build with no errors

**Step 4: Verify old command path doesn't work**

```bash
./bin/wt worktree add test-branch 2>&1
```

Expected: `Error: unknown command "worktree" for "wt"`

**Step 5: Commit**

```bash
git rm internal/cli/worktree.go
git commit -m "refactor(cli): remove worktree command group

Delete worktree.go now that all subcommands are registered
at root level. 'wt worktree *' commands are no longer available.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Update add_test.go for New Command Path

**Files:**
- Modify: `internal/cli/add_test.go`

**Step 1: Read existing test file**

```bash
cat internal/cli/add_test.go
```

**Step 2: Look for any hardcoded command path references**

```bash
grep -n "worktree" internal/cli/add_test.go
```

Expected: May have no matches (tests likely use command directly) or may need updating

**Step 3: Update any command path assertions found**

If tests reference `"worktree", "add"` as command args, change to just `"add"`.

**Step 4: Run tests**

```bash
go test -v ./internal/cli/... -run TestAdd
```

Expected: All add tests pass

**Step 5: Commit**

```bash
git add internal/cli/add_test.go
git commit -m "test(cli): update add tests for root-level command

Update test assertions to use 'add' instead of 'worktree add'.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Update list_test.go for New Command Path

**Files:**
- Modify: `internal/cli/list_test.go`

**Step 1: Read existing test file**

```bash
cat internal/cli/list_test.go
```

**Step 2: Look for any hardcoded command path references**

```bash
grep -n "worktree" internal/cli/list_test.go
```

**Step 3: Update any command path assertions found**

If tests reference `"worktree", "list"` as command args, change to just `"list"`.

**Step 4: Run tests**

```bash
go test -v ./internal/cli/... -run TestList
```

Expected: All list tests pass

**Step 5: Commit**

```bash
git add internal/cli/list_test.go
git commit -m "test(cli): update list tests for root-level command

Update test assertions to use 'list' instead of 'worktree list'.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Update remove_test.go for New Command Path

**Files:**
- Modify: `internal/cli/remove_test.go`

**Step 1: Read existing test file**

```bash
cat internal/cli/remove_test.go
```

**Step 2: Look for any hardcoded command path references**

```bash
grep -n "worktree" internal/cli/remove_test.go
```

**Step 3: Update any command path assertions found**

If tests reference `"worktree", "remove"` as command args, change to just `"remove"`.

**Step 4: Run tests**

```bash
go test -v ./internal/cli/... -run TestRemove
```

Expected: All remove tests pass

**Step 5: Commit**

```bash
git add internal/cli/remove_test.go
git commit -m "test(cli): update remove tests for root-level command

Update test assertions to use 'remove' instead of 'worktree remove'.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Run Full Test Suite

**Files:**
- All files

**Step 1: Run all tests**

```bash
make test
```

Expected: All tests pass

**Step 2: Run linter**

```bash
make lint
```

Expected: No lint errors

**Step 3: Build binary**

```bash
make build
```

Expected: Clean build at `bin/wt`

**Step 4: Manual smoke test**

```bash
# Verify all three commands work
./bin/wt --help
./bin/wt add --help
./bin/wt list --help
./bin/wt remove --help

# Verify old command path errors
./bin/wt worktree add test 2>&1 | grep "unknown command"
```

Expected: All help texts show, old path errors

**Step 5: Commit**

```bash
git add .
git commit -m "test(cli): verify full test suite after CLI flatten

All tests pass after moving worktree commands to root level.
Manual smoke test confirms new command paths work and old
paths error appropriately.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: Update docs/usage.md

**Files:**
- Modify: `docs/usage.md`

**Step 1: Find all occurrences of nested command syntax**

```bash
grep -n "worktree" docs/usage.md
```

**Step 2: Replace nested syntax with root-level**

Find patterns like:
- `wt worktree add` → `wt add`
- `wt worktree list` → `wt list`
- `wt worktree remove` → `wt remove`

**Step 3: Update Table of Contents if needed**

Change:
```markdown
## Worktree Commands
### wt worktree add
```

To:
```markdown
## Worktree Commands
### wt add
```

**Step 4: Verify documentation is consistent**

```bash
grep -c "wt worktree add" docs/usage.md
# Should return 0
grep -c "wt add" docs/usage.md
# Should return > 0
```

**Step 5: Commit**

```bash
git add docs/usage.md
git commit -m "docs(usage): update command syntax to root-level

Replace 'wt worktree add/list/remove' with 'wt add/list/remove'
throughout usage documentation to match new CLI structure.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 11: Update docs/architecture.md Command Tree

**Files:**
- Modify: `docs/architecture.md`

**Step 1: Find command tree diagram**

```bash
grep -A 20 "Command Tree\\|command tree\\|wt --" docs/architecture.md | head -30
```

**Step 2: Update any nested worktree references**

If there's a diagram showing:
```
wt
├── worktree
│   ├── add
```

Change to:
```
wt
├── add
```

**Step 3: Verify keybindings section is still correct**

Keybindings should already reference `wt add`, `wt list`, `wt remove` (this was the source of truth).

**Step 4: Commit**

```bash
git add docs/architecture.md
git commit -m "docs(architecture): update command tree structure

Update CLI structure diagram to reflect flattened commands.
Keybindings section already correct (wt add/list/remove).

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 12: Final Verification and Bead Cleanup

**Files:**
- Issue tracking

**Step 1: Run final full check**

```bash
make check
```

Expected: All checks pass (fmt, lint, test)

**Step 2: Build and verify installed binary**

```bash
make build
make install
wt --help
```

Expected: Shows help with root-level commands

**Step 3: Close the implementation bead**

```bash
bd close wt-new --reason="CLI flatten complete. All worktree commands now at root level (wt add/list/remove). Tests pass, docs updated."
```

**Step 4: Sync beads**

```bash
bd sync --flush-only
```

**Step 5: Final commit**

```bash
git add .beads/
git commit -m "chore: close wt-new bead after CLI flatten implementation

All worktree commands moved to root level:
- wt add (was wt worktree add)
- wt list (was wt worktree list)
- wt remove (was wt worktree remove)

Breaking change documented in release notes.

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Success Criteria

- [ ] `wt add`, `wt list`, `wt remove` work correctly at root level
- [ ] `wt` (no args) shows help/usage, not worktree list
- [ ] `wt worktree add` returns "unknown command" error
- [ ] All tests pass (`make test`)
- [ ] No lint errors (`make lint`)
- [ ] Documentation updated (`docs/usage.md`, `docs/architecture.md`)
- [ ] Bead closed and synced

## Rollback Plan

If issues arise, revert commits in reverse order:
1. Restore `worktree.go` from git history
2. Revert init() changes in `add.go`, `list.go`, `remove.go`
3. Restore `rootCmd.Run` in `root.go`
4. Revert documentation changes

```bash
git revert HEAD~12..HEAD
```
