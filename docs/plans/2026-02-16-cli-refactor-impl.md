# CLI Package Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor internal/cli/ for consistent naming and better code organization

**Architecture:** Extract tmux helpers to dedicated file, move shared infrastructure to root.go, rename config files to remove redundant cli_ prefix

**Tech Stack:** Go 1.21+, golangci-lint, existing test patterns

---

## Task 1: Create tmux_helpers.go

**Files:**
- Create: `internal/cli/tmux_helpers.go`

**Step 1: Create the new tmux_helpers.go file**

```go
package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/stack"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/spf13/cobra"
)

// isInTmux checks if currently running in tmux
func isInTmux() bool {
	return tmux.IsInTmux()
}

// shouldCreateTmuxWindow determines if tmux window should be created
func shouldCreateTmuxWindow(noTmuxFlag bool) bool {
	if !isInTmux() {
		return false
	}
	if noTmuxFlag {
		return false
	}
	return true
}

// createTmuxWindowForWorktree creates a tmux window for the worktree if conditions are met
func createTmuxWindowForWorktree(cmd *cobra.Command, branch, worktreePath string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	windowName := tmux.GenerateWindowName(branch)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
	}
}

// createStackTmuxWindow creates a tmux window for the stack branch
func createStackTmuxWindow(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName, worktreePath string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	// Get stack level for window naming
	stackLevel := getStackLevel(ctx, stackService, branchName)

	windowName := tmux.GenerateStackWindowName(branchName, stackLevel)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
	}
}

// getStackLevel returns the stack level for a given branch name
func getStackLevel(ctx context.Context, stackService *stack.Service, branchName string) int {
	stackBranches, _ := stackService.GetStack(ctx)
	for i, sb := range stackBranches {
		if sb.Name == branchName {
			return i
		}
	}
	return 0
}
```

**Step 2: Verify file compiles**

Run: `go build ./internal/cli/...`
Expected: No errors

---

## Task 2: Update root.go - Add shared infrastructure functions

**Files:**
- Modify: `internal/cli/root.go`

**Step 1: Update imports in root.go**

Change imports from:
```go
import (
	"fmt"
	"os"

	"github.com/joebalancio/wt/internal/tmux"
	"github.com/spf13/cobra"
)
```

To:
```go
import (
	"context"
	"fmt"
	"os"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/pkg/executor"
	"github.com/spf13/cobra"
)
```

**Step 2: Remove tmux functions from root.go**

