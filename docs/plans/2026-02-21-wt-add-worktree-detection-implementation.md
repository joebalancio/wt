# wt add: Prevent Nested Worktree Creation - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Block `wt add` when run from inside a worktree directory to prevent nested worktree creation.

**Architecture:** Add `IsInWorktree()` method to git client that compares `--show-toplevel` with `--git-common-dir` parent to detect worktree context. CLI layer validates and shows helpful error message with main repo location.

**Tech Stack:** Go 1.21+, Cobra CLI, git CLI

---

## Task 1: Add IsInWorktree to GitClient Interface

**Files:**
- Modify: `internal/git/client_interface.go:29`

**Step 1: Add interface method**

Add `IsInWorktree` method to the `GitClient` interface:

```go
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
	IsBranchMerged(ctx context.Context, branch string) (bool, error)
	RemoteBranchExists(ctx context.Context, remote, branch string) (bool, error)
	DeleteRemoteBranch(ctx context.Context, remote, branch string) error
	// IsInWorktree checks if the current directory is inside a git worktree.
	// Returns true if in a worktree, false if in main repo.
	// Also returns the main repository root path.
	IsInWorktree(ctx context.Context) (inWorktree bool, mainRepoRoot string, err error)
}
```

**Step 2: Verify compilation fails (expected)**

Run: `go build ./...`
Expected: Compile errors in `mockGitClient` (missing IsInWorktree method)

**Step 3: Commit**

```bash
git add internal/git/client_interface.go
git commit -m "feat(git): add IsInWorktree to GitClient interface

For wt-4rm: detect worktree context to prevent nested worktrees"
```

---

## Task 2: Update mockGitClient in service_test.go

**Files:**
- Modify: `internal/worktree/service_test.go:33`

**Step 1: Add mock field and method**

Add the new field to `mockGitClient` struct (after line 33):

```go
type mockGitClient struct {
	listWorktreesFunc      func(ctx context.Context) ([]*domain.Worktree, error)
	addWorktreeFunc        func(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	removeWorktreeFunc     func(ctx context.Context, path string, force bool) error
	getRepoInfoFunc        func(ctx context.Context) (*domain.GitRepo, error)
	branchExistsFunc       func(ctx context.Context, branch string) (bool, error)
	getCurrentBranchFunc   func(ctx context.Context) (string, error)
	deleteBranchFunc       func(ctx context.Context, branch string, force bool) error
	squashMergeFunc        func(ctx context.Context, sourceBranch string) error
	createSquashCommitFunc func(ctx context.Context, message string) error
	isWorktreeDirtyFunc    func(ctx context.Context, path string) (bool, error)
	isBranchMergedFunc     func(ctx context.Context, branch string) (bool, error)
	remoteBranchExistsFunc func(ctx context.Context, remote, branch string) (bool, error)
	deleteRemoteBranchFunc func(ctx context.Context, remote, branch string) error
	isInWorktreeFunc       func(ctx context.Context) (bool, string, error)
}
```

**Step 2: Add mock method implementation**

Add the method after the other mock methods (find where DeleteRemoteBranch mock ends):

```go
func (m *mockGitClient) IsInWorktree(ctx context.Context) (bool, string, error) {
	if m.isInWorktreeFunc != nil {
		return m.isInWorktreeFunc(ctx)
	}
	return false, "/repo", nil
}
```

**Step 3: Verify compilation passes**

Run: `go build ./...`
Expected: Success (no errors)

**Step 4: Commit**

```bash
git add internal/worktree/service_test.go
git commit -m "test: add IsInWorktree mock to service_test

For wt-4rm: mock for worktree context detection"
```

---

## Task 3: Write Failing Unit Test for IsInWorktree

**Files:**
- Modify: `internal/git/worktree_test.go`

**Step 1: Add failing test**

Append to `internal/git/worktree_test.go`:

```go
func TestIsInWorktree_MainRepo(t *testing.T) {
	skipIfNoGit(t)

	// Create a test repository
	tempDir := t.TempDir()
	runGitCommand(t, tempDir, "init", "-b", "main")
	runGitCommand(t, tempDir, "config", "user.name", "Test User")
	runGitCommand(t, tempDir, "config", "user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Initial commit")

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	inWorktree, mainRepoRoot, err := client.IsInWorktree(context.Background())
	if err != nil {
		t.Fatalf("IsInWorktree() error = %v", err)
	}

	if inWorktree {
		t.Error("IsInWorktree() returned true for main repo, want false")
	}

	// mainRepoRoot should be our temp directory
	if mainRepoRoot != tempDir {
		t.Errorf("mainRepoRoot = %q, want %q", mainRepoRoot, tempDir)
	}
}

func TestIsInWorktree_InWorktree(t *testing.T) {
	skipIfNoGit(t)

	// Create a test repository with a worktree
	tempDir := t.TempDir()
	runGitCommand(t, tempDir, "init", "-b", "main")
	runGitCommand(t, tempDir, "config", "user.name", "Test User")
	runGitCommand(t, tempDir, "config", "user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Initial commit")

	// Create a worktree
	worktreePath := filepath.Join(tempDir, "feature-test")
	runGitCommand(t, tempDir, "worktree", "add", "-b", "feature/test", worktreePath)

	// Change to worktree directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)
	if err := os.Chdir(worktreePath); err != nil {
		t.Fatal(err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	inWorktree, mainRepoRoot, err := client.IsInWorktree(context.Background())
	if err != nil {
		t.Fatalf("IsInWorktree() error = %v", err)
	}

	if !inWorktree {
		t.Error("IsInWorktree() returned false for worktree, want true")
	}

	// mainRepoRoot should point to the main repo, not the worktree
	if mainRepoRoot != tempDir {
		t.Errorf("mainRepoRoot = %q, want %q", mainRepoRoot, tempDir)
	}
}

// Helper function for tests
func runGitCommand(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, output)
	}
}

// Helper function for tests
func skipIfNoGit(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
}
```

