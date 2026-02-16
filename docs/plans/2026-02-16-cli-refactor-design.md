# CLI Package Refactor Design

**Date:** 2026-02-16
**Issue:** wt-jio
**Status:** Approved

## Problem

The `internal/cli/` package has inconsistent naming conventions and mixed concerns:

1. **Inconsistent file naming:** Config subcommand files use `cli_` prefix (`cli_config_get.go`) while other commands don't (`add.go`, `list.go`)
2. **Mixed concerns in `root.go`:** Contains both core CLI infrastructure and tmux helper logic
3. **Cross-file dependencies:** `stack.go` calls `runSetupHooks()` defined in `add.go`
4. **Scattered shared logic:** `loadConfigForCommand()` defined in `stack.go` but used by multiple commands

## Goals

- **Code organization:** Each file has a single, clear purpose
- **Consistency:** Follow existing naming conventions throughout
- **Reusability:** Shared infrastructure in logical locations
- **Testability:** Clear boundaries between components

## Design Decisions

### 1. Remove `cli_` Prefix from Config Files

The `cli_` prefix is redundant since we're already in `internal/cli/`. Rename all config files to match the convention used by other commands.

| Current | New |
|---------|-----|
| `cli_config_get.go` | `config_get.go` |
| `cli_config_set.go` | `config_set.go` |
| `cli_config_unset.go` | `config_unset.go` |
| `cli_config_list.go` | `config_list.go` |
| `cli_config_validate.go` | `config_validate.go` |
| `cli_config_parser.go` | `config_parser.go` |
| `cli_config_*_test.go` | `config_*_test.go` |

**Rationale:** Consistency with `add.go`, `list.go`, `remove.go`, etc.

### 2. Create `tmux_helpers.go`

Extract tmux-related helper functions into a dedicated file.

**Functions to extract:**
- `isInTmux()` - from `root.go`
- `shouldCreateTmuxWindow()` - from `root.go`
- `createTmuxWindowForWorktree()` - from `add.go`
- `createStackTmuxWindow()` - from `stack.go`
- `getStackLevel()` - from `stack.go`

**Rationale:** Groups related tmux functionality together, reduces clutter in `root.go` and command files.

### 3. Expand `root.go` as Shared Infrastructure Home

Move shared CLI infrastructure functions to `root.go`.

**Functions to add:**
- `loadConfigForCommand()` - from `stack.go`
- `runSetupHooks()` - from `add.go`

**Functions that stay:**
- `Execute()`, `RegisterCommand()` - command registration
- `Verbose()`, `NoTmux()`, `GetDryRun()` - flag accessors
- `Fatal()`, `Warn()` - output helpers

**Rationale:** `root.go` is the natural home for shared CLI infrastructure. Following Go's philosophy of keeping helpers close to usage, these are used across multiple commands and belong at the package root.

### 4. Slim Down Command Files

After extractions, command files become more focused:

**`add.go`:**
- Keep: `NewAddCmd()`, `runAddCommand()`
- Remove: `runSetupHooks()`, `createTmuxWindowForWorktree()`

**`stack.go`:**
- Keep: `NewStackCmd()`, `runStackCommand()`, `initStackService()`, `isProtectedBranch()`, `getCurrentBranchProtected()`, `validateGitSpiceConfig()`, `NewStackListCmd()`
- Remove: `loadConfigForCommand()`, `createStackTmuxWindow()`, `getStackLevel()`

## Final File Structure

```
internal/cli/
├── root.go              # Core CLI infrastructure + shared helpers
├── tmux_helpers.go      # NEW: Tmux window management helpers
│
├── add.go               # wt add command
├── list.go              # wt list command
├── remove.go            # wt remove command
├── done.go              # wt done command
├── stack.go             # wt stack command
├── doctor.go            # wt doctor command
├── init.go              # wt init command
├── setup.go             # wt setup command
├── session.go           # wt session command
│
├── config.go            # wt config parent command
├── config_get.go        # wt config get
├── config_set.go        # wt config set
├── config_unset.go      # wt config unset
├── config_list.go       # wt config list
├── config_validate.go   # wt config validate
├── config_parser.go     # Config parsing logic
│
└── *_test.go            # Test files (same renames)
```

## Implementation Steps

1. Create `tmux_helpers.go` with extracted functions
2. Move `loadConfigForCommand()` and `runSetupHooks()` to `root.go`
3. Update `add.go` to remove moved functions
4. Update `stack.go` to remove moved functions
5. Rename all `cli_config_*.go` files to `config_*.go`
6. Rename all test files accordingly
7. Run tests to verify no breakage
8. Run linter to ensure code quality

## Notes

- `config_parser.go` remains 425 lines - acceptable for Go where cohesive files of this size are normal
- No changes to `internal/config/` package - this refactor is scoped to `internal/cli/` only
- Go allows same-package files to call each other's functions without imports
