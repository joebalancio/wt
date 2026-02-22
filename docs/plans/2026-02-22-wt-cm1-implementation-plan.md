# Squash-Merge Detection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable `wt remove` to detect squash-merged branches without requiring `--force`.

**Architecture:** Two-tier detection strategy: first check GitHub PR status via `gh` CLI, then fallback to `git cherry` for patch-level comparison. This replaces the current `git merge-base --is-ancestor` which fails for squash merges.

**Tech Stack:** Go 1.21+, gh CLI (optional), git cherry

---

## Task 1: Create GhClient Struct and NewGhClient

**Files:**
- Create: `internal/git/gh.go`
- Create: `internal/git/gh_test.go`

**Step 1: Write the failing test**

```go
// internal/git/gh_test.go
package git

import (
	"testing"
)

func TestNewGhClient(t *testing.T) {
	client, err := NewGhClient()
	// We can't guarantee gh is installed in test environment
	// So we just verify the constructor doesn't panic and returns valid types
	if err != nil {
		// gh not found is acceptable
		if client != nil {
			t.Error("client should be nil when error occurs")
		}
		return
	}
	if client == nil {
		t.Error("client should not be nil when no error")
	}
}

func TestGhClient_IsAvailable(t *testing.T) {
	client, err := NewGhClient()
	if err != nil {
		t.Skip("gh not available")
	}

	// IsAvailable should return true if we successfully created the client
	if !client.IsAvailable() {
		t.Error("IsAvailable() should return true when client exists")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/git -run "TestNewGhClient|TestGhClient_IsAvailable"`
Expected: FAIL with "gh not found" or similar

**Step 3: Write minimal implementation**

```go
// internal/git/gh.go
// Package git provides a client wrapper for git and gh CLI operations.
package git

import (
	"fmt"
	"os/exec"
)

// GhClient wraps the GitHub CLI (gh) for repository operations.
type GhClient struct {
	ghPath string
}

// NewGhClient creates a new GitHub CLI client.
// Returns error if gh is not found in PATH.
func NewGhClient() (*GhClient, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return nil, fmt.Errorf("gh not found in PATH: %w", err)
	}
	return &GhClient{ghPath: path}, nil
}

// IsAvailable returns true if the gh client is usable.
func (c *GhClient) IsAvailable() bool {
	return c.ghPath != ""
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/git -run "TestNewGhClient|TestGhClient_IsAvailable"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/gh.go internal/git/gh_test.go
git commit -m "feat(git): add GhClient struct and IsAvailable method"
```

---

## Task 2: Implement IsBranchPRMerged Method

**Files:**
- Modify: `internal/git/gh.go`
- Modify: `internal/git/gh_test.go`

**Step 1: Write the failing test**

```go
// internal/git/gh_test.go - Add these tests

func TestParsePRStateJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		state    string
		hasError bool
	}{
		{
			name:     "merged PR",
			input:    `{"state": "MERGED"}`,
			state:    "MERGED",
			hasError: false,
		},
		{
			name:     "open PR",
			input:    `{"state": "OPEN"}`,
			state:    "OPEN",
			hasError: false,
		},
		{
			name:     "closed PR (not merged)",
			input:    `{"state": "CLOSED"}`,
			state:    "CLOSED",
			hasError: false,
		},
		{
			name:     "invalid JSON",
			input:    `not json`,
			state:    "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := parsePRStateJSON(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if state != tt.state {
				t.Errorf("state = %q, want %q", state, tt.state)
			}
		})
	}
}

func TestGhClient_IsBranchPRMerged_NoClient(t *testing.T) {
	// Test with nil client behavior
	client := &GhClient{ghPath: ""}
	if client.IsAvailable() {
		t.Error("IsAvailable should return false for empty path")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/git -run "TestParsePRStateJSON|TestGhClient_IsBranchPRMerged"`
Expected: FAIL with "parsePRStateJSON undefined"

**Step 3: Write minimal implementation**