**Step 2: Add missing imports**

Add at the top of the file after `package git`:

```go
import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)
```

**Step 3: Run test to verify it fails**

Run: `go test -v -run "TestIsInWorktree" ./internal/git/`
Expected: FAIL - "client.IsInWorktree undefined"

**Step 4: Commit**

```bash
git add internal/git/worktree_test.go
git commit -m "test(git): add failing tests for IsInWorktree

For wt-4rm: TDD red phase for worktree detection"
```

---

## Task 4: Implement IsInWorktree in git client

**Files:**
- Modify: `internal/git/worktree.go:311` (end of file)

**Step 1: Add helper method getOutput**

Add helper method before `IsInWorktree`:

```go
// getOutput runs a git command and returns its trimmed stdout
func (c *Client) getOutput(ctx context.Context, args ...string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}
```

**Step 2: Add IsInWorktree implementation**

Add the method at the end of the file:

```go
// IsInWorktree checks if the current directory is inside a git worktree.
// Returns true if in a worktree, false if in main repo.
// Also returns the main repository root path.
func (c *Client) IsInWorktree(ctx context.Context) (bool, string, error) {
	// Get current toplevel (worktree root or main repo root)
	toplevel, err := c.getOutput(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return false, "", fmt.Errorf("getting toplevel: %w", err)
	}

	// Get common git dir (always points to main repo's .git)
	gitCommonDir, err := c.getOutput(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return false, "", fmt.Errorf("getting git-common-dir: %w", err)
	}

	// Convert gitCommonDir to absolute path if it's relative
	if !filepath.IsAbs(gitCommonDir) {
		// gitCommonDir is relative to toplevel
		gitCommonDir = filepath.Join(toplevel, gitCommonDir)
	}

	// Main repo root is parent of .git directory
	mainRepoRoot := filepath.Dir(gitCommonDir)

	// Clean paths for comparison
	toplevel = filepath.Clean(toplevel)
	mainRepoRoot = filepath.Clean(mainRepoRoot)

	// If toplevel differs from main repo root, we're in a worktree
	inWorktree := toplevel != mainRepoRoot
	return inWorktree, mainRepoRoot, nil
}
```

**Step 3: Run tests to verify they pass**

Run: `go test -v -run "TestIsInWorktree" ./internal/git/`
Expected: PASS (both tests)

**Step 4: Run all tests**

Run: `go test ./...`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/git/worktree.go
git commit -m "feat(git): implement IsInWorktree method

Detects if current directory is inside a worktree by comparing
--show-toplevel with parent of --git-common-dir.

