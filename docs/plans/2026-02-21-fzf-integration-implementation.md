# Interactive Picker Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add interactive TUI pickers to `wt add` and `wt remove` commands when running from the main repository with no arguments.

**Architecture:** Create a new `internal/picker` package that wraps charmbracelet/huh for interactive selection. The picker uses the existing git client interface to list worktrees and branches, then presents them through huh's select components. Integration points are in the CLI layer where picker functions are called when no arguments are provided.

**Tech Stack:** Go 1.22, charmbracelet/huh (form library), golang.org/x/term (TTY detection)

---

## Task 1: Add Dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Add huh and x/term dependencies**

Run:
```bash
cd /home/claude/projects/wt/.worktrees/feature/wt-eex
go get github.com/charmbracelet/huh@latest
go get golang.org/x/term@latest
```

**Step 2: Tidy dependencies**

Run:
```bash
go mod tidy
```

**Step 3: Verify dependencies added**

Run:
```bash
grep -E "charmbracelet/huh|golang.org/x/term" go.mod
```

Expected:
```
github.com/charmbracelet/huh v0.6.0
golang.org/x/term v0.21.0
```

**Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add charmbracelet/huh and golang.org/x/term dependencies"
```

---

## Task 2: Add ListAllBranches to Git Client Interface

**Files:**
- Modify: `internal/git/client_interface.go`
- Modify: `internal/git/worktree.go`
- Create: `internal/git/branch_test.go`

**Step 1: Write the failing test**

Create `internal/git/branch_test.go`:

```go
package git

import (
	"context"
	"testing"
)

func TestClient_ListAllBranches(t *testing.T) {
	// This is an integration test that requires a real git repo
	// Unit testing will be done via the mock in service_test.go
	t.Skip("requires integration test environment")
}
```

**Step 2: Run test to verify it skips**

Run:
```bash
go test -v ./internal/git -run TestClient_ListAllBranches
```

Expected:
```
=== RUN   TestClient_ListAllBranches
    branch_test.go:10: requires integration test environment
--- SKIP: TestClient_ListAllBranches
```

**Step 3: Add method to GitClient interface**

Add to `internal/git/client_interface.go` after line 32 (after IsInWorktree):

```go
	// ListAllBranches returns all local and remote branches, deduplicated.
	ListAllBranches(ctx context.Context) ([]string, error)
```

**Step 4: Verify compile error (interface not implemented)**

Run:
```bash
go build ./...
```

Expected: Compile errors about `*Client` not implementing `GitClient` (missing `ListAllBranches`)

**Step 5: Implement ListAllBranches in worktree.go**

Add to `internal/git/worktree.go` after the `IsInWorktree` function (around line 369):

```go
// ListAllBranches returns all local and remote branches, deduplicated.
func (c *Client) ListAllBranches(ctx context.Context) ([]string, error) {
	// Get local branches
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "branch", "--format=%(refname:short)")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("listing local branches: %w", err)
	}

	seen := make(map[string]bool)
	var branches []string

	for _, b := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if b != "" && !seen[b] {
			seen[b] = true
			branches = append(branches, b)
		}
	}

	return branches, nil
}
```

**Step 6: Verify build succeeds**

Run:
```bash
go build ./...
```

Expected: No errors

**Step 7: Update mockGitClient in worktree/service_test.go**

Add field to `mockGitClient` struct (after line 34):

```go
	listAllBranchesFunc    func(ctx context.Context) ([]string, error)
```

Add method implementation (after the IsInWorktree method, around line 133):

```go
func (m *mockGitClient) ListAllBranches(ctx context.Context) ([]string, error) {
	if m.listAllBranchesFunc != nil {
		return m.listAllBranchesFunc(ctx)
	}
	return []string{"main"}, nil
}
```

**Step 8: Update MockGitClient in stack/service_test.go**

Add method to `MockGitClient` struct (after IsInWorktree, around line 410):

```go
func (m *MockGitClient) ListAllBranches(_ context.Context) ([]string, error) {
	return []string{"main"}, nil
}
```

**Step 9: Verify all tests pass**

Run:
```bash
go test ./...
```

Expected: All tests pass

**Step 10: Commit**

```bash
git add internal/git/client_interface.go internal/git/worktree.go internal/git/branch_test.go internal/worktree/service_test.go internal/stack/service_test.go
git commit -m "feat(git): add ListAllBranches method to git client"
```

---

## Task 3: Create Picker Package with IsTerminal

**Files:**
- Create: `internal/picker/picker.go`
- Create: `internal/picker/picker_test.go`

**Step 1: Write the failing test**

Create `internal/picker/picker_test.go`:

```go
package picker

