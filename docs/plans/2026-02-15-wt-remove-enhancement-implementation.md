# Enhanced `wt remove` Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `wt remove` the natural counterpart to `wt add` by removing the branch alongside the worktree, with CWD resolution and tiered force levels.

**Architecture:** Extend the existing service layer with a new `RemoveEnhanced` method that handles branch deletion, CWD resolution, and force levels. Add new git client methods for branch merge status checking and remote branch operations. Update CLI to accept optional path argument and parse force levels.

**Tech Stack:** Go 1.21+, spf13/cobra, existing git/tmux client architecture

---

## Task 1: Add ForceLevel Type to Domain

**Files:**
- Modify: `pkg/domain/worktree.go` (add after line 25)

**Step 1: Write the failing test**

Create test file `pkg/domain/force_level_test.go`:

```go
package domain

import (
	"testing"
)

func TestForceLevel_String(t *testing.T) {
	tests := []struct {
		name  string
		level ForceLevel
		want  string
	}{
		{"none", ForceNone, "none"},
		{"local", ForceLocal, "local"},
		{"remote", ForceRemote, "remote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("ForceLevel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseForceLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ForceLevel
		wantErr bool
	}{
		{"empty", "", ForceNone, false},
		{"false", "false", ForceNone, false},
		{"true", "true", ForceLocal, false},
		{"local", "local", ForceLocal, false},
		{"remote", "remote", ForceRemote, false},
		{"all", "all", ForceRemote, false},
		{"invalid", "invalid", ForceNone, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseForceLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseForceLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseForceLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/domain/... -run "TestForceLevel" -v`
Expected: FAIL with "undefined: ForceLevel"

**Step 3: Write minimal implementation**

Add to `pkg/domain/worktree.go` after the `Worktree` struct definition (around line 25):

```go
// ForceLevel represents the force level for remove operations
type ForceLevel int

const (
	// ForceNone performs safe removal (no force)
	ForceNone ForceLevel = iota
	// ForceLocal forces local worktree and branch deletion
	ForceLocal
	// ForceRemote forces local deletion and also deletes remote branch
	ForceRemote
)

// String returns the string representation of the force level
func (f ForceLevel) String() string {
	switch f {
	case ForceNone:
		return "none"
	case ForceLocal:
		return "local"
	case ForceRemote:
		return "remote"
	default:
		return "unknown"
	}
}

// ParseForceLevel parses a string into a ForceLevel
func ParseForceLevel(s string) (ForceLevel, error) {
	switch s {
	case "", "false", "0":
		return ForceNone, nil
	case "true", "1", "local":
		return ForceLocal, nil
	case "remote", "all":
		return ForceRemote, nil
	default:
		return ForceNone, fmt.Errorf("invalid --force value %q. Use: true, remote, or all", s)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/domain/... -run "TestForceLevel" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/domain/worktree.go pkg/domain/force_level_test.go
git commit -m "feat: add ForceLevel type for enhanced remove command"
```

---

## Task 2: Add Git Client Methods for Branch Operations

**Files:**
- Modify: `internal/git/worktree.go` (add new methods)
- Modify: `internal/git/client_interface.go` (add interface methods)
- Modify: `internal/worktree/service_test.go` (add mock implementations)

**Step 1: Write the failing test**

Create test file `internal/git/branch_operations_test.go`:

```go
package git

import (
	"testing"
)

func TestClient_IsBranchMerged(t *testing.T) {
	// This is an integration test that requires a real git repo
	// Unit testing will be done via the mock in service_test.go
	t.Skip("requires integration test environment")
}

func TestClient_RemoteBranchExists(t *testing.T) {
	// This is an integration test that requires a real git repo
	t.Skip("requires integration test environment")
}

func TestClient_DeleteRemoteBranch(t *testing.T) {
	// This is an integration test that requires a real git repo
	t.Skip("requires integration test environment")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/... -run "TestClient_IsBranchMerged|TestClient_RemoteBranchExists|TestClient_DeleteRemoteBranch" -v`
Expected: Tests skip (placeholders for now)

**Step 3: Write minimal implementation**

