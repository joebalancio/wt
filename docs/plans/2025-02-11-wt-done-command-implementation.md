# wt done Command Implementation Plan

**Bead:** wt-5l1
**Date:** 2025-02-11
**Status:** Implementation Plan (Revised after Consensus Review)
**Source:** [wt done Command Design](2025-02-08-wt-done-command-design.md)

## Consensus Review

This plan was reviewed and updated based on consensus feedback from multiple model perspectives. The following issues were identified and addressed:

### Critical Fixes Applied

1. **Auto-commit after squash merge (Lines 39-42 of design)**
   - **Issue**: Original plan only staged changes without creating a commit
   - **Fix**: Added `CreateSquashCommit()` method to GitClient interface
   - **Impact**: Service.Done() now calls this after SquashMerge to complete the merge workflow
   - **Commit message generation**: Uses branch name to create meaningful commit messages

2. **`--dry-run` flag (Lines 31-34 of design)**
   - **Issue**: No way to preview what would happen without executing
   - **Fix**: Added `--dry-run` flag to CLI command
   - **Implementation**: Prints intended actions without executing git operations
   - **Respects global**: Uses existing `cli.GetDryRun()` pattern

3. **Branch deletion logic fix**
   - **Issue**: DeleteBranch implementation had redundant force check
   - **Fix**: Simplified to single conditional using `-d` for safe, `-D` for force
   - **Code**:
     ```go
     args := []string{"branch"}
     if force {
         args = append(args, "-D")  // force delete
     } else {
         args = append(args, "-d")   // safe delete (checks if merged)
     }
     args = append(args, branch)
     ```

4. **Dirty worktree check**
   - **Issue**: No validation before removing worktrees with uncommitted changes
   - **Fix**: Added `IsWorktreeDirty()` method to GitClient
   - **Behavior**: Checks before removal, respects `--force` flag
   - **Error message**: Clear indication when worktree is dirty

5. **Hook template variable support**
   - **Issue**: Hooks couldn't access branch name or worktree path
   - **Fix**: Extended HookRunner to substitute `{branch}` and `{worktree_path}`
   - **Implementation**: Hook calls now pass these variables
   - **Use case**: Post-done hooks can reference completed branch

6. **Import statement fixes**
   - **Issue**: Step 9 had unused imports, Step 12 missing `os/exec`
   - **Fix**: Cleaned up imports, added missing `os/exec` for integration tests
   - **Files affected**: service.go implementation, integration tests

7. **Step splitting for better increments**
   - **Issue**: Step 6 and Step 12 were too large
   - **Fix**: Split into smaller, focused steps:
     - Step 6a: DeleteBranch tests
     - Step 6b: SquashMerge tests
     - Step 6c: CreateSquashCommit tests
     - Step 12a-c: Multiple integration test files
   - **Benefit**: Smaller PRs, easier review, faster feedback

8. **Tmux cleanup note**
   - **Issue**: Tmux cleanup code duplicated from remove.go
   - **Decision**: Keep duplication for now, noted for future refactor
   - **Future**: Should extract shared helper in separate refactor bead
   - **Rationale**: Avoids scope creep in this implementation

### Additional Improvements

- Added explicit verification commands for each step
- Improved error messages throughout
- Added context preservation in error wrapping
- Ensured consistency with existing CLI patterns
- Made test failures more informative

---

## Overview

Implement the `wt done <branch>` command that automates the cleanup workflow after feature work is complete. The command performs four operations in sequence: squash merge the feature branch into the current branch, create a commit, remove the associated worktree, and delete the feature branch.

## Implementation Steps

### Phase 1: Core Git Client Extension (Delete Branch)