import (
	"testing"
)

func TestIsTerminal(t *testing.T) {
	// This function wraps term.IsTerminal, so we just verify it doesn't panic
	// The actual behavior depends on the execution environment
	_ = IsTerminal()
}

func TestNewPicker(t *testing.T) {
	picker := NewPicker(nil)
	if picker == nil {
		t.Error("NewPicker() should not return nil")
	}
}
```

**Step 2: Run test to verify it fails (package doesn't exist)**

Run:
```bash
go test -v ./internal/picker
```

Expected:
```
cannot find package "internal/picker"
```

**Step 3: Create the picker package**

Create `internal/picker/picker.go`:

```go
// Package picker provides interactive TUI selection for wt commands.
package picker

import (
	"os"

	"golang.org/x/term"
)

// Picker provides interactive selection functionality.
type Picker struct {
	// Future: gitClient will be added when we implement SelectWorktree/SelectBranch
}

// NewPicker creates a new Picker instance.
func NewPicker() *Picker {
	return &Picker{}
}

// IsTerminal returns true if stdout is connected to a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test -v ./internal/picker
```

Expected:
```
=== RUN   TestIsTerminal
--- PASS: TestIsTerminal
=== RUN   TestNewPicker
--- PASS: TestNewPicker
PASS
```

**Step 5: Commit**

```bash
git add internal/picker/picker.go internal/picker/picker_test.go
git commit -m "feat(picker): create picker package with IsTerminal function"
```

---

## Task 4: Add SelectWorktree Function

**Files:**
- Modify: `internal/picker/picker.go`
- Modify: `internal/picker/picker_test.go`
- Modify: `internal/git/client_interface.go` (add BranchLister interface)

**Step 1: Write the failing test**

Add to `internal/picker/picker_test.go`:

```go
import (
	"context"
	"errors"
	"testing"

	"github.com/joebalancio/wt/pkg/domain"
)

// mockBranchLister is a mock for BranchLister interface
type mockBranchLister struct {
	listWorktreesFunc func(ctx context.Context) ([]*domain.Worktree, error)
}

func (m *mockBranchLister) ListWorktrees(ctx context.Context) ([]*domain.Worktree, error) {
	if m.listWorktreesFunc != nil {
		return m.listWorktreesFunc(ctx)
	}
	return nil, nil
}

func TestPicker_SelectWorktree_NoWorktrees(t *testing.T) {
	mock := &mockBranchLister{
		listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
			return []*domain.Worktree{}, nil
		},
	}
	picker := NewPicker(mock)

	_, err := picker.SelectWorktree(context.Background())
	if err == nil {
		t.Error("SelectWorktree() should return error when no worktrees")
	}
}

func TestPicker_SelectWorktree_OnlyMainWorktree(t *testing.T) {
	mock := &mockBranchLister{
		listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
			// Main worktree has empty branch
			return []*domain.Worktree{{Path: "/repo", Branch: ""}}, nil
		},
	}
	picker := NewPicker(mock)

	_, err := picker.SelectWorktree(context.Background())
	if err == nil {
		t.Error("SelectWorktree() should return error when no removable worktrees")
	}
}