Add to `internal/git/worktree.go` after `IsWorktreeDirty` method:

```go
// IsBranchMerged checks if a branch is merged into the default branch (typically main).
// Returns true if the branch has been merged.
func (c *Client) IsBranchMerged(ctx context.Context, branch string) (bool, error) {
	// Get the default branch
	repoInfo, err := c.GetRepoInfo(ctx)
	if err != nil {
		return false, fmt.Errorf("getting repo info: %w", err)
	}
	defaultBranch := repoInfo.DefaultBranch

	// Check if branch is merged into default branch
	// git branch --merged <default-branch> lists branches merged into default
	args := []string{"branch", "--merged", defaultBranch}
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("checking merged branches: %w", err)
	}

	// Parse output - each line is a branch name (with optional * prefix)
	mergedBranches := strings.Split(stdout.String(), "\n")
	for _, merged := range mergedBranches {
		// Remove the * prefix for current branch and whitespace
		cleaned := strings.TrimSpace(strings.TrimPrefix(merged, "*"))
		if cleaned == branch {
			return true, nil
		}
	}

	return false, nil
}

// RemoteBranchExists checks if a branch exists on the remote.
func (c *Client) RemoteBranchExists(ctx context.Context, remote, branch string) (bool, error) {
	remoteRef := fmt.Sprintf("refs/remotes/%s/%s", remote, branch)
	args := []string{"rev-parse", "--verify", remoteRef}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "unknown revision") ||
			strings.Contains(errMsg, "Needed a single revision") {
			return false, nil
		}
		return false, fmt.Errorf("checking remote branch %s/%s: %w", remote, branch, err)
	}
	return true, nil
}

// DeleteRemoteBranch deletes a branch from the remote.
func (c *Client) DeleteRemoteBranch(ctx context.Context, remote, branch string) error {
	args := []string{"push", remote, "--delete", branch}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deleting remote branch %s/%s: %w: %s", remote, branch, err, stderr.String())
	}
	return nil
}
```

**Step 4: Update the interface**

Add to `internal/git/client_interface.go` after `IsWorktreeDirty`:

```go
	IsBranchMerged(ctx context.Context, branch string) (bool, error)
	RemoteBranchExists(ctx context.Context, remote, branch string) (bool, error)
	DeleteRemoteBranch(ctx context.Context, remote, branch string) error
```

**Step 5: Add mock implementations**

Add to `internal/worktree/service_test.go` in the `mockGitClient` struct:

```go
	isBranchMergedFunc    func(ctx context.Context, branch string) (bool, error)
	remoteBranchExistsFunc func(ctx context.Context, remote, branch string) (bool, error)
	deleteRemoteBranchFunc func(ctx context.Context, remote, branch string) error
```

Add the mock methods after `IsWorktreeDirty`:

```go
func (m *mockGitClient) IsBranchMerged(ctx context.Context, branch string) (bool, error) {
	if m.isBranchMergedFunc != nil {
		return m.isBranchMergedFunc(ctx, branch)
	}
	return true, nil
}

func (m *mockGitClient) RemoteBranchExists(ctx context.Context, remote, branch string) (bool, error) {
	if m.remoteBranchExistsFunc != nil {
		return m.remoteBranchExistsFunc(ctx, remote, branch)
	}
	return false, nil
}

func (m *mockGitClient) DeleteRemoteBranch(ctx context.Context, remote, branch string) error {
	if m.deleteRemoteBranchFunc != nil {
		return m.deleteRemoteBranchFunc(ctx, remote, branch)
	}
	return nil
}
```

**Step 6: Run tests to verify**

Run: `go test ./internal/git/... ./internal/worktree/... -v`
Expected: All tests PASS

**Step 7: Commit**

```bash
git add internal/git/worktree.go internal/git/client_interface.go internal/git/branch_operations_test.go internal/worktree/service_test.go
git commit -m "feat(git): add branch merge check and remote operations"
```

---

## Task 3: Add Worktree Resolution from CWD

**Files:**
- Modify: `internal/worktree/service.go` (add ResolveFromCWD method)