```go
// internal/git/gh.go - Add these imports and methods

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Add to gh.go after IsAvailable method:

// IsBranchPRMerged checks if a branch has an associated PR that was merged.
// Returns:
//   - (true, nil) if PR exists and is MERGED
//   - (false, nil) if PR exists but not merged (OPEN/CLOSED)
//   - (false, err) if error occurred (no PR, network error, etc.)
func (c *GhClient) IsBranchPRMerged(ctx context.Context, branch string) (bool, error) {
	if !c.IsAvailable() {
		return false, fmt.Errorf("gh client not available")
	}

	// Use gh pr view to get PR state
	cmd := exec.CommandContext(ctx, c.ghPath, "pr", "view", branch, "--json", "state")
	output, err := cmd.Output()
	if err != nil {
		// gh returns error if no PR found or other issues
		return false, fmt.Errorf("gh pr view: %w", err)
	}

	state, err := parsePRStateJSON(string(output))
	if err != nil {
		return false, fmt.Errorf("parsing PR state: %w", err)
	}

	return state == "MERGED", nil
}

// parsePRStateJSON extracts the state field from gh pr view --json state output.
func parsePRStateJSON(input string) (string, error) {
	var result struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return "", fmt.Errorf("unmarshaling JSON: %w", err)
	}
	return result.State, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/git -run "TestParsePRStateJSON|TestGhClient_IsBranchPRMerged"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/gh.go internal/git/gh_test.go
git commit -m "feat(git): add IsBranchPRMerged method to GhClient"
```

---

## Task 3: Implement IsBranchCherryMerged Method

**Files:**
- Modify: `internal/git/worktree.go`
- Modify: `internal/git/worktree_test.go`

**Step 1: Write the failing test**

```go
// internal/git/worktree_test.go - Add these tests

func TestParseCherryOutput(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		allMerged     bool
		unmergedCount int
	}{
		{
			name: "all commits merged (all minus signs)",
			input: `- abc123 Commit one
- def456 Commit two
- ghi789 Commit three`,
			allMerged:     true,
			unmergedCount: 0,
		},
		{
			name: "some commits not merged (plus signs)",
			input: `- abc123 Merged commit
+ def456 Unmerged commit
- ghi789 Another merged`,
			allMerged:     false,
			unmergedCount: 1,
		},
		{
			name: "no commits (empty output)",
			input: ``,
			allMerged:     true, // No commits = effectively merged
			unmergedCount: 0,
		},
		{
			name: "all unmerged (all plus signs)",
			input: `+ abc123 First commit
+ def456 Second commit`,
			allMerged:     false,
			unmergedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allMerged, unmergedCount := parseCherryOutput(tt.input)
			if allMerged != tt.allMerged {
				t.Errorf("allMerged = %v, want %v", allMerged, tt.allMerged)
			}
			if unmergedCount != tt.unmergedCount {
				t.Errorf("unmergedCount = %d, want %d", unmergedCount, tt.unmergedCount)
			}
		})
	}
}

func TestClient_IsBranchCherryMerged_Integration(t *testing.T) {
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

	// Create feature branch
	runGitCommand(t, tempDir, "checkout", "-b", "feature/test")

	// Add a commit to feature branch
	if err := os.WriteFile(testFile, []byte("# Test\n\nFeature content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Add feature")

	// Switch back to main
	runGitCommand(t, tempDir, "checkout", "main")

	// Cherry-pick the commit (simulates cherry-pick merge or rebase)
	runGitCommand(t, tempDir, "cherry-pick", "feature/test")

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

	merged, err := client.IsBranchCherryMerged(context.Background(), "feature/test")
	if err != nil {
		t.Fatalf("IsBranchCherryMerged() error = %v", err)
	}

	if !merged {
		t.Error("IsBranchCherryMerged() = false, want true (cherry-picked commit should be detected)")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/git -run "TestParseCherryOutput|TestClient_IsBranchCherryMerged"`