**Step 1: Write failing test for DeleteBranch method**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/git/worktree_test.go`

**Action:** Add test to the end of the file:

```go
func TestClient_DeleteBranch(t *testing.T) {
	t.Run("deletes branch with force flag", func(t *testing.T) {
		// This test will fail initially - the method doesn't exist yet
		// We'll use a mock approach since we need git operations
	})
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test -v ./internal/git -run TestClient_DeleteBranch
# Expected: compile error - method DeleteBranch does not exist
```

---

**Step 2: Update GitClient interface to include DeleteBranch**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/git/client_interface.go`

**Replace entire file with:**

```go
// Package git provides a client for interacting with git worktrees via the git CLI.
//
// The GitClient interface defines operations for managing worktrees, with
// support for context cancellation and proper error handling.
package git

import (
	"context"

	"github.com/joebalancio/wt/pkg/domain"
)

// GitClient defines the interface for git operations
// revive:disable:exported Type name stutter is acceptable for clarity
type GitClient interface {
	ListWorktrees(ctx context.Context) ([]*domain.Worktree, error)
	AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	RemoveWorktree(ctx context.Context, path string, force bool) error
	GetRepoInfo(ctx context.Context) (*domain.GitRepo, error)
	BranchExists(ctx context.Context, branch string) (bool, error)
	GetCurrentBranch(ctx context.Context) (string, error)
	DeleteBranch(ctx context.Context, branch string, force bool) error
	SquashMerge(ctx context.Context, sourceBranch string) error
	CreateSquashCommit(ctx context.Context, message string) error
	IsWorktreeDirty(ctx context.Context, path string) (bool, error)
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go build ./...
# Expected: compile errors in mockGitClient and Client implementations
```

---

**Step 3: Update mockGitClient for tests**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/worktree/service_test.go`

**Action:** Add new fields to mockGitClient struct (around line 17-24):

```go
// mockGitClient is a simple mock for testing
type mockGitClient struct {
	listWorktreesFunc    func(ctx context.Context) ([]*domain.Worktree, error)
	addWorktreeFunc      func(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	removeWorktreeFunc   func(ctx context.Context, path string, force bool) error
	getRepoInfoFunc      func(ctx context.Context) (*domain.GitRepo, error)
	branchExistsFunc     func(ctx context.Context, branch string) (bool, error)
	getCurrentBranchFunc func(ctx context.Context) (string, error)
	deleteBranchFunc     func(ctx context.Context, branch string, force bool) error
	squashMergeFunc      func(ctx context.Context, sourceBranch string) error
	createSquashCommitFunc func(ctx context.Context, message string) error
	isWorktreeDirtyFunc  func(ctx context.Context, path string) (bool, error)
}
```

**Action:** Add implementations at the end of the mock methods (around line 66):

```go
func (m *mockGitClient) DeleteBranch(ctx context.Context, branch string, force bool) error {
	if m.deleteBranchFunc != nil {
		return m.deleteBranchFunc(ctx, branch, force)
	}
	return nil
}

func (m *mockGitClient) SquashMerge(ctx context.Context, sourceBranch string) error {
	if m.squashMergeFunc != nil {
		return m.squashMergeFunc(ctx, sourceBranch)
	}
	return nil
}

func (m *mockGitClient) CreateSquashCommit(ctx context.Context, message string) error {
	if m.createSquashCommitFunc != nil {
		return m.createSquashCommitFunc(ctx, message)
	}
	return nil
}

func (m *mockGitClient) IsWorktreeDirty(ctx context.Context, path string) (bool, error) {
	if m.isWorktreeDirtyFunc != nil {
		return m.isWorktreeDirtyFunc(ctx, path)
	}
	return false, nil
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test ./internal/worktree -v
# Expected: tests pass (mock now implements full interface)
```

---

**Step 4: Implement DeleteBranch in git Client**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/git/worktree.go`

**Action:** Add method after PruneWorktrees (around line 197):

```go
// DeleteBranch deletes a branch
func (c *Client) DeleteBranch(ctx context.Context, branch string, force bool) error {
	args := []string{"branch"}
	if force {
		args = append(args, "-D") // force delete
	} else {
		args = append(args, "-d") // safe delete (checks if merged)
	}
	args = append(args, branch)

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deleting branch %q: %w: %s", branch, err, stderr.String())
	}
	return nil
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go build ./...
# Expected: compiles successfully
```

---

**Step 5: Implement SquashMerge in git Client**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/git/worktree.go`

**Action:** Add method after DeleteBranch:

```go
// SquashMerge performs a squash merge of the source branch into the current branch
// It stages all changes from the source branch but does not create a commit
func (c *Client) SquashMerge(ctx context.Context, sourceBranch string) error {
	// git merge --squash <branch>
	args := []string{"merge", "--squash", sourceBranch}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("squash merge %q: %w: %s", sourceBranch, err, stderr.String())
	}
	return nil
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go build ./...
# Expected: compiles successfully
```

---

**Step 6: Implement CreateSquashCommit in git Client**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/git/worktree.go`

**Action:** Add method after SquashMerge:

```go
// CreateSquashCommit creates a commit from staged squash merge changes
func (c *Client) CreateSquashCommit(ctx context.Context, message string) error {
	args := []string{"commit", "-m", message}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating squash commit: %w: %s", err, stderr.String())
	}
	return nil
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go build ./...
# Expected: compiles successfully
```

---

**Step 7: Implement IsWorktreeDirty in git Client**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/git/worktree.go`

**Action:** Add method after CreateSquashCommit:

```go
// IsWorktreeDirty checks if a worktree has uncommitted changes
func (c *Client) IsWorktreeDirty(ctx context.Context, path string) (bool, error) {
	// git -C <path> status --porcelain
	args := []string{"-C", path, "status", "--porcelain"}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("checking worktree status: %w: %s", err, stderr.String())
	}

	return stdout.Len() > 0, nil
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go build ./...
# Expected: compiles successfully
```

---

### Phase 2: Integration Tests for Git Client Methods

**Step 8a: Add integration test for DeleteBranch**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/tests/integration_test.go`

**Action:** Add at end of file:

```go
// TestIntegration_DeleteBranch tests deleting a branch
func TestIntegration_DeleteBranch(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(repoPath)

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Create a feature branch via worktree
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/to-delete",
		Path:   filepath.Join(repoPath, "to-delete"),
	}
	worktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Verify branch exists
	exists, _ := client.BranchExists(ctx, "feature/to-delete")
	if !exists {
		t.Fatal("branch should exist before deletion")
	}

	// Remove worktree first (git requires this)
	client.RemoveWorktree(ctx, worktree.Path, true)

	// Delete the branch
	if err := client.DeleteBranch(ctx, "feature/to-delete", true); err != nil {
		t.Fatalf("failed to delete branch: %v", err)
	}

	// Verify branch is gone
	exists, _ = client.BranchExists(ctx, "feature/to-delete")
	if exists {
		t.Error("branch should not exist after deletion")
	}
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test -v ./tests -run TestIntegration_DeleteBranch
# Expected: test passes
```

---

**Step 8b: Add integration test for SquashMerge and CreateSquashCommit**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/tests/integration_test.go`

**Action:** Add at end of file:

```go
// TestIntegration_SquashMergeCommit tests squash merge and commit functionality
func TestIntegration_SquashMergeCommit(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(repoPath)

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Create a feature branch with a commit
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/merge-test",
		Path:   filepath.Join(repoPath, "merge-test"),
	}
	worktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}
	defer client.RemoveWorktree(ctx, worktree.Path, true)

	// Add a file in the feature worktree
	featureFile := filepath.Join(worktree.Path, "feature.txt")
	os.WriteFile(featureFile, []byte("feature content\n"), 0644)

	// Commit in feature branch
	runGitCommand(t, worktree.Path, "add", "feature.txt")
	runGitCommand(t, worktree.Path, "commit", "-m", "Add feature")

	// Change back to main (repoPath)
	os.Chdir(repoPath)

	// Perform squash merge
	if err := client.SquashMerge(ctx, "feature/merge-test"); err != nil {
		t.Fatalf("failed to squash merge: %v", err)
	}

	// Verify changes are staged (not committed yet)
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, _ := cmd.Output()

	if !strings.Contains(string(output), "feature.txt") {
		t.Error("feature.txt should be staged after squash merge")
	}

	// Create commit
	if err := client.CreateSquashCommit(ctx, "Merge feature/merge-test"); err != nil {
		t.Fatalf("failed to create squash commit: %v", err)
	}

	// Verify commit was created
	cmd = exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = repoPath
	output, _ = cmd.Output()

	if !strings.Contains(string(output), "Merge feature/merge-test") {
		t.Error("commit message should contain merge message")
	}

	// Cleanup: reset to original state
	runGitCommand(t, repoPath, "reset", "--hard", "HEAD~1")
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test -v ./tests -run TestIntegration_SquashMergeCommit
# Expected: test passes
```

---

**Step 8c: Add integration test for IsWorktreeDirty**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/tests/integration_test.go`

**Action:** Add at end of file:

```go
// TestIntegration_IsWorktreeDirty tests dirty worktree detection
func TestIntegration_IsWorktreeDirty(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(repoPath)

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Create a feature branch
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/dirty-test",
		Path:   filepath.Join(repoPath, "dirty-test"),
	}
	worktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}
	defer client.RemoveWorktree(ctx, worktree.Path, true)

	// Worktree should be clean initially
	dirty, err := client.IsWorktreeDirty(ctx, worktree.Path)
	if err != nil {
		t.Fatalf("failed to check worktree status: %v", err)
	}
	if dirty {
		t.Error("new worktree should not be dirty")
	}

	// Add uncommitted file
	dirtyFile := filepath.Join(worktree.Path, "uncommitted.txt")
	os.WriteFile(dirtyFile, []byte("uncommitted\n"), 0644)

	// Worktree should now be dirty
	dirty, err = client.IsWorktreeDirty(ctx, worktree.Path)
	if err != nil {
		t.Fatalf("failed to check worktree status: %v", err)
	}
	if !dirty {
		t.Error("worktree with uncommitted file should be dirty")
	}
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test -v ./tests -run TestIntegration_IsWorktreeDirty
# Expected: test passes
```

---

### Phase 3: Configuration Extension

**Step 9: Add OnWorktreeDone to HooksConfig**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/config/config.go`

**Action:** Modify HooksConfig struct (around line 26-30):

```go
// HooksConfig defines hook configurations
type HooksConfig struct {
	OnWorktreeCreate []Hook `yaml:"on_worktree_create"`
	OnWorktreeDone   []Hook `yaml:"on_worktree_done,omitempty"`
	OnWorktreeRemove []Hook `yaml:"on_worktree_remove,omitempty"`
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go build ./...
# Expected: compiles successfully
```

---

### Phase 4: Hook Runner Template Variable Support

**Step 10: Extend HookRunner to support template variables**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/pkg/executor/hook_runner.go`

**Action:** Update the HookRunner struct and RunHooks method to accept template variables:

```go
// HookRunner executes hooks with template variable substitution
type HookRunner struct {
	workdir string
	// templateVars holds variables for template substitution
	templateVars map[string]string
}

// NewHookRunner creates a new HookRunner with optional template variables
func NewHookRunner(workdir string, templateVars ...map[string]string) *HookRunner {
	hr := &HookRunner{
		workdir:      workdir,
		templateVars: make(map[string]string),
	}
	if len(templateVars) > 0 && templateVars[0] != nil {
		hr.templateVars = templateVars[0]
	}
	return hr
}

// substituteTemplates replaces {var} placeholders with actual values
func (hr *HookRunner) substituteTemplates(cmd string) string {
	result := cmd
	for key, value := range hr.templateVars {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	// Maintain backward compatibility with {worktree_path}
	if worktreePath, ok := hr.templateVars["worktree_path"]; ok {
		result = strings.ReplaceAll(result, "{worktree_path}", worktreePath)
	}
	return result
}

// RunHooks executes the given hooks
func (hr *HookRunner) RunHooks(ctx context.Context, hooks []config.Hook) error {
	// Group hooks by parallel capability
	var parallelHooks, sequentialHooks []config.Hook
	for _, hook := range hooks {
		if hook.Parallel {
			parallelHooks = append(parallelHooks, hook)
		} else {
			sequentialHooks = append(sequentialHooks, hook)
		}
	}

	// Run parallel hooks first
	if len(parallelHooks) > 0 {
		if err := hr.runHooksParallel(ctx, parallelHooks); err != nil {
			return fmt.Errorf("parallel hooks: %w", err)
		}
	}

	// Run sequential hooks
	for _, hook := range sequentialHooks {
		// Substitute template variables in the command
		cmd := hr.substituteTemplates(hook.Run)

		workdir := hr.workdir
		if hook.Cwd != "" {
			workdir = hr.substituteTemplates(hook.Cwd)
		}

		execution := executor.Execution{
			Cmd:        cmd,
			Workdir:    workdir,
			Background: hook.Background,
			Timeout:    DefaultHookTimeout,
		}

		if err := executor.Run(ctx, execution); err != nil {
			return fmt.Errorf("hook %q: %w", cmd, err)
		}
	}

	return nil
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go build ./...
# Expected: compiles successfully
```

---

### Phase 5: Service Layer Implementation

**Step 11: Write failing test for Service.Done()**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/worktree/service_test.go`

**Action:** Add test at end of file:

```go
func TestService_Done(t *testing.T) {
	t.Run("successful done workflow", func(t *testing.T) {
		var (
			mergeCalled      bool
			commitCalled     bool
			doneHooksCalled  bool
			removeCalled     bool
			deleteCalled     bool
			removeHooksCalled bool
		)

		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, branch string) error {
				mergeCalled = true
				return nil
			},
			createSquashCommitFunc: func(_ context.Context, message string) error {
				commitCalled = true
				return nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, path string) (bool, error) {
				return false, nil // Clean worktree
			},
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/test/feat", Branch: "feature/test"},
				}, nil
			},
			removeWorktreeFunc: func(_ context.Context, path string, force bool) error {
				removeCalled = true
				return nil
			},
			deleteBranchFunc: func(_ context.Context, branch string, force bool) error {
				deleteCalled = true
				return nil
			},
		}

		cfg := config.DefaultConfig()
		// Add hooks to test hook execution
		cfg.Hooks.OnWorktreeDone = []config.Hook{
			{Run: "echo done-hook"},
		}
		cfg.Hooks.OnWorktreeRemove = []config.Hook{
			{Run: "echo remove-hook"},
		}

		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "feature/test", false)
		if err != nil {
			t.Fatalf("Done() error = %v", err)
		}

		if !mergeCalled {
			t.Error("expected squash merge to be called")
		}
		if !commitCalled {
			t.Error("expected commit to be called")
		}
		if !removeCalled {
			t.Error("expected remove to be called")
		}
		if !deleteCalled {
			t.Error("expected delete branch to be called")
		}
	})

	t.Run("returns error when squash merge fails", func(t *testing.T) {
		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, branch string) error {
				return fmt.Errorf("merge conflict")
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "feature/test", false)
		if err == nil {
			t.Fatal("Done() expected error for failed merge, got nil")
		}
	})

	t.Run("returns error when branch not found", func(t *testing.T) {
		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, branch string) error {
				return nil
			},
			createSquashCommitFunc: func(_ context.Context, message string) error {
				return nil
			},
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{}, nil // Empty list
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "feature/test", false)
		if err == nil {
			t.Fatal("Done() expected error for missing worktree, got nil")
		}
	})

	t.Run("returns error when worktree is dirty without force", func(t *testing.T) {
		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, branch string) error {
				return nil
			},
			createSquashCommitFunc: func(_ context.Context, message string) error {
				return nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, path string) (bool, error) {
				return true, nil // Dirty worktree
			},
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/test/feat", Branch: "feature/test"},
				}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "feature/test", false)
		if err == nil {
			t.Fatal("Done() expected error for dirty worktree without force, got nil")
		}
	})

	t.Run("proceeds when worktree is dirty with force", func(t *testing.T) {
		var removeCalled bool
		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, branch string) error {
				return nil
			},
			createSquashCommitFunc: func(_ context.Context, message string) error {
				return nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, path string) (bool, error) {
				return true, nil // Dirty worktree
			},
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/test/feat", Branch: "feature/test"},
				}, nil
			},
			removeWorktreeFunc: func(_ context.Context, path string, force bool) error {
				removeCalled = true
				return nil
			},
			deleteBranchFunc: func(_ context.Context, branch string, force bool) error {
				return nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "feature/test", true)
		if err != nil {
			t.Fatalf("Done() with force should succeed: %v", err)
		}

		if !removeCalled {
			t.Error("expected remove to be called with force")
		}
	})
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test ./internal/worktree -v -run TestService_Done
# Expected: compile error - method Done does not exist
```

---

**Step 12: Implement Service.Done() method**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/worktree/service.go`