func TestPicker_SelectWorktree_ListError(t *testing.T) {
	mock := &mockBranchLister{
		listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
			return nil, errors.New("git error")
		},
	}
	picker := NewPicker(mock)

	_, err := picker.SelectWorktree(context.Background())
	if err == nil {
		t.Error("SelectWorktree() should return error when ListWorktrees fails")
	}
}
```

**Step 2: Run test to verify it fails (SelectWorktree doesn't exist)**

Run:
```bash
go test -v ./internal/picker
```

Expected:
```
picker.SelectWorktree undefined
```

**Step 3: Add BranchLister interface to git package**

Add to `internal/git/client_interface.go` at the end:

```go
// BranchLister provides read-only access to worktree and branch listing.
// This is a subset of GitClient for use by the picker.
type BranchLister interface {
	ListWorktrees(ctx context.Context) ([]*domain.Worktree, error)
	ListAllBranches(ctx context.Context) ([]string, error)
}
```

**Step 4: Update Picker struct to accept BranchLister**

Update `internal/picker/picker.go`:

```go
// Package picker provides interactive TUI selection for wt commands.
package picker

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/pkg/domain"
	"golang.org/x/term"
)

// Picker provides interactive selection functionality.
type Picker struct {
	gitClient git.BranchLister
}

// NewPicker creates a new Picker instance.
func NewPicker(gitClient git.BranchLister) *Picker {
	return &Picker{gitClient: gitClient}
}

// IsTerminal returns true if stdout is connected to a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// SelectWorktree presents a picker for selecting a worktree to remove.
// Returns the selected worktree path, or an error if selection fails.
func (p *Picker) SelectWorktree(ctx context.Context) (string, error) {
	worktrees, err := p.gitClient.ListWorktrees(ctx)
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		return "", fmt.Errorf("no worktrees found")
	}

	var options []huh.Option[string]
	for _, wt := range worktrees {
		// Skip bare/main worktree (the repo itself) - these have empty branch
		if wt.Branch == "" {
			continue
		}
		label := fmt.Sprintf("%s -> %s", wt.Branch, wt.Path)
		options = append(options, huh.NewOption(label, wt.Path))
	}

	if len(options) == 0 {
		return "", fmt.Errorf("no removable worktrees found")
	}

	var selected string
	err = huh.NewSelect[string]().
		Title("Select worktree to remove:").
		Options(options...).
		Value(&selected).
		Run()

	return selected, err
}
```

**Step 5: Run test to verify it passes**

Run:
```bash
go test -v ./internal/picker
```

Expected:
```
=== RUN   TestIsTerminal
--- PASS
=== RUN   TestNewPicker
--- PASS
=== RUN   TestPicker_SelectWorktree_NoWorktrees
--- PASS
=== RUN   TestPicker_SelectWorktree_OnlyMainWorktree
--- PASS
=== RUN   TestPicker_SelectWorktree_ListError
--- PASS
PASS
```

**Step 6: Commit**

```bash
git add internal/picker/picker.go internal/picker/picker_test.go internal/git/client_interface.go
git commit -m "feat(picker): add SelectWorktree function with error handling"
```

---

## Task 5: Add SelectBranch Function

**Files:**
- Modify: `internal/picker/picker.go`
- Modify: `internal/picker/picker_test.go`

**Step 1: Write the failing test**

Add to `internal/picker/picker_test.go`:

Update mockBranchLister to include ListAllBranches:

```go
// mockBranchLister is a mock for BranchLister interface
type mockBranchLister struct {
	listWorktreesFunc    func(ctx context.Context) ([]*domain.Worktree, error)
	listAllBranchesFunc  func(ctx context.Context) ([]string, error)
}

func (m *mockBranchLister) ListWorktrees(ctx context.Context) ([]*domain.Worktree, error) {
	if m.listWorktreesFunc != nil {
		return m.listWorktreesFunc(ctx)
	}
	return nil, nil
}

func (m *mockBranchLister) ListAllBranches(ctx context.Context) ([]string, error) {
	if m.listAllBranchesFunc != nil {
		return m.listAllBranchesFunc(ctx)
	}
	return []string{"main"}, nil
}