Delete these functions (they're now in tmux_helpers.go):
```go
// isInTmux checks if currently running in tmux
func isInTmux() bool {
	return tmux.IsInTmux()
}

// shouldCreateTmuxWindow determines if tmux window should be created
func shouldCreateTmuxWindow(noTmuxFlag bool) bool {
	if !isInTmux() {
		return false
	}
	if noTmuxFlag {
		return false
	}
	return true
}
```

**Step 3: Add loadConfigForCommand to root.go**

Add after `Warn()` function:
```go
// loadConfigForCommand loads config from flags, returning defaults if not found
func loadConfigForCommand() (*config.Config, error) {
	// Check for --config flag
	customPath, _ := rootCmd.PersistentFlags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		// No config found - return defaults
		return config.DefaultConfig(), nil
	}

	return config.LoadMerged(projectPath, globalPath)
}
```

**Step 4: Add runSetupHooks to root.go**

Add after `loadConfigForCommand()`:
```go
// runSetupHooks executes post-create hooks for a worktree
func runSetupHooks(ctx context.Context, worktreePath string) error {
	cfg, err := loadConfigForCommand()
	if err != nil {
		return err
	}

	runner := executor.NewHookRunner(worktreePath)
	return runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate)
}
```

**Step 5: Verify file compiles**

Run: `go build ./internal/cli/...`
Expected: No errors

---

## Task 3: Update add.go - Remove moved functions

**Files:**
- Modify: `internal/cli/add.go`

**Step 1: Update imports in add.go**

Change imports from:
```go
import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/joebalancio/wt/pkg/executor"
	"github.com/spf13/cobra"
)
```

To:
```go
import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/spf13/cobra"
)
```

**Step 2: Remove createTmuxWindowForWorktree function**

Delete lines 96-111:
```go
// createTmuxWindowForWorktree creates a tmux window for the worktree if conditions are met
func createTmuxWindowForWorktree(cmd *cobra.Command, branch, worktreePath string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	windowName := tmux.GenerateWindowName(branch)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
	}
}
```

**Step 3: Remove runSetupHooks function**

Delete lines 117-126:
```go
// runSetupHooks executes post-create hooks for a worktree
func runSetupHooks(ctx context.Context, worktreePath string) error {
	cfg, err := loadConfigForCommand()
	if err != nil {
		return err
	}

	runner := executor.NewHookRunner(worktreePath)
	return runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate)
}
```

**Step 4: Verify file compiles**

Run: `go build ./internal/cli/...`
Expected: No errors

---

## Task 4: Update stack.go - Remove moved functions

**Files:**
- Modify: `internal/cli/stack.go`

**Step 1: Remove createStackTmuxWindow function**

Delete lines 141-159:
```go
// createStackTmuxWindow creates a tmux window for the stack branch
func createStackTmuxWindow(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName, worktreePath string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	// Get stack level for window naming
	stackLevel := getStackLevel(ctx, stackService, branchName)

	windowName := tmux.GenerateStackWindowName(branchName, stackLevel)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
	}
}
```

**Step 2: Remove getStackLevel function**

Delete lines 161-170:
```go
// getStackLevel returns the stack level for a given branch name
func getStackLevel(ctx context.Context, stackService *stack.Service, branchName string) int {
	stackBranches, _ := stackService.GetStack(ctx)
	for i, sb := range stackBranches {
		if sb.Name == branchName {
			return i
		}
	}
	return 0
}
```

**Step 3: Remove loadConfigForCommand function**

Delete lines 248-259:
```go
func loadConfigForCommand() (*config.Config, error) {
	// Check for --config flag
	customPath, _ := rootCmd.PersistentFlags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		// No config found - return defaults
		return config.DefaultConfig(), nil
	}

	return config.LoadMerged(projectPath, globalPath)
}
```

**Step 4: Verify file compiles**

Run: `go build ./internal/cli/...`
Expected: No errors

---

## Task 5: Rename config files - Remove cli_ prefix

**Files:**
- Rename: `internal/cli/cli_config_get.go` → `internal/cli/config_get.go`
- Rename: `internal/cli/cli_config_set.go` → `internal/cli/config_set.go`
- Rename: `internal/cli/cli_config_unset.go` → `internal/cli/config_unset.go`
- Rename: `internal/cli/cli_config_list.go` → `internal/cli/config_list.go`
- Rename: `internal/cli/cli_config_validate.go` → `internal/cli/config_validate.go`
- Rename: `internal/cli/cli_config_parser.go` → `internal/cli/config_parser.go`

**Step 1: Rename source files**

```bash
cd internal/cli
git mv cli_config_get.go config_get.go
git mv cli_config_set.go config_set.go
git mv cli_config_unset.go config_unset.go
git mv cli_config_list.go config_list.go
git mv cli_config_validate.go config_validate.go
git mv cli_config_parser.go config_parser.go
```

**Step 2: Verify build succeeds**

Run: `go build ./...`
Expected: No errors

---

## Task 6: Rename config test files

**Files:**
- Rename: `internal/cli/cli_config_get_test.go` → `internal/cli/config_get_test.go`
- Rename: `internal/cli/cli_config_set_test.go` → `internal/cli/config_set_test.go`
- Rename: `internal/cli/cli_config_unset_test.go` → `internal/cli/config_unset_test.go`
- Rename: `internal/cli/cli_config_list_test.go` → `internal/cli/config_list_test.go`
- Rename: `internal/cli/cli_config_parser_test.go` → `internal/cli/config_parser_test.go`
- Rename: `internal/cli/cli_config_integration_test.go` → `internal/cli/config_integration_test.go`

**Step 1: Rename test files**

```bash
cd internal/cli
git mv cli_config_get_test.go config_get_test.go
git mv cli_config_set_test.go config_set_test.go
git mv cli_config_unset_test.go config_unset_test.go
git mv cli_config_list_test.go config_list_test.go
git mv cli_config_parser_test.go config_parser_test.go
git mv cli_config_integration_test.go config_integration_test.go
```

**Step 2: Verify tests still pass**

Run: `go test ./internal/cli/... -v`
Expected: All tests pass

---

## Task 7: Run full test suite and lint

**Step 1: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 2: Run linter**

Run: `make lint`
Expected: No errors

**Step 3: Run full check**

Run: `make check`
Expected: All checks pass (fmt + lint + test)

---

## Task 8: Commit the refactor

**Step 1: Review changes**

Run: `git status`
Expected: All renamed and modified files staged

**Step 2: Commit**

```bash
git add internal/cli/
git commit -m "$(cat <<'EOF'
refactor(cli): reorganize for consistency and better separation of concerns

- Remove redundant cli_ prefix from config files
- Create tmux_helpers.go for tmux-related functions
- Move shared infrastructure (loadConfigForCommand, runSetupHooks) to root.go
- Slim down add.go and stack.go by removing moved functions

File changes:
- cli_config_*.go → config_*.go (consistent naming)
- New: tmux_helpers.go (isInTmux, shouldCreateTmuxWindow, etc.)
- root.go: added loadConfigForCommand, runSetupHooks
- add.go: removed moved functions
- stack.go: removed moved functions

Closes: wt-jio

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

| Task | Description | Risk |
|------|-------------|------|
| 1 | Create tmux_helpers.go | Low - new file only |
| 2 | Update root.go with shared functions | Medium - modifies core file |
| 3 | Update add.go - remove moved functions | Low - just deletions |
| 4 | Update stack.go - remove moved functions | Low - just deletions |
| 5 | Rename config source files | Low - git mv preserves history |
| 6 | Rename config test files | Low - git mv preserves history |
| 7 | Run tests and lint | Verification step |
| 8 | Commit | Final step |

**Total estimated time:** 20-30 minutes