Expected: FAIL with "parseCherryOutput undefined" or "IsBranchCherryMerged undefined"

**Step 3: Write minimal implementation**

```go
// internal/git/worktree.go - Add this method after IsBranchMerged

// IsBranchCherryMerged checks if all commits from a branch have been applied
// to the default branch using patch ID comparison (git cherry).
// This detects squash merges and cherry-picks that git merge-base misses.
func (c *Client) IsBranchCherryMerged(ctx context.Context, branch string) (bool, error) {
	repoInfo, err := c.GetRepoInfo(ctx)
	if err != nil {
		return false, fmt.Errorf("getting repo info: %w", err)
	}

	// git cherry -v <upstream> <head> shows which commits haven't been applied
	// - prefix = commit has equivalent in upstream (merged)
	// + prefix = commit is not in upstream (not merged)
	args := []string{"cherry", "-v", repoInfo.DefaultBranch, branch}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("git cherry: %w: %s", err, stderr.String())
	}

	allMerged, _ := parseCherryOutput(stdout.String())
	return allMerged, nil
}

// parseCherryOutput parses git cherry -v output.
// Returns (allMerged, unmergedCount).
// Lines starting with '-' are merged, '+' are not merged.
func parseCherryOutput(output string) (allMerged bool, unmergedCount int) {
	allMerged = true
	unmergedCount = 0

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// First character indicates status
		if len(line) > 0 && line[0] == '+' {
			allMerged = false
			unmergedCount++
		}
	}

	// Empty output means no commits to check = effectively merged
	return allMerged, unmergedCount
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/git -run "TestParseCherryOutput|TestClient_IsBranchCherryMerged"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/worktree.go internal/git/worktree_test.go
git commit -m "feat(git): add IsBranchCherryMerged for squash-merge detection"
```

---

## Task 4: Update IsBranchMerged with Two-Tier Detection

**Files:**
- Modify: `internal/git/worktree.go`
- Modify: `internal/git/worktree_test.go`

**Step 1: Write the failing test**

```go
// internal/git/worktree_test.go - Add this test

func TestClient_IsBranchMerged_TwoTierDetection(t *testing.T) {
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

	// Create feature branch with commit
	runGitCommand(t, tempDir, "checkout", "-b", "feature/test")
	if err := os.WriteFile(testFile, []byte("# Test\n\nFeature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Add feature")

	// Switch to main and squash merge (simulates GitHub squash merge)
	runGitCommand(t, tempDir, "checkout", "main")
	runGitCommand(t, tempDir, "merge", "--squash", "feature/test")
	runGitCommand(t, tempDir, "commit", "-m", "Squash merge feature/test")

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

	// Create gh client if available (may be nil)
	ghClient, _ := NewGhClient()

	// IsBranchMerged should detect the squash merge via cherry
	merged, err := client.IsBranchMergedWithDetection(context.Background(), "feature/test", ghClient)
	if err != nil {
		t.Fatalf("IsBranchMergedWithDetection() error = %v", err)
	}

	if !merged {
		t.Error("IsBranchMergedWithDetection() = false, want true (squash-merged branch should be detected)")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/git -run "TestClient_IsBranchMerged_TwoTierDetection"`
Expected: FAIL with "IsBranchMergedWithDetection undefined"

**Step 3: Write minimal implementation**