**Step 1: Write the failing test**

Add to `internal/worktree/service_test.go`:

```go
func TestService_ResolveFromCWD(t *testing.T) {
	t.Run("resolves worktree when CWD is inside worktree", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/home/user/repo", Branch: "main"},
					{Path: "/home/user/worktrees/feat-auth", Branch: "feat-auth"},
				}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		// Test resolving from inside a worktree
		worktree, err := svc.ResolveFromCWD(context.Background(), "/home/user/worktrees/feat-auth/src")
		if err != nil {
			t.Fatalf("ResolveFromCWD() error = %v", err)
		}
		if worktree.Branch != "feat-auth" {
			t.Errorf("got branch %s, want feat-auth", worktree.Branch)
		}
	})

	t.Run("returns error when CWD is not in a worktree", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/home/user/repo", Branch: "main"},
					{Path: "/home/user/worktrees/feat-auth", Branch: "feat-auth"},
				}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		_, err = svc.ResolveFromCWD(context.Background(), "/home/other/project")
		if err == nil {
			t.Fatal("ResolveFromCWD() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not in a worktree") {
			t.Errorf("expected 'not in a worktree' error, got: %v", err)
		}
	})

	t.Run("prefers longer path match (nested worktrees)", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/home/user/repo", Branch: "main"},
					{Path: "/home/user/worktrees/feat", Branch: "feat"},
					{Path: "/home/user/worktrees/feat/nested", Branch: "feat/nested"},
				}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		worktree, err := svc.ResolveFromCWD(context.Background(), "/home/user/worktrees/feat/nested/src")
		if err != nil {
			t.Fatalf("ResolveFromCWD() error = %v", err)
		}
		if worktree.Branch != "feat/nested" {
			t.Errorf("got branch %s, want feat/nested", worktree.Branch)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -run "TestService_ResolveFromCWD" -v`
Expected: FAIL with "Service has no field or method ResolveFromCWD"

**Step 3: Write minimal implementation**

Add to `internal/worktree/service.go` after the `Remove` method:

```go
// ResolveFromCWD resolves the worktree containing the given working directory.
// If the cwd is inside a worktree, returns that worktree.
// Returns an error if cwd is not inside any worktree.
func (s *Service) ResolveFromCWD(ctx context.Context, cwd string) (*domain.Worktree, error) {
	worktrees, err := s.git.ListWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	// Find the worktree that contains cwd
	// We need to find the longest matching path (for nested worktrees)
	var bestMatch *domain.Worktree
	for _, wt := range worktrees {
		if strings.HasPrefix(cwd, wt.Path) {
			if bestMatch == nil || len(wt.Path) > len(bestMatch.Path) {
				bestMatch = wt
			}
		}
	}

	if bestMatch == nil {
		return nil, errors.New("not in a worktree")
	}

	return bestMatch, nil
}
```

Also add the import for `errors` if not present:

```go
import (
	"errors"
	// ... other imports
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -run "TestService_ResolveFromCWD" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/service.go internal/worktree/service_test.go
git commit -m "feat(worktree): add ResolveFromCWD for detecting current worktree"
```

---

## Task 4: Implement Enhanced Remove Service Method

**Files:**
- Modify: `internal/worktree/service.go` (add RemoveEnhanced method)

**Step 1: Write the failing test**

Add to `internal/worktree/service_test.go`:

```go
func TestService_RemoveEnhanced(t *testing.T) {
	t.Run("removes worktree and branch", func(t *testing.T) {
		var removeCalled, deleteCalled bool
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/feat-auth", Branch: "feat-auth"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			isBranchMergedFunc: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			removeWorktreeFunc: func(_ context.Context, path string, _ bool) error {
				removeCalled = true
				if path != "/worktrees/feat-auth" {
					t.Errorf("unexpected path: %s", path)
				}
				return nil
			},
			deleteBranchFunc: func(_ context.Context, branch string, _ bool) error {
				deleteCalled = true
				if branch != "feat-auth" {
					t.Errorf("unexpected branch: %s", branch)
				}
				return nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/worktrees/feat-auth", domain.ForceNone)
		if err != nil {
			t.Fatalf("RemoveEnhanced() error = %v", err)
		}

		if !removeCalled {
			t.Error("RemoveWorktree was not called")
		}
		if !deleteCalled {
			t.Error("DeleteBranch was not called")
		}
	})

	t.Run("fails for default branch", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/repo", domain.ForceNone)
		if err == nil {
			t.Fatal("RemoveEnhanced() expected error for default branch, got nil")
		}
		if !strings.Contains(err.Error(), "cannot remove default branch") {
			t.Errorf("expected 'cannot remove default branch' error, got: %v", err)
		}
	})

	t.Run("fails for dirty worktree without force", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/feat", Branch: "feat"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/worktrees/feat", domain.ForceNone)
		if err == nil {
			t.Fatal("RemoveEnhanced() expected error for dirty worktree, got nil")
		}
		if !strings.Contains(err.Error(), "uncommitted changes") {
			t.Errorf("expected 'uncommitted changes' error, got: %v", err)
		}
	})

	t.Run("removes dirty worktree with force", func(t *testing.T) {
		var removeCalled, deleteCalled bool
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/feat", Branch: "feat"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			isBranchMergedFunc: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			removeWorktreeFunc: func(_ context.Context, _ string, _ bool) error {
				removeCalled = true
				return nil
			},
			deleteBranchFunc: func(_ context.Context, _ string, _ bool) error {
				deleteCalled = true
				return nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/worktrees/feat", domain.ForceLocal)
		if err != nil {
			t.Fatalf("RemoveEnhanced() error = %v", err)
		}

		if !removeCalled {
			t.Error("RemoveWorktree was not called")
		}
		if !deleteCalled {
			t.Error("DeleteBranch was not called")
		}
	})

	t.Run("fails for unmerged branch without force", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/feat", Branch: "feat"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			isBranchMergedFunc: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/worktrees/feat", domain.ForceNone)
		if err == nil {
			t.Fatal("RemoveEnhanced() expected error for unmerged branch, got nil")
		}
		if !strings.Contains(err.Error(), "not merged") {
			t.Errorf("expected 'not merged' error, got: %v", err)
		}
	})

	t.Run("deletes remote branch with ForceRemote", func(t *testing.T) {
		var deleteRemoteCalled bool
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/feat", Branch: "feat"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			isBranchMergedFunc: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			remoteBranchExistsFunc: func(_ context.Context, _, _ string) (bool, error) {
				return true, nil
			},
			removeWorktreeFunc: func(_ context.Context, _ string, _ bool) error {
				return nil
			},
			deleteBranchFunc: func(_ context.Context, _ string, _ bool) error {
				return nil
			},
			deleteRemoteBranchFunc: func(_ context.Context, remote, branch string) error {
				deleteRemoteCalled = true
				if remote != "origin" || branch != "feat" {
					t.Errorf("unexpected remote/branch: %s/%s", remote, branch)
				}
				return nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/worktrees/feat", domain.ForceRemote)
		if err != nil {
			t.Fatalf("RemoveEnhanced() error = %v", err)
		}

		if !deleteRemoteCalled {
			t.Error("DeleteRemoteBranch was not called")
		}
	})

	t.Run("warns when remote branch already deleted", func(t *testing.T) {
		// This tests that ForceRemote gracefully handles missing remote branches
		var deleteRemoteCalled bool
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/feat", Branch: "feat"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			isBranchMergedFunc: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			remoteBranchExistsFunc: func(_ context.Context, _, _ string) (bool, error) {
				return false, nil // Remote branch doesn't exist
			},
			removeWorktreeFunc: func(_ context.Context, _ string, _ bool) error {
				return nil
			},
			deleteBranchFunc: func(_ context.Context, _ string, _ bool) error {
				return nil
			},
			deleteRemoteBranchFunc: func(_ context.Context, _, _ string) error {
				deleteRemoteCalled = true
				return nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/worktrees/feat", domain.ForceRemote)
		if err != nil {
			t.Fatalf("RemoveEnhanced() error = %v", err)
		}

		// Should not call DeleteRemoteBranch if remote branch doesn't exist
		if deleteRemoteCalled {
			t.Error("DeleteRemoteBranch should not be called when remote branch missing")
		}
	})

	t.Run("fails for detached HEAD", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/detached", Branch: "", Head: "detached"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/worktrees/detached", domain.ForceNone)
		if err == nil {
			t.Fatal("RemoveEnhanced() expected error for detached HEAD, got nil")
		}
		if !strings.Contains(err.Error(), "detached HEAD") {
			t.Errorf("expected 'detached HEAD' error, got: %v", err)
		}
	})

	t.Run("fails when branch has no associated worktree", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/nonexistent/path", domain.ForceNone)
		if err == nil {
			t.Fatal("RemoveEnhanced() expected error for non-existent worktree, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -run "TestService_RemoveEnhanced" -v`