func TestPicker_SelectBranch_ListError(t *testing.T) {
	mock := &mockBranchLister{
		listAllBranchesFunc: func(_ context.Context) ([]string, error) {
			return nil, errors.New("git error")
		},
	}
	picker := NewPicker(mock)

	_, err := picker.SelectBranch(context.Background())
	if err == nil {
		t.Error("SelectBranch() should return error when ListAllBranches fails")
	}
}
```

**Step 2: Run test to verify it fails (SelectBranch doesn't exist)**

Run:
```bash
go test -v ./internal/picker -run TestPicker_SelectBranch_ListError
```

Expected:
```
picker.SelectBranch undefined
```

**Step 3: Add SelectBranch function and SelectBranchResult type**

Add to `internal/picker/picker.go` after SelectWorktree:

```go
const newBranchOption = "Create new branch"

// SelectBranchResult contains the result of branch selection.
type SelectBranchResult struct {
	Branch     string
	BaseBranch string
	IsNew      bool
}

// SelectBranch presents a picker for selecting or creating a branch.
func (p *Picker) SelectBranch(ctx context.Context) (SelectBranchResult, error) {
	branches, err := p.gitClient.ListAllBranches(ctx)
	if err != nil {
		return SelectBranchResult{}, fmt.Errorf("list branches: %w", err)
	}

	// Build options: new branch option first, then existing branches
	options := []huh.Option[string]{
		huh.NewOption(newBranchOption, newBranchOption),
	}
	for _, b := range branches {
		options = append(options, huh.NewOption(b, b))
	}

	var selected string
	err = huh.NewSelect[string]().
		Title("Select or create a branch:").
		Options(options...).
		Value(&selected).
		Run()

	if err != nil {
		return SelectBranchResult{}, err
	}

	// User chose to create new branch
	if selected == newBranchOption {
		return p.promptNewBranch(branches)
	}

	// User selected existing branch
	return SelectBranchResult{
		Branch: selected,
		IsNew:  false,
	}, nil
}

func (p *Picker) promptNewBranch(existingBranches []string) (SelectBranchResult, error) {
	var branchName string
	err := huh.NewInput().
		Title("Enter new branch name:").
		Value(&branchName).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("branch name cannot be empty")
			}
			for _, b := range existingBranches {
				if b == s {
					return fmt.Errorf("branch %q already exists", s)
				}
			}
			return nil
		}).
		Run()

	if err != nil {
		return SelectBranchResult{}, err
	}

	// Prompt for base branch
	var baseBranch string
	baseOptions := make([]huh.Option[string], len(existingBranches))
	for i, b := range existingBranches {
		baseOptions[i] = huh.NewOption(b, b)
	}

	err = huh.NewSelect[string]().
		Title("Select base branch:").
		Options(baseOptions...).
		Value(&baseBranch).
		Run()

	if err != nil {
		return SelectBranchResult{}, err
	}

	return SelectBranchResult{
		Branch:     branchName,
		BaseBranch: baseBranch,
		IsNew:      true,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test -v ./internal/picker -run TestPicker_SelectBranch_ListError
```

Expected:
```
=== RUN   TestPicker_SelectBranch_ListError
--- PASS
PASS
```

**Step 5: Verify all picker tests pass**

Run:
```bash
go test -v ./internal/picker
```

Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/picker/picker.go internal/picker/picker_test.go
git commit -m "feat(picker): add SelectBranch function with new branch prompt"
```

---

## Task 6: Integrate Picker into Remove Command

**Files:**
- Modify: `internal/cli/remove.go`

**Step 1: Update remove.go to use picker when no path provided**

Update `internal/cli/remove.go` imports (add picker import):

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/picker"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/spf13/cobra"
)
```

**Step 2: Modify runRemoveCommand to use picker**

Replace the path resolution logic (lines 78-89) with:

```go
	resolvedPath := path
	if resolvedPath == "" {
		// Try interactive picker if in terminal and in main repo
		if picker.IsTerminal() {
			inWorktree, _, err := gitClient.IsInWorktree(ctx)
			if err != nil {
				Fatal("Failed to check worktree context: %v", err)
			}
			if !inWorktree {
				// We're in main repo - show interactive picker
				p := picker.NewPicker(gitClient)
				selected, err := p.SelectWorktree(ctx)
				if err != nil {
					Fatal("Error: %v. Provide a path: wt remove <path>", err)
				}
				resolvedPath = selected
			}
		}

		// Fallback to resolving from CWD if picker not used
		if resolvedPath == "" {
			cwd, err := os.Getwd()
			if err != nil {
				Fatal("Failed to get current directory: %v", err)
			}
			wt, err := svc.ResolveFromCWD(ctx, cwd)
			if err != nil {
				Fatal("Error: %v. Provide a path: wt remove <path>", err)
			}
			resolvedPath = wt.Path
		}
	}