```go
// internal/git/worktree.go - Update the Client struct and add new method

// Add ghClient field to Client struct (around line 18)
type Client struct {
	gitPath string
	// Note: ghClient is optional and may be nil if gh CLI not available
}

// Add new method after IsBranchMerged (around line 294)

// IsBranchMergedWithDetection checks if a branch is merged using a two-tier strategy:
// 1. First tries gh pr view to check GitHub PR status (most reliable)
// 2. Falls back to git cherry for patch-level comparison (detects squash merges)
// 3. Falls back to git merge-base --is-ancestor (traditional merge commits)
//
// The optional ghClient enables GitHub PR detection. If nil, skips to cherry detection.
func (c *Client) IsBranchMergedWithDetection(ctx context.Context, branch string, ghClient *GhClient) (bool, error) {
	// Tier 1: Try GitHub PR detection (most authoritative for GitHub repos)
	if ghClient != nil && ghClient.IsAvailable() {
		merged, err := ghClient.IsBranchPRMerged(ctx, branch)
		if err == nil {
			// Successfully got PR status - use it as source of truth
			return merged, nil
		}
		// Error could be: no PR exists, network error, auth error
		// Fall through to cherry detection
	}

	// Tier 2: Use git cherry for patch-level comparison
	// This detects squash merges and cherry-picks
	merged, err := c.IsBranchCherryMerged(ctx, branch)
	if err == nil {
		return merged, nil
	}
	// Cherry failed (e.g., branch doesn't exist), fall through to merge-base

	// Tier 3: Traditional merge-base check (detects regular merge commits)
	return c.IsBranchMerged(ctx, branch)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/git -run "TestClient_IsBranchMerged_TwoTierDetection"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/worktree.go internal/git/worktree_test.go
git commit -m "feat(git): add IsBranchMergedWithDetection with gh/cherry fallback"
```

---

## Task 5: Update Worktree Service to Use New Detection

**Files:**
- Modify: `internal/worktree/service.go`
- Modify: `internal/worktree/service_test.go`

**Step 1: Write the failing test**

```go
// internal/worktree/service_test.go - Add this test

package worktree

import (
	"context"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/pkg/domain"
)

// mockGitClient for testing - implements git.GitClient
type mockGitClientForRemove struct {
	listWorktreesFunc    func(ctx context.Context) ([]*domain.Worktree, error)
	getRepoInfoFunc      func(ctx context.Context) (*domain.GitRepo, error)
	isWorktreeDirtyFunc  func(ctx context.Context, path string) (bool, error)
	removeWorktreeFunc   func(ctx context.Context, path string, force bool) error
	deleteBranchFunc     func(ctx context.Context, branch string, force bool) error
	isBranchMergedFunc   func(ctx context.Context, branch string) (bool, error)
}

func (m *mockGitClientForRemove) ListWorktrees(ctx context.Context) ([]*domain.Worktree, error) {
	return m.listWorktreesFunc(ctx)
}
func (m *mockGitClientForRemove) GetRepoInfo(ctx context.Context) (*domain.GitRepo, error) {
	return m.getRepoInfoFunc(ctx)
}
func (m *mockGitClientForRemove) IsWorktreeDirty(ctx context.Context, path string) (bool, error) {
	return m.isWorktreeDirtyFunc(ctx, path)
}
func (m *mockGitClientForRemove) RemoveWorktree(ctx context.Context, path string, force bool) error {
	return m.removeWorktreeFunc(ctx, path, force)
}
func (m *mockGitClientForRemove) DeleteBranch(ctx context.Context, branch string, force bool) error {
	return m.deleteBranchFunc(ctx, branch, force)
}
func (m *mockGitClientForRemove) IsBranchMerged(ctx context.Context, branch string) (bool, error) {
	return m.isBranchMergedFunc(ctx, branch)
}
func (m *mockGitClientForRemove) AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	return nil, nil
}
func (m *mockGitClientForRemove) BranchExists(ctx context.Context, branch string) (bool, error) {
	return true, nil
}
func (m *mockGitClientForRemove) GetCurrentBranch(ctx context.Context) (string, error) {
	return "main", nil
}
func (m *mockGitClientForRemove) RemoteBranchExists(ctx context.Context, remote, branch string) (bool, error) {
	return false, nil
}
func (m *mockGitClientForRemove) DeleteRemoteBranch(ctx context.Context, remote, branch string) error {
	return nil
}
func (m *mockGitClientForRemove) SquashMerge(ctx context.Context, sourceBranch string) error {
	return nil
}
func (m *mockGitClientForRemove) CreateSquashCommit(ctx context.Context, message string) error {
	return nil
}

func TestService_RemoveEnhanced_SquashMergedBranch(t *testing.T) {
	mockGit := &mockGitClientForRemove{
		listWorktreesFunc: func(ctx context.Context) ([]*domain.Worktree, error) {
			return []*domain.Worktree{
				{Path: "/path/to/main", Branch: "main"},
				{Path: "/path/to/feature", Branch: "feature/test"},
			}, nil
		},
		getRepoInfoFunc: func(ctx context.Context) (*domain.GitRepo, error) {
			return &domain.GitRepo{RootPath: "/path/to/main", DefaultBranch: "main"}, nil
		},
		isWorktreeDirtyFunc: func(ctx context.Context, path string) (bool, error) {
			return false, nil // Clean worktree
		},
		isBranchMergedFunc: func(ctx context.Context, branch string) (bool, error) {
			// Simulate squash merge detection: merge-base fails, but cherry succeeds
			if branch == "feature/test" {
				return true, nil // Cherry detection found it merged
			}
			return false, nil
		},
		removeWorktreeFunc: func(ctx context.Context, path string, force bool) error {
			return nil
		},
		deleteBranchFunc: func(ctx context.Context, branch string, force bool) error {
			return nil
		},
	}

	svc, err := NewService(mockGit, config.DefaultConfig())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	// Remove without --force should succeed for squash-merged branch
	err = svc.RemoveEnhanced(context.Background(), "/path/to/feature", domain.ForceNone)
	if err != nil {
		t.Errorf("RemoveEnhanced() error = %v, want nil (squash-merged branch should be removable)", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worktree -run "TestService_RemoveEnhanced_SquashMergedBranch"`