For wt-4rm: TDD green phase for worktree detection"
```

---

## Task 5: Write Failing Integration Test for CLI add

**Files:**
- Create: `tests/add_worktree_detection_test.go`

**Step 1: Create integration test file**

```go
package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_AddFromWorktree_Error tests that wt add fails
// with a helpful error when run from inside a worktree
func TestIntegration_AddFromWorktree_Error(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create main repo
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	// Create a worktree
	worktreePath := filepath.Join(repoPath, "existing-worktree")
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/existing",
		Path:   worktreePath,
		Base:   "main",
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	_, err = client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer client.RemoveWorktree(ctx, worktreePath, true)

	// Change to the worktree directory
	if err := os.Chdir(worktreePath); err != nil {
		t.Fatalf("failed to change to worktree directory: %v", err)
	}

	// Try to run wt add (should fail with helpful error)
	cmd := exec.Command("wt", "add", "feature/nested")
	output, err := cmd.CombinedOutput()

	// Should have failed
	if err == nil {
		t.Error("wt add should fail when run from inside a worktree")
	}

	// Check error message contains key information
	errMsg := string(output)
	if !strings.Contains(errMsg, "cannot add worktree from inside") {
		t.Errorf("error message should contain 'cannot add worktree from inside', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, repoPath) {
		t.Errorf("error message should show main repo path %q, got: %s", repoPath, errMsg)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `WT_INTEGRATION_TEST=1 go test -v -run "TestIntegration_AddFromWorktree_Error" ./tests/`
Expected: FAIL - wt add succeeds instead of failing

**Step 3: Commit**

```bash
git add tests/add_worktree_detection_test.go
git commit -m "test(cli): add failing integration test for add from worktree

For wt-4rm: TDD red phase for CLI worktree detection"
```

---

## Task 6: Add worktree check to CLI add command

**Files:**
- Modify: `internal/cli/add.go:45`

**Step 1: Add validation at start of runAddCommand**

Modify `runAddCommand` function to add worktree check after creating gitClient:

```go
func runAddCommand(cmd *cobra.Command, branch, base, path string, force bool, track string, noCheckout bool) {
	ctx := context.Background()

	gitClient, err := git.NewClient()
	if err != nil {
		Fatal("Failed to create git client: %v", err)
	}

	// Check if we're inside a worktree - this is not allowed
	inWorktree, mainRepoRoot, err := gitClient.IsInWorktree(ctx)
	if err != nil {
		Fatal("Failed to check worktree context: %v", err)
	}

	if inWorktree {
		// Get current toplevel for the error message
		repoInfo, err := gitClient.GetRepoInfo(ctx)
		currentPath := "unknown"
		if err == nil {
			currentPath = repoInfo.RootPath
		}

		Fatal(`cannot add worktree from inside another worktree

Current location: %s
Main repository:  %s

Run this command from the main repository instead:
  cd %s && wt add %s`,
			currentPath,
			mainRepoRoot,
			mainRepoRoot,
			branch)
	}

	cfg, err := loadConfigForCommand()
	if err != nil {
		Fatal("Failed to load config: %v", err)
	}

	svc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		Fatal("Failed to create service: %v", err)
	}

	spec := domain.WorktreeCreateSpec{
		Branch:   branch,
		Base:     base,
		Path:     path,
		Force:    force,
		Checkout: !noCheckout,
	}

	if track != "" {
		spec.Track = &track
	}

	wt, err := svc.Add(ctx, spec)
	if err != nil {
		Fatal("Failed to add worktree: %v", err)
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Created worktree: %s [%s]\n", wt.Path, wt.Branch); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Run setup hooks
	if err := runSetupHooks(ctx, wt.Path); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
	}

	// Create tmux window if in tmux and not disabled
	createTmuxWindowForWorktree(cmd, wt.Branch, wt.Path)
}
```

**Step 2: Build and verify**

Run: `make build`
Expected: Success

**Step 3: Run integration test**

Run: `WT_INTEGRATION_TEST=1 go test -v -run "TestIntegration_AddFromWorktree_Error" ./tests/`
Expected: PASS

**Step 4: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/cli/add.go
git commit -m "feat(cli): block wt add when run from inside a worktree

Prevents nested worktree creation by detecting worktree context
and showing helpful error with main repo location.

For wt-4rm: TDD green phase for CLI worktree detection"
```

---

## Task 7: Run Quality Gates

**Step 1: Run linter**

Run: `make lint`
Expected: No errors

**Step 2: Run all tests**

Run: `make test`
Expected: All pass

**Step 3: Run full check**

Run: `make check`
Expected: All pass

**Step 4: Manual verification**

Build and test manually:

```bash
make build

# In main repo
cd /home/claude/projects/wt/.worktrees/feature/wt-4rm
./bin/wt add test-manual-branch
# Expected: Creates worktree successfully

# Clean up
./bin/wt remove /path/to/test-manual-branch

# From inside a worktree
cd /home/claude/projects/wt/.worktrees/feature/wt-4rm/.worktrees  # if it exists
./bin/wt add test-nested-branch
# Expected: Error "cannot add worktree from inside another worktree"
```

---

## Task 8: Final Commit and Push

**Step 1: Review changes**

Run: `git status`
Run: `git log --oneline -10`

**Step 2: Push to remote**

```bash
git push -u origin feature/wt-4rm
```

---

## Summary

This plan implements worktree detection to prevent nested worktree creation:

1. **Interface** - Added `IsInWorktree()` to `GitClient` interface
2. **Mock** - Updated `mockGitClient` for tests
3. **Unit Tests** - Tests for main repo and worktree contexts
4. **Implementation** - Detect worktree by comparing `--show-toplevel` with parent of `--git-common-dir`
5. **Integration Test** - Test CLI blocks `wt add` from worktree
6. **CLI Integration** - Added check to `add` command with helpful error message

**Files Changed:**
- `internal/git/client_interface.go` - Interface method
- `internal/git/worktree.go` - Implementation
- `internal/git/worktree_test.go` - Unit tests
- `internal/worktree/service_test.go` - Mock update
- `internal/cli/add.go` - CLI integration
- `tests/add_worktree_detection_test.go` - Integration test