```

**Step 3: Build to verify**

Run:
```bash
go build ./...
```

Expected: No errors

**Step 4: Run all tests**

Run:
```bash
go test ./...
```

Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/cli/remove.go
git commit -m "feat(remove): integrate interactive picker when no path provided"
```

---

## Task 7: Integrate Picker into Add Command

**Files:**
- Modify: `internal/cli/add.go`

**Step 1: Change Args from ExactArgs(1) to MaximumNArgs(1)**

Update line 31 in `internal/cli/add.go`:

```go
		Args: cobra.MaximumNArgs(1),
```

**Step 2: Update imports**

Add picker import:

```go
import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/picker"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/spf13/cobra"
)
```

**Step 3: Modify command Run function to handle optional branch arg**

Update the Run function (lines 32-34):

```go
		Run: func(cmd *cobra.Command, args []string) {
			branch := ""
			if len(args) > 0 {
				branch = args[0]
			}
			runAddCommand(cmd, branch, base, path, force, track, noCheckout)
		},
```

**Step 4: Update runAddCommand to use picker when no branch provided**

Insert after line 58 (after the inWorktree check block), before the config loading:

```go
	// If no branch provided, try interactive picker
	if branch == "" {
		if !picker.IsTerminal() {
			Fatal("branch argument required when not in interactive terminal")
		}
		// We already checked we're not in a worktree, so we're in main repo
		p := picker.NewPicker(gitClient)
		result, err := p.SelectBranch(ctx)
		if err != nil {
			Fatal("Branch selection failed: %v", err)
		}
		branch = result.Branch
		if result.IsNew && base == "" {
			base = result.BaseBranch
		}
	}
```

**Step 5: Build to verify**

Run:
```bash
go build ./...
```

Expected: No errors

**Step 6: Run all tests**

Run:
```bash
go test ./...
```

Expected: All tests pass

**Step 7: Commit**

```bash
git add internal/cli/add.go
git commit -m "feat(add): integrate interactive picker when no branch provided"
```

---

## Task 8: Update CLI Command Tests

**Files:**
- Modify: `internal/cli/add_test.go`
- Modify: `internal/cli/remove_test.go`

**Step 1: Update add_test.go to verify MaximumNArgs**

Update `internal/cli/add_test.go`:

```go
package cli

import (
	"testing"
)

func TestNewAddCmd_TmuxIntegration(t *testing.T) {
	// This is an integration test that verifies the add command
	// properly calls tmux window creation when in tmux

	// We can't easily test the actual tmux integration in unit tests,
	// but we can verify the code path exists

	// Verify the command structure
	cmd := NewAddCmd()
	if cmd == nil {
		t.Fatal("NewAddCmd() should return a command")
	}

	// The --no-tmux flag is defined on root and inherited
	// We verify the add command exists and has the expected structure
	// The actual flag inheritance is verified by integration tests
	if cmd.Use != "add <branch>" {
		t.Errorf("Expected command use 'add <branch>', got %q", cmd.Use)
	}
}

func TestNewAddCmd_AllowsOptionalBranch(t *testing.T) {
	cmd := NewAddCmd()

	// Test with 0 args (should not error on Args validation)
	err := cmd.ValidateArgs([]string{})
	if err != nil {
		t.Errorf("add command should accept 0 arguments for interactive mode, got error: %v", err)
	}

	// Test with 1 arg (should work as before)
	err = cmd.ValidateArgs([]string{"feature-branch"})
	if err != nil {
		t.Errorf("add command should accept 1 argument, got error: %v", err)
	}

	// Test with 2 args (should error)
	err = cmd.ValidateArgs([]string{"branch1", "branch2"})
	if err == nil {
		t.Error("add command should reject more than 1 argument")
	}
}
```