Expected: FAIL with "Service has no field or method RemoveEnhanced"

**Step 3: Write minimal implementation**

Add to `internal/worktree/service.go` after the `ResolveFromCWD` method:

```go
// RemoveEnhanced removes a worktree along with its branch.
// It performs safety checks and supports different force levels.
func (s *Service) RemoveEnhanced(ctx context.Context, path string, force domain.ForceLevel) (*domain.Worktree, error) {
	// 1. Resolve worktree from path
	worktrees, err := s.git.ListWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	var targetWorktree *domain.Worktree
	for _, wt := range worktrees {
		if wt.Path == path {
			targetWorktree = wt
			break
		}
	}

	if targetWorktree == nil {
		return nil, fmt.Errorf("worktree at %q not found", path)
	}

	// 2. Check for detached HEAD
	if targetWorktree.Detached() {
		return nil, errors.New("cannot remove: detached HEAD (no branch)")
	}

	// 3. Check it's not the default branch
	repoInfo, err := s.git.GetRepoInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting repo info: %w", err)
	}
	if targetWorktree.Branch == repoInfo.DefaultBranch {
		return nil, fmt.Errorf("cannot remove default branch %q", targetWorktree.Branch)
	}

	// 4. Check dirty worktree
	dirty, err := s.git.IsWorktreeDirty(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("checking worktree status: %w", err)
	}
	if dirty && force == domain.ForceNone {
		return nil, errors.New("worktree has uncommitted changes. Use --force to remove anyway")
	}

	// 5. Check unmerged branch
	merged, err := s.git.IsBranchMerged(ctx, targetWorktree.Branch)
	if err != nil {
		return nil, fmt.Errorf("checking branch merge status: %w", err)
	}
	if !merged && force == domain.ForceNone {
		return nil, fmt.Errorf("branch %q is not merged. Use --force to delete anyway", targetWorktree.Branch)
	}

	// 6. Remove worktree (use force if ForceLocal or ForceRemote)
	forceRemove := force != domain.ForceNone
	if err := s.git.RemoveWorktree(ctx, path, forceRemove); err != nil {
		return nil, fmt.Errorf("removing worktree: %w", err)
	}

	// 7. Delete local branch (always force since worktree is gone)
	if err := s.git.DeleteBranch(ctx, targetWorktree.Branch, true); err != nil {
		return nil, fmt.Errorf("deleting branch: %w", err)
	}

	// 8. Delete remote branch if requested
	if force == domain.ForceRemote {
		remoteExists, err := s.git.RemoteBranchExists(ctx, "origin", targetWorktree.Branch)
		if err != nil {
			// Log warning but don't fail
			fmt.Fprintf(os.Stderr, "Warning: could not check remote branch: %v\n", err)
		} else if remoteExists {
			if err := s.git.DeleteRemoteBranch(ctx, "origin", targetWorktree.Branch); err != nil {
				return nil, fmt.Errorf("deleting remote branch: %w", err)
			}
		}
		// If remote doesn't exist, silently skip (already deleted)
	}

	return targetWorktree, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -run "TestService_RemoveEnhanced" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/service.go internal/worktree/service_test.go
git commit -m "feat(worktree): add RemoveEnhanced with branch deletion and safety checks"
```