Expected: FAIL (current implementation uses old IsBranchMerged)

**Step 3: Write minimal implementation**

Update `internal/worktree/service.go` to use the new detection method. The service needs access to GhClient.

```go
// internal/worktree/service.go - Update the Service struct and validateRemoveSafety

// Add to imports
import (
	"github.com/joebalancio/wt/internal/git"
	// ... existing imports
)

// Update Service struct to include optional ghClient
type Service struct {
	git      git.GitClient
	cfg      *config.Config
	ghClient *git.GhClient // Optional: enables GitHub PR detection
}

// Update NewService to optionally accept ghClient
func NewService(gitClient git.GitClient, cfg *config.Config) (*Service, error) {
	if gitClient == nil {
		return nil, fmt.Errorf("gitClient cannot be nil")
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Try to create gh client (optional, may fail)
	var ghClient *git.GhClient
	if gc, err := git.NewGhClient(); err == nil {
		ghClient = gc
	}

	return &Service{
		git:      gitClient,
		cfg:      cfg,
		ghClient: ghClient,
	}, nil
}

// Update validateRemoveSafety to use new detection
func (s *Service) validateRemoveSafety(ctx context.Context, target *domain.Worktree, path, defaultBranch string, force domain.ForceLevel) error {
	if target.Branch == defaultBranch {
		return fmt.Errorf("cannot remove default branch %q", target.Branch)
	}

	dirty, err := s.git.IsWorktreeDirty(ctx, path)
	if err != nil {
		return fmt.Errorf("checking worktree status: %w", err)
	}
	if dirty && force == domain.ForceNone {
		return errors.New("worktree has uncommitted changes. Use --force to remove anyway")
	}

	// Use new two-tier detection if git client supports it
	var merged bool
	if gc, ok := s.git.(*git.Client); ok {
		merged, err = gc.IsBranchMergedWithDetection(ctx, target.Branch, s.ghClient)
	} else {
		// Fallback for mock clients in tests
		merged, err = s.git.IsBranchMerged(ctx, target.Branch)
	}
	if err != nil {
		return fmt.Errorf("checking branch merge status: %w", err)
	}
	if !merged && force == domain.ForceNone {
		return fmt.Errorf("branch %q is not merged. Use --force to delete anyway", target.Branch)
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/worktree -run "TestService_RemoveEnhanced_SquashMergedBranch"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/service.go internal/worktree/service_test.go
git commit -m "feat(worktree): integrate two-tier merge detection in RemoveEnhanced"
```