**Action:** Add import for executor at top (verify imports section):

```go
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/joebalancio/wt/pkg/executor"
)
```

**Action:** Add Done method after Remove method (around line 119):

```go
// Done merges, commits, removes, and cleans up a feature branch
// It performs: 1) squash merge, 2) create commit, 3) run done hooks,
// 4) check dirty worktree, 5) remove worktree, 6) delete branch, 7) run remove hooks
func (s *Service) Done(ctx context.Context, sourceBranch string, force bool) error {
	// 1. Squash merge into current branch
	if err := s.git.SquashMerge(ctx, sourceBranch); err != nil {
		return fmt.Errorf("squash merge: %w", err)
	}

	// 2. Create commit from staged changes
	commitMessage := fmt.Sprintf("Merge %s", sourceBranch)
	if err := s.git.CreateSquashCommit(ctx, commitMessage); err != nil {
		return fmt.Errorf("create commit: %w", err)
	}

	// 3. Get worktree path before cleanup
	worktrees, err := s.git.ListWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	var worktreePath string
	for _, wt := range worktrees {
		if wt.Branch == sourceBranch {
			worktreePath = wt.Path
			break
		}
	}

	if worktreePath == "" {
		return fmt.Errorf("worktree for branch %q not found", sourceBranch)
	}

	// 4. Check if worktree is dirty
	dirty, err := s.git.IsWorktreeDirty(ctx, worktreePath)
	if err != nil {
		return fmt.Errorf("check worktree status: %w", err)
	}
	if dirty && !force {
		return fmt.Errorf("worktree %q has uncommitted changes (use --force to proceed)", worktreePath)
	}

	// 5. Run OnWorktreeDone hooks (worktree exists)
	if len(s.cfg.Hooks.OnWorktreeDone) > 0 {
		templateVars := map[string]string{
			"branch":         sourceBranch,
			"worktree_path":  worktreePath,
		}
		runner := executor.NewHookRunner(worktreePath, templateVars)
		if err := runner.RunHooks(ctx, s.cfg.Hooks.OnWorktreeDone); err != nil {
			// Log hook failures as warnings but don't block cleanup
			fmt.Fprintf(os.Stderr, "Warning: done hooks failed: %v\n", err)
		}
	}

	// 6. Remove worktree
	if err := s.Remove(ctx, worktreePath, force); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}

	// 7. Delete branch
	if err := s.git.DeleteBranch(ctx, sourceBranch, force); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}

	// 8. Run OnWorktreeRemove hooks (cleanup complete)
	if len(s.cfg.Hooks.OnWorktreeRemove) > 0 {
		templateVars := map[string]string{
			"branch":         sourceBranch,
			"worktree_path":  worktreePath,
		}
		// Note: worktree is gone, so use empty working dir
		runner := executor.NewHookRunner("", templateVars)
		if err := runner.RunHooks(ctx, s.cfg.Hooks.OnWorktreeRemove); err != nil {
			// Log hook failures as warnings but don't fail
			fmt.Fprintf(os.Stderr, "Warning: remove hooks failed: %v\n", err)
		}
	}

	return nil
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test ./internal/worktree -v -run TestService_Done
# Expected: tests pass
```