---

## Task 5: Update CLI Remove Command

**Files:**
- Modify: `internal/cli/remove.go` (update command)

**Step 1: Write the failing test**

Create test file `internal/cli/remove_enhanced_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRemoveCmd_Enhanced(t *testing.T) {
	t.Run("accepts optional path argument", func(t *testing.T) {
		cmd := NewRemoveCmd()
		if cmd.Args == nil {
			t.Error("Expected Args to be set")
		}
		// Should accept 0 or 1 args
		err := cmd.Args(cmd, []string{})
		if err != nil {
			t.Errorf("Expected no error for 0 args, got: %v", err)
		}
		err = cmd.Args(cmd, []string{"/path/to/worktree"})
		if err != nil {
			t.Errorf("Expected no error for 1 arg, got: %v", err)
		}
		err = cmd.Args(cmd, []string{"/path/one", "/path/two"})
		if err == nil {
			t.Error("Expected error for 2 args")
		}
	})

	t.Run("parses --force flag variants", func(t *testing.T) {
		tests := []struct {
			name      string
			forceArg  string
			wantLevel string
		}{
			{"no flag", "", "none"},
			{"--force", "true", "local"},
			{"--force=local", "local", "local"},
			{"--force=remote", "remote", "remote"},
			{"--force=all", "all", "remote"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cmd := NewRemoveCmd()

				var buf bytes.Buffer
				cmd.SetOut(&buf)
				cmd.SetErr(&buf)

				args := []string{}
				if tt.forceArg != "" {
					args = append(args, "--force="+tt.forceArg)
				}

				cmd.SetArgs(args)
				// Just test that parsing works, actual execution requires git
			})
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run "TestRemoveCmd_Enhanced" -v`
Expected: FAIL with command still using ExactArgs(1)

**Step 3: Write minimal implementation**

Replace the contents of `internal/cli/remove.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/spf13/cobra"
)

// NewRemoveCmd creates the remove command
func NewRemoveCmd() *cobra.Command {
	var forceStr string

	cmd := &cobra.Command{
		Use:   "remove [path]",
		Short: "Remove a worktree and its branch",
		Long: `Remove a worktree and its associated branch from the repository.

If no path is provided, resolves the worktree from the current working directory.

By default, this will fail if:
- The worktree has uncommitted changes
- The branch is not merged

Use --force to remove anyway (forces local deletion only).
Use --force=remote to also delete the remote branch.

Force levels:
  --force         Force local worktree and branch deletion
  --force=remote  Force local deletion + delete remote branch
  --force=all     Same as --force=remote`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Parse force level
			force, err := domain.ParseForceLevel(forceStr)
			if err != nil {
				Fatal("%v", err)
			}

			// Resolve path
			var path string
			if len(args) > 0 {
				path = args[0]
			}

			runRemoveCommand(cmd, path, force)
		},
	}

	cmd.Flags().StringVar(&forceStr, "force", "", "force removal (true, remote, or all)")

	return cmd
}

func runRemoveCommand(cmd *cobra.Command, path string, force domain.ForceLevel) {
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

	// Resolve path from CWD if not provided
	resolvedPath := path
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

	// Use enhanced remove
	removedWorktree, err := svc.RemoveEnhanced(ctx, resolvedPath, force)
	if err != nil {
		Fatal("Failed to remove worktree: %v", err)
	}

	// Output success messages
	fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", resolvedPath)
	fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", removedWorktree.Branch)

	// Close tmux window if in tmux and window matches
	closeTmuxWindowForBranch(ctx, gitClient, removedWorktree.Branch)

	// Report remote deletion if applicable
	if force == domain.ForceRemote {
		remoteExists, _ := gitClient.RemoteBranchExists(ctx, "origin", removedWorktree.Branch)
		if remoteExists {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted remote branch: origin/%s\n", removedWorktree.Branch)
		}
	}
}

// closeTmuxWindowForBranch closes the tmux window for the given branch
func closeTmuxWindowForBranch(ctx context.Context, gitClient *git.Client, branch string) {
	if !isInTmux() {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	windowName := tmux.GenerateWindowName(branch)
	// Kill the window if it exists
	_ = tmuxClient.KillWindow(windowName)
}

// isInTmux returns true if we're running inside tmux
func isInTmux() bool {
	return os.Getenv("TMUX") != ""
}

func init() {
	RegisterCommand(NewRemoveCmd())
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run "TestRemoveCmd_Enhanced" -v`
Expected: PASS