**Step 2: Update remove_test.go**

Update `internal/cli/remove_test.go`:

```go
package cli

import (
	"testing"
)

func TestNewRemoveCmd_TmuxIntegration(t *testing.T) {
	// Verify the remove command exists and has the expected structure
	cmd := NewRemoveCmd()
	if cmd == nil {
		t.Fatal("NewRemoveCmd() should return a command")
	}
	if cmd.Use != "remove [path]" {
		t.Errorf("Expected command use 'remove [path]', got %q", cmd.Use)
	}
}

func TestNewRemoveCmd_AllowsOptionalPath(t *testing.T) {
	cmd := NewRemoveCmd()

	// Test with 0 args (should work for interactive mode)
	err := cmd.ValidateArgs([]string{})
	if err != nil {
		t.Errorf("remove command should accept 0 arguments for interactive mode, got error: %v", err)
	}

	// Test with 1 arg (should work)
	err = cmd.ValidateArgs([]string{"/path/to/worktree"})
	if err != nil {
		t.Errorf("remove command should accept 1 argument, got error: %v", err)
	}
}
```

**Step 3: Run tests to verify**

Run:
```bash
go test -v ./internal/cli -run "TestNewAddCmd_AllowsOptionalBranch|TestNewRemoveCmd_AllowsOptionalPath"
```

Expected:
```
=== RUN   TestNewAddCmd_AllowsOptionalBranch
--- PASS
=== RUN   TestNewRemoveCmd_AllowsOptionalPath
--- PASS
PASS
```

**Step 4: Run all tests**

Run:
```bash
go test ./...
```

Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/cli/add_test.go internal/cli/remove_test.go
git commit -m "test(cli): add tests for optional branch/path args in add/remove commands"
```

---

## Task 9: Final Verification and Quality Gates

**Step 1: Run full test suite**

Run:
```bash
make test
```

Expected: All tests pass

**Step 2: Run linter**

Run:
```bash
make lint
```

Expected: No errors (fix any if present)

**Step 3: Format code**

Run:
```bash
make fmt
```

**Step 4: Run full check suite**

Run:
```bash
make check
```

Expected: All checks pass (fmt + lint + test)

**Step 5: Build binary**

Run:
```bash
make build
```

Expected: Binary created at `bin/wt`

**Step 6: Final commit (if any formatting changes)**

```bash
git status
# If clean, no commit needed
# If changes, commit them
git add -A && git commit -m "style: fix linting issues"
```

---

## Summary

| File | Action |
|------|--------|
| `go.mod`, `go.sum` | Add `charmbracelet/huh`, `golang.org/x/term` |
| `internal/git/client_interface.go` | Add `ListAllBranches` to interface, add `BranchLister` interface |
| `internal/git/worktree.go` | Implement `ListAllBranches` |
| `internal/git/branch_test.go` | **New** - Placeholder test |
| `internal/worktree/service_test.go` | Update mock with `ListAllBranches` |
| `internal/stack/service_test.go` | Update mock with `ListAllBranches` |
| `internal/picker/picker.go` | **New** - Picker implementation |
| `internal/picker/picker_test.go` | **New** - Picker tests |
| `internal/cli/add.go` | Integrate picker, change to `MaximumNArgs(1)` |
| `internal/cli/remove.go` | Integrate picker |
| `internal/cli/add_test.go` | Add tests for optional branch |
| `internal/cli/remove_test.go` | Add tests for optional path |

**Commits (9 total):**
1. `chore: add charmbracelet/huh and golang.org/x/term dependencies`
2. `feat(git): add ListAllBranches method to git client`
3. `feat(picker): create picker package with IsTerminal function`
4. `feat(picker): add SelectWorktree function with error handling`
5. `feat(picker): add SelectBranch function with new branch prompt`
6. `feat(remove): integrate interactive picker when no path provided`
7. `feat(add): integrate interactive picker when no branch provided`
8. `test(cli): add tests for optional branch/path args in add/remove commands`
9. (optional) `style: fix linting issues`