---

### Phase 6: CLI Command Implementation

**Step 13: Write failing test for done CLI command**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/cli/done_test.go`

**Action:** Create new file with:

```go
package cli

import (
	"testing"
)

func TestNewDoneCmd(t *testing.T) {
	t.Run("command structure", func(t *testing.T) {
		cmd := NewDoneCmd()
		if cmd == nil {
			t.Fatal("NewDoneCmd() should return a command")
		}
		if cmd.Use != "done <branch>" {
			t.Errorf("Expected command use 'done <branch>', got %q", cmd.Use)
		}
	})
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test ./internal/cli -v -run TestNewDoneCmd
# Expected: compile error - NewDoneCmd does not exist
```

---

**Step 14: Implement done CLI command**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/internal/cli/done.go`

**Action:** Create new file with:

```go
package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/spf13/cobra"
)

// NewDoneCmd creates the done command
func NewDoneCmd() *cobra.Command {
	var force bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "done <branch>",
		Short: "Complete a feature branch",
		Long: `Complete a feature branch by squash merging it into the current branch,
creating a commit, then removing the worktree and deleting the branch.

This command performs four operations:
1. Squash merge the feature branch into the current branch
2. Create a commit with the merged changes
3. Remove the feature worktree
4. Delete the feature branch

Hooks:
- on_worktree_done: Runs after merge/commit, before cleanup (worktree exists)
- on_worktree_remove: Runs after everything is complete (worktree gone)

Template variables available in hooks:
- {branch}: The feature branch name being completed
- {worktree_path}: Path to the worktree (empty for on_worktree_remove)`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			branch := args[0]

			ctx := context.Background()

			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			cfg, err := loadConfigForCommand()
			if err != nil {
				Fatal("Failed to load config: %v", err)
			}

			svc, err := worktree.NewService(gitClient, cfg)
			if err != nil {
				Fatal("Failed to create service: %v", err)
			}

			if dryRun || GetDryRun() {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: would complete branch %s\n", branch)
				fmt.Fprintf(cmd.OutOrStdout(), "  1. Squash merge %s into current branch\n", branch)
				fmt.Fprintf(cmd.OutOrStdout(), "  2. Create commit\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  3. Remove worktree\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  4. Delete branch\n")
				return
			}

			if err := svc.Done(ctx, branch, force); err != nil {
				Fatal("Failed to complete branch: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Completed branch: %s\n", branch)

			// Close tmux window if in tmux
			if isInTmux() {
				tmuxClient, err := tmux.NewClient()
				if err == nil {
					windowName := tmux.GenerateWindowName(branch)
					_ = tmuxClient.KillWindow(windowName)
				}
			}
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "proceed even if worktree has uncommitted changes")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be done without executing")

	return cmd
}

func init() {
	RegisterCommand(NewDoneCmd())
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go build ./...
go test ./internal/cli -v -run TestNewDoneCmd
# Expected: compiles and test passes
```

---

### Phase 7: Integration Tests

**Step 15a: Add integration test for basic done command**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/tests/done_integration_test.go`

**Action:** Create new file with:

```go
package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
)