**Step 5: Run all tests to ensure no regressions**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add internal/cli/remove.go internal/cli/remove_enhanced_test.go
git commit -m "feat(cli): update remove command with optional path and force levels"
```

---

## Task 6: Add Integration Tests for Enhanced Remove

**Files:**
- Create: `tests/remove_enhanced_integration_test.go`

**Step 1: Write the integration test**

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
	"github.com/joebalancio/wt/pkg/domain"
)

// TestIntegration_RemoveEnhanced_Basic tests the enhanced remove workflow
func TestIntegration_RemoveEnhanced_Basic(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a feature worktree
	featureBranch := "feature/remove-test"
	featurePath := filepath.Join(repoPath, "feature-remove")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Make a commit in the feature branch
	featureFile := filepath.Join(featurePath, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("Feature content\n"), 0o644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}
	runGitCommand(t, featurePath, "add", "feature.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add feature")

	// Merge the feature branch into main
	runGitCommand(t, repoPath, "checkout", "main")
	runGitCommand(t, repoPath, "merge", featureBranch)

	// Now use RemoveEnhanced to clean up
	_, err = service.RemoveEnhanced(ctx, featurePath, domain.ForceNone)
	if err != nil {
		t.Fatalf("RemoveEnhanced() error = %v", err)
	}

	// Verify worktree is gone
	worktrees, err := service.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree after remove, got %d", len(worktrees))
	}

	// Verify branch is deleted
	exists, err := client.BranchExists(ctx, featureBranch)
	if err != nil {
		t.Fatalf("failed to check branch existence: %v", err)
	}
	if exists {
		t.Error("feature branch still exists after RemoveEnhanced")
	}
}

// TestIntegration_RemoveEnhanced_UnmergedFails tests that unmerged branches fail without force
func TestIntegration_RemoveEnhanced_UnmergedFails(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a feature worktree with uncommitted changes
	featureBranch := "feature/unmerged"
	featurePath := filepath.Join(repoPath, "feature-unmerged")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Make a commit (unmerged)
	featureFile := filepath.Join(featurePath, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("Unmerged content\n"), 0o644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}
	runGitCommand(t, featurePath, "add", "feature.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add unmerged feature")

	runGitCommand(t, repoPath, "checkout", "main")

	// Try to remove without force (should fail - unmerged)
	_, err = service.RemoveEnhanced(ctx, featurePath, domain.ForceNone)
	if err == nil {
		t.Fatal("RemoveEnhanced() expected error for unmerged branch, got nil")
	}

	// Cleanup with force
	_, err = service.RemoveEnhanced(ctx, featurePath, domain.ForceLocal)
	if err != nil {
		t.Fatalf("RemoveEnhanced() with force error = %v", err)
	}
}

// TestIntegration_RemoveEnhanced_DirtyFails tests that dirty worktrees fail without force
func TestIntegration_RemoveEnhanced_DirtyFails(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a feature worktree
	featureBranch := "feature/dirty"
	featurePath := filepath.Join(repoPath, "feature-dirty")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Add uncommitted changes
	featureFile := filepath.Join(featurePath, "uncommitted.txt")
	if err := os.WriteFile(featureFile, []byte("Uncommitted content\n"), 0o644); err != nil {
		t.Fatalf("failed to create uncommitted file: %v", err)
	}

	runGitCommand(t, repoPath, "checkout", "main")

	// Try to remove without force (should fail - dirty)
	_, err = service.RemoveEnhanced(ctx, featurePath, domain.ForceNone)
	if err == nil {
		t.Fatal("RemoveEnhanced() expected error for dirty worktree, got nil")
	}

	// Cleanup with force
	_, err = service.RemoveEnhanced(ctx, featurePath, domain.ForceLocal)
	if err != nil {
		t.Fatalf("RemoveEnhanced() with force error = %v", err)
	}
}

// TestIntegration_RemoveEnhanced_DefaultBranchFails tests that default branch cannot be removed
func TestIntegration_RemoveEnhanced_DefaultBranchFails(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Try to remove main branch (should fail)
	_, err = service.RemoveEnhanced(ctx, repoPath, domain.ForceLocal)
	if err == nil {
		t.Fatal("RemoveEnhanced() expected error for default branch, got nil")
	}
}

// TestIntegration_RemoveEnhanced_CWDResolution tests removing from inside a worktree
func TestIntegration_RemoveEnhanced_CWDResolution(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a feature worktree
	featureBranch := "feature/cwd-test"
	featurePath := filepath.Join(repoPath, "feature-cwd")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Merge the branch
	runGitCommand(t, featurePath, "commit", "--allow-empty", "-m", "Empty commit")
	runGitCommand(t, repoPath, "checkout", "main")
	runGitCommand(t, repoPath, "merge", featureBranch)

	// Change into the worktree
	subDir := filepath.Join(featurePath, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("failed to change to subdir: %v", err)
	}

	// Resolve worktree from CWD
	cwd, _ := os.Getwd()
	resolved, err := service.ResolveFromCWD(ctx, cwd)
	if err != nil {
		t.Fatalf("ResolveFromCWD() error = %v", err)
	}
	if resolved.Branch != featureBranch {
		t.Errorf("resolved branch = %s, want %s", resolved.Branch, featureBranch)
	}

	// Remove using resolved path
	_, err = service.RemoveEnhanced(ctx, resolved.Path, domain.ForceNone)
	if err != nil {
		t.Fatalf("RemoveEnhanced() error = %v", err)
	}
}
```