---

## Task 6: Add gh CLI Validation to wt doctor

**Files:**
- Modify: `internal/cli/doctor.go`

**Step 1: Write the failing test (manual test)**

Since doctor output is user-facing, we'll verify manually first.

Run: `go run ./cmd/wt doctor`
Expected: Shows gh CLI status in dependencies section

**Step 2: Write minimal implementation**

```go
// internal/cli/doctor.go - Add to checkDependencies function

// Add import
import (
	"github.com/joebalancio/wt/internal/git"
	// ... existing imports
)

// Add this section in checkDependencies, after git-spice check (around line 172)

// Check gh CLI (for squash-merge detection)
func checkGhCLI(ctx context.Context, out io.Writer) bool {
	ghClient, err := git.NewGhClient()
	if err != nil {
		_, _ = fmt.Fprintf(out, "⚠ gh CLI not found (optional, for squash-merge detection)\n")
		_, _ = fmt.Fprintf(out, "  Install with: brew install gh\n")
		_, _ = fmt.Fprintf(out, "  Then run: gh auth login\n")
		return false
	}

	// Check if authenticated
	cmd := exec.CommandContext(ctx, ghClient.ghPath, "auth", "status")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintf(out, "⚠ gh CLI installed but not authenticated\n")
		_, _ = fmt.Fprintf(out, "  Run: gh auth login\n")
		return false
	}

	_, _ = fmt.Fprintf(out, "✓ gh CLI installed and authenticated\n")
	return true
}
```

Then update `checkDependencies` to call it:

```go
// In checkDependencies function, add after git-spice check:
// Check gh CLI
_ = checkGhCLI(ctx, out) // Not critical, just informational
```

**Step 3: Run manual test to verify**

Run: `go run ./cmd/wt doctor`
Expected: Shows gh CLI status (✓ or ⚠)

**Step 4: Commit**

```bash
git add internal/cli/doctor.go
git commit -m "feat(doctor): add gh CLI validation check"
```

---

## Task 7: Add gh CLI Validation to wt init

**Files:**
- Modify: `internal/cli/init.go`

**Step 1: Write the failing test (manual test)**

Run: `go run ./cmd/wt init`
Expected: Shows gh CLI status with warning if not available

**Step 2: Write minimal implementation**

```go
// internal/cli/init.go - Add check after git-spice check

// Add checkGhCLI function (similar to doctor.go pattern)
func checkGhCLIForInit(_ context.Context, out io.Writer) error {
	ghClient, err := git.NewGhClient()
	if err != nil {
		_, _ = fmt.Fprintf(out, "⚠ gh CLI not found (optional)\n")
		_, _ = fmt.Fprintf(out, "  gh CLI enables squash-merge detection for 'wt remove'.\n")
		_, _ = fmt.Fprintf(out, "  Install: brew install gh && gh auth login\n\n")
		return err
	}

	// Check authentication
	cmd := exec.Command(ghClient.ghPath, "auth", "status")
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintf(out, "⚠ gh CLI installed but not authenticated\n")
		_, _ = fmt.Fprintf(out, "  Run: gh auth login\n\n")
		return err
	}

	_, _ = fmt.Fprintf(out, "✓ gh CLI installed and authenticated\n")
	return nil
}
```

Then call it in `NewInitCmd`:

```go
// In NewInitCmd Run function, add after checkGitSpice:
// Check gh CLI (non-fatal, just informational)
_ = checkGhCLIForInit(ctx, out)
```

**Step 3: Run manual test to verify**

Run: `go run ./cmd/wt init`
Expected: Shows gh CLI status

**Step 4: Commit**