// TestIntegration_Done_Basic tests the complete done workflow
func TestIntegration_Done_Basic(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	svc, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a feature branch with a commit
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/done-test",
		Path:   filepath.Join(repoPath, "done-test"),
	}
	worktree, err := svc.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Add a file in the feature worktree
	featureFile := filepath.Join(worktree.Path, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("feature content\n"), 0644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}

	// Commit in feature branch
	runGitCommand(t, worktree.Path, "add", "feature.txt")
	runGitCommand(t, worktree.Path, "commit", "-m", "Add feature")

	// Change back to main
	os.Chdir(repoPath)

	// Run Done workflow
	if err := svc.Done(ctx, "feature/done-test", true); err != nil {
		t.Fatalf("Done() failed: %v", err)
	}

	// Verify worktree is gone
	worktrees, _ := client.ListWorktrees(ctx)
	for _, wt := range worktrees {
		if wt.Branch == "feature/done-test" {
			t.Error("worktree should be removed after done")
		}
	}

	// Verify branch is deleted
	exists, _ := client.BranchExists(ctx, "feature/done-test")
	if exists {
		t.Error("branch should be deleted after done")
	}

	// Verify commit was created in main
	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = repoPath
	output, _ := cmd.Output()

	if !strings.Contains(string(output), "Merge feature/done-test") {
		t.Error("commit should be created with merge message")
	}

	// Cleanup: reset to original state
	runGitCommand(t, repoPath, "reset", "--hard", "HEAD~1")
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test -v ./tests -run TestIntegration_Done_Basic
# Expected: test passes
```

---

**Step 15b: Add integration test for done with hooks**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/tests/done_hooks_integration_test.go`