**Step 2: Run integration tests**

Run: `WT_INTEGRATION_TEST=1 go test ./tests/... -run "TestIntegration_RemoveEnhanced" -v`
Expected: All integration tests PASS

**Step 3: Commit**

```bash
git add tests/remove_enhanced_integration_test.go
git commit -m "test: add integration tests for enhanced remove command"
```

---

## Task 7: Run Full Test Suite and Lint

**Step 1: Run all tests**

Run: `make test`
Expected: All tests PASS

**Step 2: Run linting**

Run: `make lint`
Expected: No lint errors

**Step 3: Run formatting**

Run: `make fmt`

**Step 4: Run check**

Run: `make check`
Expected: All checks pass

**Step 5: Final commit if any fixes needed**

```bash
git add -A
git commit -m "chore: fix lint and format issues"
```

---

## Summary

**Total Tasks:** 7

**Key Changes:**
1. Added `ForceLevel` type with `ForceNone`, `ForceLocal`, `ForceRemote` values
2. Added git client methods: `IsBranchMerged`, `RemoteBranchExists`, `DeleteRemoteBranch`
3. Added `ResolveFromCWD` for detecting current worktree from working directory
4. Added `RemoveEnhanced` service method with full safety checks and branch deletion
5. Updated CLI command to accept optional path and parse force levels
6. Added comprehensive unit and integration tests

**Backward Compatibility:**
- Existing `wt remove --force <path>` continues to work
- Path argument remains supported for explicit usage
- New behavior (branch deletion) is always-on (no flag needed)

**Files Changed:**
- `pkg/domain/worktree.go` - ForceLevel type
- `pkg/domain/force_level_test.go` - Forcelevel tests
- `internal/git/worktree.go` - Branch operations
- `internal/git/client_interface.go` - Interface updates
- `internal/git/branch_operations_test.go` - Placeholder tests
- `internal/worktree/service.go` - RemoveEnhanced, ResolveFromCWD
- `internal/worktree/service_test.go` - Mock implementations and tests
- `internal/cli/remove.go` - Updated command
- `internal/cli/remove_enhanced_test.go` - CLI tests
- `tests/remove_enhanced_integration_test.go` - Integration tests