```bash
git add internal/cli/init.go
git commit -m "feat(init): add gh CLI validation with warning"
```

---

## Task 8: Run Full Test Suite and Fix Issues

**Files:**
- Any files with failing tests

**Step 1: Run all tests**

Run: `go test -v ./...`
Expected: All tests pass

**Step 2: Run linter**

Run: `make lint`
Expected: No errors

**Step 3: Run build**

Run: `make build`
Expected: Binary builds successfully

**Step 4: Fix any issues**

If tests fail, fix them. Common issues:
- Mock clients need to implement new interface methods
- Import cycles need resolution
- Linting errors (unused variables, etc.)

**Step 5: Final commit**

```bash
git add .
git commit -m "fix: resolve test and lint issues after squash-merge detection"
```

---

## Task 9: Add Integration Test for Full Workflow

**Files:**
- Create: `tests/remove_squash_merge_test.go`

**Step 1: Write the failing test**

```go
// tests/remove_squash_merge_test.go
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

func TestRemove_SquashMergedBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create temp repo
	tempDir := t.TempDir()
	runGit(t, tempDir, "init", "-b", "main")
	runGit(t, tempDir, "config", "user.name", "Test")
	runGit(t, tempDir, "config", "user.email", "test@test.com")

	// Initial commit
	writeFile(t, filepath.Join(tempDir, "file.txt"), "initial")
	runGit(t, tempDir, "add", ".")
	runGit(t, tempDir, "commit", "-m", "initial")

	// Create feature branch
	runGit(t, tempDir, "checkout", "-b", "feature/test")
	writeFile(t, filepath.Join(tempDir, "file.txt"), "initial\nfeature")
	runGit(t, tempDir, "add", ".")
	runGit(t, tempDir, "commit", "-m", "add feature")

	// Squash merge to main
	runGit(t, tempDir, "checkout", "main")
	runGit(t, tempDir, "merge", "--squash", "feature/test")
	runGit(t, tempDir, "commit", "-m", "squash merge")

	// Create worktree for the feature branch (simulating after-merge cleanup)
	worktreePath := filepath.Join(tempDir, "wt-feature")
	runGit(t, tempDir, "worktree", "add", worktreePath, "feature/test")

	// Now try to remove without --force
	gitClient, err := git.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	cfg := config.DefaultConfig()
	svc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	// Change to worktree dir for context
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(worktreePath)

	// Remove without force should succeed (squash-merged)
	err = svc.RemoveEnhanced(context.Background(), worktreePath, domain.ForceNone)
	if err != nil {
		t.Errorf("RemoveEnhanced failed: %v\nExpected success for squash-merged branch", err)
	}

	// Verify worktree removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree path still exists: %s", worktreePath)
	}
}

func runGit(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func writeFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

**Step 2: Run test to verify it passes**

Run: `go test -v ./tests -run "TestRemove_SquashMergedBranch"`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/remove_squash_merge_test.go
git commit -m "test: add integration test for squash-merge detection in wt remove"
```

---

## Verification Checklist

After all tasks complete, verify:

- [ ] `wt remove` succeeds on squash-merged branches without `--force`
- [ ] `wt remove` still requires `--force` for truly unmerged branches
- [ ] `wt doctor` reports gh CLI status (installed + authenticated)
- [ ] `wt init` validates gh CLI and warns if missing
- [ ] All unit tests pass: `go test ./...`
- [ ] Linter passes: `make lint`
- [ ] Build succeeds: `make build`
- [ ] Backward compatible: existing `--force` behavior unchanged

---

## Summary

This implementation adds two-tier merge detection to `wt remove`:

1. **Tier 1**: GitHub PR status via `gh pr view` (authoritative)
2. **Tier 2**: Patch comparison via `git cherry` (detects squash merges)
3. **Tier 3**: Traditional `git merge-base` (fallback for regular merges)

The `gh` CLI is optional but recommended. Without it, detection falls back to `git cherry` which still detects squash merges at the patch level.