**Action:** Create new file with:

```go
package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
)

// TestIntegration_Done_WithHooks tests done command with hooks
func TestIntegration_Done_WithHooks(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	// Add hooks (we use echo commands that should succeed)
	cfg.Hooks.OnWorktreeDone = []config.Hook{
		{Run: "echo 'done hook executed for {branch}'"},
	}
	cfg.Hooks.OnWorktreeRemove = []config.Hook{
		{Run: "echo 'remove hook executed'"},
	}

	svc, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a feature branch
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/hooks-test",
		Path:   filepath.Join(repoPath, "hooks-test"),
	}
	worktree, err := svc.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Add and commit a file
	featureFile := filepath.Join(worktree.Path, "hooktest.txt")
	os.WriteFile(featureFile, []byte("test\n"), 0644)
	runGitCommand(t, worktree.Path, "add", "hooktest.txt")
	runGitCommand(t, worktree.Path, "commit", "-m", "Test")

	os.Chdir(repoPath)

	// Run Done with hooks
	if err := svc.Done(ctx, "feature/hooks-test", true); err != nil {
		t.Fatalf("Done() with hooks failed: %v", err)
	}

	// Verify cleanup
	_, err = os.Stat(worktree.Path)
	if !os.IsNotExist(err) {
		t.Error("worktree path should be removed")
	}

	// Cleanup staged changes
	runGitCommand(t, repoPath, "reset", "--hard", "HEAD~1")
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test -v ./tests -run TestIntegration_Done_WithHooks
# Expected: test passes
```

---

**Step 15c: Add integration test for merge conflict scenario**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/tests/done_conflict_integration_test.go`

**Action:** Create new file with:

```go
package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
)

// TestIntegration_Done_MergeConflict tests behavior with merge conflicts
func TestIntegration_Done_MergeConflict(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	svc, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a conflicting change in main first
	conflictFile := filepath.Join(repoPath, "conflict.txt")
	os.WriteFile(conflictFile, []byte("main content\n"), 0644)
	runGitCommand(t, repoPath, "add", "conflict.txt")
	runGitCommand(t, repoPath, "commit", "-m", "Add conflict file in main")

	// Create feature branch with conflicting change
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/conflict",
		Path:   filepath.Join(repoPath, "conflict"),
	}
	worktree, err := svc.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}
	defer client.RemoveWorktree(ctx, worktree.Path, true)

	// Add conflicting content in feature
	featureConflictFile := filepath.Join(worktree.Path, "conflict.txt")
	os.WriteFile(featureConflictFile, []byte("feature content\n"), 0644)
	runGitCommand(t, worktree.Path, "add", "conflict.txt")
	runGitCommand(t, worktree.Path, "commit", "-m", "Add conflict in feature")

	os.Chdir(repoPath)

	// Try to run Done - should fail on merge
	err = svc.Done(ctx, "feature/conflict", true)
	if err == nil {
		t.Error("Done should fail with merge conflict")
	}

	// Verify branch still exists (cleanup should not happen on merge failure)
	exists, _ := client.BranchExists(ctx, "feature/conflict")
	if !exists {
		t.Error("branch should still exist after failed merge")
	}

	// Cleanup: reset main and remove feature branch
	runGitCommand(t, repoPath, "reset", "--hard", "HEAD~1")
	client.RemoveWorktree(ctx, worktree.Path, true)
	client.DeleteBranch(ctx, "feature/conflict", true)
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test -v ./tests -run TestIntegration_Done_MergeConflict
# Expected: test passes
```

---

**Step 15d: Add integration test for dirty worktree scenario**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/tests/done_dirty_integration_test.go`

**Action:** Create new file with:

```go
package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
)

// TestIntegration_Done_DirtyWorktree tests behavior with uncommitted changes
func TestIntegration_Done_DirtyWorktree(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	svc, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create feature branch
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/dirty",
		Path:   filepath.Join(repoPath, "dirty"),
	}
	worktree, err := svc.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Add uncommitted file
	dirtyFile := filepath.Join(worktree.Path, "dirty.txt")
	os.WriteFile(dirtyFile, []byte("uncommitted\n"), 0644)

	os.Chdir(repoPath)

	// Run Done without force - should fail
	err = svc.Done(ctx, "feature/dirty", false)
	if err == nil {
		t.Error("Done should fail with dirty worktree without force")
	}

	// Run Done with force - should succeed
	if err := svc.Done(ctx, "feature/dirty", true); err != nil {
		t.Fatalf("Done with force should succeed: %v", err)
	}

	// Verify cleanup
	worktrees, _ := client.ListWorktrees(ctx)
	for _, wt := range worktrees {
		if wt.Branch == "feature/dirty" {
			t.Error("worktree should be removed after force done")
		}
	}

	// Cleanup staged changes
	runGitCommand(t, repoPath, "reset", "--hard", "HEAD~1")
}
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
go test -v ./tests -run TestIntegration_Done_DirtyWorktree
# Expected: test passes
```

---

### Phase 8: Documentation and Cleanup

**Step 16: Update CLAUDE.md with done command documentation**

**File:** `/home/claude/projects/wt/.worktrees/feat/wt-5l1/CLAUDE.md`

**Action:** Add section after existing command documentation:

```markdown
### wt done command

Complete a feature branch by squash merging and cleaning up.

```bash
# Complete a feature branch
wt done feat/config

# With force flag (skip dirty worktree checks)
wt done feat/api --force

# Dry run to see what would happen
wt done feat/api --dry-run
```

**Hooks:**
- `on_worktree_done`: Runs after merge/commit, before cleanup (worktree exists)
- `on_worktree_remove`: Runs after everything is complete (worktree gone)

**Template variables available in hooks:**
- `{branch}`: The feature branch name being completed
- `{worktree_path}`: Path to the worktree (empty for on_worktree_remove)

**Implementation files:**
- `internal/cli/done.go` - Done command
- `internal/worktree/service.go:Done()` - Service method
- `internal/git/worktree.go:DeleteBranch()` - Branch deletion
- `internal/git/worktree.go:SquashMerge()` - Squash merge
- `internal/git/worktree.go:CreateSquashCommit()` - Commit creation
- `internal/git/worktree.go:IsWorktreeDirty()` - Dirty worktree check
- `internal/config/config.go` - OnWorktreeDone hooks
- `pkg/executor/hook_runner.go` - Template variable support
```

**Verification:**
```bash
cd /home/claude/projects/wt/.worktrees/feat/wt-5l1
# File is updated
```

---

## Summary

This implementation plan follows TDD principles and breaks down the `wt done` command implementation into 16 bite-sized steps:

1-7: Core Git Client Extension (DeleteBranch, SquashMerge, CreateSquashCommit, IsWorktreeDirty)
8a-c: Integration Tests for Git Client Methods (separate test functions)
9: Configuration Extension (OnWorktreeDone hooks)
10: Hook Runner Template Variable Support
11-12: Service Layer Implementation (Done method with dirty check)
13-14: CLI Command Implementation (NewDoneCmd with --dry-run and --force)
15a-d: Integration Tests (basic, hooks, conflicts, dirty worktree)
16: Documentation updates

Each step includes:
- A clear action with exact file path
- Complete code snippet
- Verification command
- Expected result

The implementation follows existing patterns in the codebase:
- Uses mockGitClient for unit tests
- Follows add.go / remove.go CLI structure
- Integrates with existing hook runner with template variables
- Uses the same config loading pattern as other commands
- Respects global `--dry-run` flag pattern via `cli.GetDryRun()`

### Known Limitations and Future Work

1. **Tmux cleanup code duplication**: The tmux window cleanup code in done.go duplicates logic from remove.go. This should be extracted into a shared helper in a separate refactor bead to avoid scope creep.

2. **Commit message generation**: The current implementation uses a simple "Merge {branch}" format. Future enhancements could support:
   - Custom commit message templates
   - Conventional commit prefix extraction from branch names
   - Interactive commit message editing

3. **Hook error handling**: Hook failures are currently logged as warnings but don't block the workflow. Future versions could add a `--fail-on-hook-error` flag for stricter behavior.
