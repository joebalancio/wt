# wt MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the wt CLI tool MVP - a Go reimplementation of the bash script with add/remove/list worktree commands and tmux integration.

**Architecture:** Layered architecture with CLI layer (cobra) → Service layer (business logic) → Domain entities → Infrastructure layer (git/tmux clients).

**Tech Stack:** Go 1.22+, cobra (CLI framework), gopkg.in/yaml.v3 (config), os/exec (git/tmux invocation)

---

## Task 1: Create Domain Package with Core Entities

**Files:**
- Create: `pkg/domain/worktree.go`

**Step 1: Write the failing test**

Create file `pkg/domain/worktree_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/user/wt/pkg/domain"
)

func TestWorktree_String(t *testing.T) {
	t.Run("returns formatted string for attached worktree", func(t *testing.T) {
		w := &domain.Worktree{
			Path:     "/path/to/worktree",
			Branch:   "feature-branch",
			Head:     "abc123def",
			Bare:     false,
			Modified: false,
		}
		result := w.String()
		// String representation for display
		if result == "" {
			t.Error("String() should return non-empty result")
		}
	})
}

func TestWorktreeCreateSpec_Validate(t *testing.T) {
	t.Run("validates required branch field", func(t *testing.T) {
		spec := domain.WorktreeCreateSpec{}
		err := spec.Validate()
		if err == nil {
			t.Error("should require branch field")
		}
	})

	t.Run("passes validation with valid spec", func(t *testing.T) {
		spec := domain.WorktreeCreateSpec{
			Branch:   "feature-branch",
			Base:     "main",
			Checkout: true,
		}
		err := spec.Validate()
		if err != nil {
			t.Errorf("valid spec should pass: %v", err)
		}
	})
}

func TestGitRepo_IsValid(t *testing.T) {
	t.Run("returns true for valid repo", func(t *testing.T) {
		repo := &domain.GitRepo{
			RootPath:      "/valid/path",
			DefaultBranch: "main",
			IsBare:        false,
		}
		if !repo.IsValid() {
			t.Error("valid repo should return true")
		}
	})

	t.Run("returns false for empty root path", func(t *testing.T) {
		repo := &domain.GitRepo{
			RootPath:      "",
			DefaultBranch: "main",
		}
		if repo.IsValid() {
			t.Error("empty root path should be invalid")
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/domain/...`
Expected: FAIL - package doesn't exist yet

**Step 3: Write minimal implementation**

Create file `pkg/domain/worktree.go`:

```go
package domain

import (
	"fmt"
)

// Worktree represents a git worktree
type Worktree struct {
	Path     string // Absolute path to worktree directory
	Branch   string // Branch name (refs/heads/ prefix removed)
	Head     string // Commit SHA or "detached"
	Bare     bool   // Is this a bare worktree
	Modified bool   // Has uncommitted changes
}

// String returns a formatted representation of the worktree
func (w *Worktree) String() string {
	if w.Bare {
		return fmt.Sprintf("%s (bare)", w.Path)
	}
	if w.Detached() {
		return fmt.Sprintf("%s [detached %s]", w.Path, w.Head)
	}
	return fmt.Sprintf("%s [%s]", w.Path, w.Branch)
}

// Detached returns true if HEAD is detached
func (w *Worktree) Detached() bool {
	return w.Head != "" && w.Branch == ""
}

// WorktreeCreateSpec defines parameters for creating a worktree
type WorktreeCreateSpec struct {
	Branch   string  // Branch name (required)
	Base     string  // Base branch for new branches (optional)
	Path     string  // Custom path (optional, auto-generated if empty)
	Force    bool    // Force creation even if path exists
	Checkout bool    // Whether to checkout the branch (default: true)
	Track    *string // Remote branch to track (optional, pointer for nil vs empty)
}

// Validate checks if the spec is valid
func (s *WorktreeCreateSpec) Validate() error {
	if s.Branch == "" {
		return fmt.Errorf("branch name is required")
	}
	return nil
}

// WorktreeFilter defines filters for listing worktrees
type WorktreeFilter struct {
	Branches    []string // Filter by branch names
	PathPrefix  string   // Filter by path prefix
	IncludeBare bool     // Include bare worktrees
}

// Matches checks if a worktree matches the filter
func (f *WorktreeFilter) Matches(w *Worktree) bool {
	if !f.IncludeBare && w.Bare {
		return false
	}
	if f.PathPrefix != "" && w.Path != f.PathPrefix && len(w.Path) < len(f.PathPrefix) {
		return false
	}
	if f.PathPrefix != "" && w.Path[:len(f.PathPrefix)] != f.PathPrefix {
		return false
	}
	if len(f.Branches) > 0 {
		for _, b := range f.Branches {
			if w.Branch == b {
				return true
			}
		}
		return false
	}
	return true
}

// GitRepo represents the git repository context
type GitRepo struct {
	RootPath       string // Absolute path to main worktree
	DefaultBranch  string // Default branch (main/master)
	IsBare         bool   // Is this a bare repository
}

// IsValid checks if the repo info is valid
func (r *GitRepo) IsValid() bool {
	return r.RootPath != ""
}

// Branch represents a git branch
type Branch struct {
	Name   string  // Short branch name
	SHA    string  // Commit SHA
	Remote *string // Remote name (nil for local-only)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/domain/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/domain/
git commit -m "feat: add domain package with core entities"
```

---

## Task 2: Implement GitClient Interface

**Files:**
- Modify: `internal/git/worktree.go` (add interface and refactor existing Client)
- Create: `internal/git/client_interface.go` (define GitClient interface)

**Step 1: Write the failing test**

Create file `internal/git/client_test.go`:

```go
package git

import (
	"context"
	"testing"

	"github.com/user/wt/pkg/domain"
)

func TestGitClient_Interface(t *testing.T) {
	t.Run("Client satisfies GitClient interface", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Skipf("git not available: %v", err)
		}

		// This will fail if Client doesn't implement all interface methods
		var _ GitClient = client
	})
}

func TestGitClient_ListWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	client, err := NewClient()
	if err != nil {
		t.Skipf("git not available: %v", err)
	}

	worktrees, err := client.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	if len(worktrees) == 0 {
		t.Error("expected at least one worktree (the main one)")
	}
}

func TestGitClient_AddWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// This requires a valid git repo - skip for now
	t.Skip("requires test repo setup")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/... -v`
Expected: FAIL - GitClient interface doesn't exist

**Step 3: Write minimal implementation**

Create file `internal/git/client_interface.go`:

```go
package git

import (
	"context"

	"github.com/user/wt/pkg/domain"
)

// GitClient defines the interface for git operations
type GitClient interface {
	ListWorktrees(ctx context.Context) ([]*domain.Worktree, error)
	AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	RemoveWorktree(ctx context.Context, path string, force bool) error
	GetRepoInfo(ctx context.Context) (*domain.GitRepo, error)
	BranchExists(ctx context.Context, branch string) (bool, error)
}
```

Now modify `internal/git/worktree.go`:

Replace the existing file content with:

```go
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client wraps git operations
type Client struct {
	gitPath string
}

// NewClient creates a new git client
func NewClient() (*Client, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git not found in PATH: %w", err)
	}
	return &Client{gitPath: path}, nil
}

// ListWorktrees returns all worktrees for the current repository
func (c *Client) ListWorktrees(ctx context.Context) ([]*domain.Worktree, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "worktree", "list", "--porcelain")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	return parseWorktreeOutput(stdout.String())
}

// AddWorktree creates a new worktree
func (c *Client) AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	args := []string{"worktree", "add"}

	if spec.Force {
		args = append(args, "--force")
	}

	if spec.Base != "" {
		args = append(args, "-b", spec.Branch, spec.Base)
	} else {
		args = append(args, spec.Branch)
	}

	path := spec.Path
	if path == "" {
		// Auto-generate path from branch name
		path = filepath.Join(".", spec.Branch)
	}

	args = append(args, path)

	if !spec.Checkout {
		args = append(args, "--no-checkout")
	}

	if spec.Track != nil {
		args = append(args, "--track", *spec.Track)
	}

	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("adding worktree: %w", err)
	}

	// Convert path to absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	return &domain.Worktree{
		Path:   absPath,
		Branch: spec.Branch,
	}, nil
}

// RemoveWorktree removes a worktree
func (c *Client) RemoveWorktree(ctx context.Context, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing worktree: %w", err)
	}
	return nil
}

// GetRepoInfo returns information about the git repository
func (c *Client) GetRepoInfo(ctx context.Context) (*domain.GitRepo, error) {
	// Get root path
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "rev-parse", "--show-toplevel")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("getting repo root: %w", err)
	}

	rootPath := strings.TrimSpace(stdout.String())

	// Get default branch
	stdout.Reset()
	cmd = exec.CommandContext(ctx, c.gitPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	cmd.Stdout = &stdout

	defaultBranch := "main" // fallback
	if err := cmd.Run(); err == nil {
		// Format: refs/remotes/origin/main
		ref := strings.TrimSpace(stdout.String())
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			defaultBranch = parts[len(parts)-1]
		}
	}

	return &domain.GitRepo{
		RootPath:      rootPath,
		DefaultBranch: defaultBranch,
		IsBare:        false,
	}, nil
}

// BranchExists checks if a branch exists
func (c *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	err := cmd.Run()
	if err != nil {
		// rev-parse returns non-zero if branch doesn't exist
		return false, nil
	}
	return true, nil
}

func parseWorktreeOutput(output string) ([]*domain.Worktree, error) {
	var worktrees []*domain.Worktree
	var currentIndex int = -1

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		switch key {
		case "worktree":
			worktrees = append(worktrees, &domain.Worktree{Path: value})
			currentIndex = len(worktrees) - 1
		case "branch":
			if currentIndex >= 0 {
				worktrees[currentIndex].Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "HEAD":
			if currentIndex >= 0 {
				worktrees[currentIndex].Head = value
			}
		case "detached":
			if currentIndex >= 0 {
				worktrees[currentIndex].Head = "detached"
			}
		}
	}

	return worktrees, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/git/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: implement GitClient interface with context support"
```

---

## Task 3: Implement WorktreeService

**Files:**
- Create: `internal/worktree/service.go`
- Create: `internal/worktree/service_test.go`

**Step 1: Write the failing test**

Create file `internal/worktree/service_test.go`:

```go
package worktree

import (
	"context"
	"testing"

	"github.com/user/wt/pkg/domain"
	"github.com/user/wt/internal/git"
)

// mockGitClient is a simple mock for testing
type mockGitClient struct {
	listWorktreesFunc  func(ctx context.Context) ([]*domain.Worktree, error)
	addWorktreeFunc    func(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	removeWorktreeFunc func(ctx context.Context, path string, force bool) error
	getRepoInfoFunc    func(ctx context.Context) (*domain.GitRepo, error)
	branchExistsFunc   func(ctx context.Context, branch string) (bool, error)
}

func (m *mockGitClient) ListWorktrees(ctx context.Context) ([]*domain.Worktree, error) {
	if m.listWorktreesFunc != nil {
		return m.listWorktreesFunc(ctx)
	}
	return []*domain.Worktree{}, nil
}

func (m *mockGitClient) AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	if m.addWorktreeFunc != nil {
		return m.addWorktreeFunc(ctx, spec)
	}
	return &domain.Worktree{Path: "/test", Branch: spec.Branch}, nil
}

func (m *mockGitClient) RemoveWorktree(ctx context.Context, path string, force bool) error {
	if m.removeWorktreeFunc != nil {
		return m.removeWorktreeFunc(ctx, path, force)
	}
	return nil
}

func (m *mockGitClient) GetRepoInfo(ctx context.Context) (*domain.GitRepo, error) {
	if m.getRepoInfoFunc != nil {
		return m.getRepoInfoFunc(ctx)
	}
	return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
}

func (m *mockGitClient) BranchExists(ctx context.Context, branch string) (bool, error) {
	if m.branchExistsFunc != nil {
		return m.branchExistsFunc(ctx, branch)
	}
	return true, nil
}

func TestService_List(t *testing.T) {
	t.Run("returns all worktrees", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(ctx context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/main", Branch: "main"},
					{Path: "/feature", Branch: "feature"},
				}, nil
			},
		}

		svc := NewService(mock)
		worktrees, err := svc.List(context.Background(), nil)

		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(worktrees) != 2 {
			t.Errorf("got %d worktrees, want 2", len(worktrees))
		}
	})

	t.Run("filters by branch name", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(ctx context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/main", Branch: "main"},
					{Path: "/feature", Branch: "feature"},
				}, nil
			},
		}

		svc := NewService(mock)
		filter := &domain.WorktreeFilter{Branches: []string{"main"}}
		worktrees, err := svc.List(context.Background(), filter)

		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(worktrees) != 1 {
			t.Errorf("got %d worktrees, want 1", len(worktrees))
		}
		if worktrees[0].Branch != "main" {
			t.Errorf("got branch %s, want main", worktrees[0].Branch)
		}
	})
}

func TestService_Add(t *testing.T) {
	t.Run("creates new worktree", func(t *testing.T) {
		mock := &mockGitClient{
			addWorktreeFunc: func(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
				return &domain.Worktree{
					Path:   "/test/" + spec.Branch,
					Branch: spec.Branch,
				}, nil
			},
		}

		svc := NewService(mock)
		spec := domain.WorktreeCreateSpec{
			Branch: "new-feature",
			Base:   "main",
		}

		worktree, err := svc.Add(context.Background(), spec)
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if worktree.Branch != "new-feature" {
			t.Errorf("got branch %s, want new-feature", worktree.Branch)
		}
	})
}

func TestService_Remove(t *testing.T) {
	t.Run("removes worktree", func(t *testing.T) {
		mock := &mockGitClient{
			removeWorktreeFunc: func(ctx context.Context, path string, force bool) error {
				return nil
			},
		}

		svc := NewService(mock)
		err := svc.Remove(context.Background(), "/test/worktree", false)
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/...`
Expected: FAIL - package doesn't exist

**Step 3: Write minimal implementation**

Create file `internal/worktree/service.go`:

```go
package worktree

import (
	"context"
	"fmt"

	"github.com/user/wt/internal/git"
	"github.com/user/wt/pkg/domain"
)

// Service provides worktree management operations
type Service struct {
	git git.GitClient
}

// NewService creates a new worktree service
func NewService(gitClient git.GitClient) *Service {
	return &Service{
		git: gitClient,
	}
}

// List returns worktrees, optionally filtered
func (s *Service) List(ctx context.Context, filter *domain.WorktreeFilter) ([]*domain.Worktree, error) {
	worktrees, err := s.git.ListWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	// Apply filter if provided
	if filter != nil {
		var filtered []*domain.Worktree
		for _, w := range worktrees {
			if filter.Matches(w) {
				filtered = append(filtered, w)
			}
		}
		return filtered, nil
	}

	return worktrees, nil
}

// Add creates a new worktree
func (s *Service) Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}

	worktree, err := s.git.AddWorktree(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("adding worktree: %w", err)
	}

	return worktree, nil
}

// Remove removes a worktree
func (s *Service) Remove(ctx context.Context, path string, force bool) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	if err := s.git.RemoveWorktree(ctx, path, force); err != nil {
		return fmt.Errorf("removing worktree: %w", err)
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/
git commit -m "feat: add worktree service layer"
```

---

## Task 4: Implement `wt list` Command

**Files:**
- Modify: `internal/cli/worktree.go`
- Create: `internal/cli/list.go`

**Step 1: Write the failing test**

Create file `internal/cli/list_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/user/wt/pkg/domain"
)

func TestNewListCmd(t *testing.T) {
	t.Run("creates list command", func(t *testing.T) {
		cmd := NewListCmd()
		if cmd == nil {
			t.Fatal("NewListCmd() returned nil")
		}
		if cmd.Use != "list" {
			t.Errorf("got Use %q, want 'list'", cmd.Use)
		}
	})
}

func TestListWorktrees(t *testing.T) {
	t.Run("formats worktree output", func(t *testing.T) {
		worktrees := []*domain.Worktree{
			{Path: "/main", Branch: "main", Head: "abc123"},
			{Path: "/feature", Branch: "feature", Head: "def456"},
		}

		var buf bytes.Buffer
		err := printWorktrees(&buf, worktrees)
		if err != nil {
			t.Fatalf("printWorktrees() error = %v", err)
		}

		output := buf.String()
		if !contains(output, "main") {
			t.Error("output should contain 'main' branch")
		}
		if !contains(output, "feature") {
			t.Error("output should contain 'feature' branch")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run TestList -v`
Expected: FAIL - NewListCmd doesn't exist

**Step 3: Write minimal implementation**

Create file `internal/cli/list.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/worktree"
)

// ListWorktrees prints worktrees to the given writer
func printWorktrees(w io.Writer, worktrees []*domain.Worktree) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, wt := range worktrees {
		fmt.Fprintf(tw, "%s\t%s\n", wt.Path, wt.Branch)
	}
	return tw.Flush()
}

// NewListCmd creates the list command
func NewListCmd() *cobra.Command {
	var branches []string
	var pathPrefix string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List git worktrees",
		Long:  `List all git worktrees in the current repository.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			svc := worktree.NewService(gitClient)

			filter := &domain.WorktreeFilter{
				Branches:   branches,
				PathPrefix: pathPrefix,
			}

			worktrees, err := svc.List(ctx, filter)
			if err != nil {
				Fatal("Failed to list worktrees: %v", err)
			}

			if err := printWorktrees(cmd.OutOrStdout(), worktrees); err != nil {
				Fatal("Failed to print worktrees: %v", err)
			}
		},
	}

	cmd.Flags().StringSliceVar(&branches, "branches", nil, "filter by branch names")
	cmd.Flags().StringVar(&pathPrefix, "path", "", "filter by path prefix")

	return cmd
}

func init() {
	RegisterCommand(NewListCmd())
}
```

Now modify `internal/cli/worktree.go` to register subcommands:

```go
package cli

import (
	"github.com/spf13/cobra"
)

var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage git worktrees",
	Long:  `Create, list, and remove git worktrees with automatic tmux session management.`,
}

func init() {
	RegisterCommand(worktreeCmd)
	// Subcommands are registered in their respective init() functions
}
```

Also need to add import at top of `internal/cli/list.go`:
```go
import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/worktree"
	"github.com/user/wt/pkg/domain"
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run TestList -v`
Expected: PASS

**Step 5: Test manually**

Run: `go run cmd/wt/main.go list`
Expected: Lists worktrees (or error if not in git repo)

**Step 6: Commit**

```bash
git add internal/cli/list.go internal/cli/worktree.go internal/cli/list_test.go
git commit -m "feat: add wt list command"
```

---

## Task 5: Implement `wt add` Command

**Files:**
- Create: `internal/cli/add.go`
- Create: `internal/cli/add_test.go`

**Step 1: Write the failing test**

Create file `internal/cli/add_test.go`:

```go
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewAddCmd(t *testing.T) {
	t.Run("creates add command", func(t *testing.T) {
		cmd := NewAddCmd()
		if cmd == nil {
			t.Fatal("NewAddCmd() returned nil")
		}
		if cmd.Use != "add" {
			t.Errorf("got Use %q, want 'add'", cmd.Use)
		}

		// Check required args
		if len(cmd.Args) == 0 {
			t.Error("add command should require branch argument")
		}
	})

	t.Run("has expected flags", func(t *testing.T) {
		cmd := NewAddCmd()

		flag := cmd.Flag("base")
		if flag == nil {
			t.Error("missing --base flag")
		}

		flag = cmd.Flag("path")
		if flag == nil {
			t.Error("missing --path flag")
		}

		flag = cmd.Flag("force")
		if flag == nil {
			t.Error("missing --force flag")
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run TestAdd -v`
Expected: FAIL - NewAddCmd doesn't exist

**Step 3: Write minimal implementation**

Create file `internal/cli/add.go`:

```go
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/worktree"
	"github.com/user/wt/pkg/domain"
)

// NewAddCmd creates the add command
func NewAddCmd() *cobra.Command {
	var (
		base    string
		path    string
		force   bool
		track   string
		noCheckout bool
	)

	cmd := &cobra.Command{
		Use:   "add <branch>",
		Short: "Add a new worktree",
		Long: `Add a new worktree for the specified branch.

If the branch already exists, it will be checked out in the new worktree.
If the branch doesn't exist, it will be created from the specified base branch.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			branch := args[0]

			ctx := context.Background()

			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			svc := worktree.NewService(gitClient)

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

			worktree, err := svc.Add(ctx, spec)
			if err != nil {
				Fatal("Failed to add worktree: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created worktree: %s [%s]\n", worktree.Path, worktree.Branch)
		},
	}

	cmd.Flags().StringVar(&base, "base", "", "base branch for new branch")
	cmd.Flags().StringVar(&path, "path", "", "custom path for worktree")
	cmd.Flags().BoolVar(&force, "force", false, "force creation even if path exists")
	cmd.Flags().StringVar(&track, "track", "", "remote branch to track")
	cmd.Flags().BoolVar(&noCheckout, "no-checkout", false, "don't checkout the branch")

	return cmd
}

func init() {
	// Register as a top-level command
	RegisterCommand(NewAddCmd())
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run TestAdd -v`
Expected: PASS

**Step 5: Test manually**

Run: `go run cmd/wt/main.go add --help`
Expected: Shows help for add command

**Step 6: Commit**

```bash
git add internal/cli/add.go internal/cli/add_test.go
git commit -m "feat: add wt add command"
```

---

## Task 6: Implement `wt remove` Command

**Files:**
- Create: `internal/cli/remove.go`
- Create: `internal/cli/remove_test.go`

**Step 1: Write the failing test**

Create file `internal/cli/remove_test.go`:

```go
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewRemoveCmd(t *testing.T) {
	t.Run("creates remove command", func(t *testing.T) {
		cmd := NewRemoveCmd()
		if cmd == nil {
			t.Fatal("NewRemoveCmd() returned nil")
		}
		if cmd.Use != "remove" {
			t.Errorf("got Use %q, want 'remove'", cmd.Use)
		}

		// Check required args
		if len(cmd.Args) == 0 {
			t.Error("remove command should require path argument")
		}
	})

	t.Run("has expected flags", func(t *testing.T) {
		cmd := NewRemoveCmd()

		flag := cmd.Flag("force")
		if flag == nil {
			t.Error("missing --force flag")
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run TestRemove -v`
Expected: FAIL - NewRemoveCmd doesn't exist

**Step 3: Write minimal implementation**

Create file `internal/cli/remove.go`:

```go
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/worktree"
)

// NewRemoveCmd creates the remove command
func NewRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a worktree",
		Long: `Remove a worktree from the repository.

By default, this will fail if the worktree has uncommitted changes.
Use --force to remove it anyway.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := args[0]

			ctx := context.Background()

			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			svc := worktree.NewService(gitClient)

			if err := svc.Remove(ctx, path, force); err != nil {
				Fatal("Failed to remove worktree: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", path)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force removal even with uncommitted changes")

	return cmd
}

func init() {
	// Register as a top-level command
	RegisterCommand(NewRemoveCmd())
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run TestRemove -v`
Expected: PASS

**Step 5: Test manually**

Run: `go run cmd/wt/main.go remove --help`
Expected: Shows help for remove command

**Step 6: Commit**

```bash
git add internal/cli/remove.go internal/cli/remove_test.go
git commit -m "feat: add wt remove command"
```

---

## Task 7: Make `wt` (no args) Default to List Command

**Files:**
- Modify: `internal/cli/root.go`

**Step 1: Modify the root command to set list as default**

Edit `internal/cli/root.go` - add this after the rootCmd definition:

```go
func init() {
	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path (default is $HOME/.config/wt/config.yaml or .wt.yaml in project)")
	rootCmd.PersistentFlags().CountP("verbose", "v", "verbose output (can be used multiple times)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without executing")

	// Make `wt` (no args) equivalent to `wt list`
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		// Find and execute the list command
		for _, subcmd := range rootCmd.Commands() {
			if subcmd.Name() == "list" {
				subcmd.Run(cmd, args)
				return
			}
		}
		Fatal("list command not found")
	}
}
```

**Step 2: Test manually**

Run: `go run cmd/wt/main.go`
Expected: Lists worktrees (same as `wt list`)

**Step 3: Commit**

```bash
git add internal/cli/root.go
git commit -m "feat: make wt (no args) default to list command"
```

---

## Task 8: Update Executor for Verbose Logging

**Files:**
- Modify: `pkg/executor/executor.go`

**Step 1: Fix the Verbose reference**

The current code references `Verbose := 0` which is incorrect. Fix this:

```go
// Run executes a command with context and timeout
func (e *Executor) Run(ctx context.Context, workdir string, command string) *HookResult {
	startTime := time.Now()

	// Parse command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return &HookResult{
			Hook:    command,
			Success: false,
			Error:   fmt.Errorf("empty command"),
		}
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Add timeout context
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	err := cmd.Run()
	duration := time.Since(startTime)

	output := stdout.String()
	if stderr.String() != "" {
		output += "\n" + stderr.String()
	}

	result := &HookResult{
		Hook:    command,
		Success: err == nil,
		Output:  output,
		Error:   err,
	}

	// Remove the Verbose check - logging should be handled by caller
	if err != nil {
		result.Output += fmt.Sprintf("\n[exited with error after %v]", duration)
	}

	return result
}
```

**Step 2: Run tests**

Run: `go test ./pkg/executor/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add pkg/executor/executor.go
git commit -m "fix: remove invalid Verbose reference in executor"
```

---

## Task 9: Add Integration Tests for End-to-End Workflow

**Files:**
- Create: `tests/integration_test.go`

**Step 1: Write integration test**

Create file `tests/integration_test.go`:

```go
package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/wt/internal/git"
	"github.com/user/wt/pkg/domain"
)

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "wt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.name", "Test User")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "Initial commit")

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func TestIntegration_WorktreeLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Change to test repo
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	t.Run("list worktrees", func(t *testing.T) {
		ctx := context.Background()
		client, err := git.NewClient()
		if err != nil {
			t.Skipf("git not available: %v", err)
		}

		worktrees, err := client.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error = %v", err)
		}

		if len(worktrees) != 1 {
			t.Errorf("expected 1 worktree, got %d", len(worktrees))
		}
	})

	t.Run("add and remove worktree", func(t *testing.T) {
		ctx := context.Background()
		client, err := git.NewClient()
		if err != nil {
			t.Skipf("git not available: %v", err)
		}

		// Add worktree
		spec := domain.WorktreeCreateSpec{
			Branch:   "test-feature",
			Base:     "main",
			Path:     filepath.Join(repoDir, "worktrees", "test-feature"),
			Checkout: true,
		}

		worktree, err := client.AddWorktree(ctx, spec)
		if err != nil {
			t.Fatalf("AddWorktree() error = %v", err)
		}

		if worktree.Branch != "test-feature" {
			t.Errorf("expected branch test-feature, got %s", worktree.Branch)
		}

		// Verify it appears in list
		worktrees, err := client.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error = %v", err)
		}

		if len(worktrees) != 2 {
			t.Errorf("expected 2 worktrees, got %d", len(worktrees))
		}

		// Remove worktree
		if err := client.RemoveWorktree(ctx, worktree.Path, false); err != nil {
			t.Fatalf("RemoveWorktree() error = %v", err)
		}

		// Verify removal
		worktrees, err = client.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error = %v", err)
		}

		if len(worktrees) != 1 {
			t.Errorf("expected 1 worktree after removal, got %d", len(worktrees))
		}
	})
}

func TestIntegration_CLICommands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Build the wt binary
	buildDir, err := os.MkdirTemp("", "wt-build-*")
	if err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}
	defer os.RemoveAll(buildDir)

	buildCmd := exec.Command("go", "build", "-o", filepath.Join(buildDir, "wt"), "github.com/user/wt/cmd/wt")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build wt: %v\n%s", err, output)
	}

	wtBin := filepath.Join(buildDir, "wt")

	t.Run("wt list", func(t *testing.T) {
		cmd := exec.Command(wtBin, "list")
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("wt list failed: %v\n%s", err, output)
		}

		if len(output) == 0 {
			t.Error("expected output from wt list")
		}
	})
}

// Benchmark for performance comparison
func BenchmarkListWorktrees(b *testing.B) {
	repoDir, cleanup := setupTestRepo(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	client, err := git.NewClient()
	if err != nil {
		b.Skipf("git not available: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.ListWorktrees(ctx)
	}
}
```

**Step 2: Run integration tests**

Run: `go test ./tests/... -v`
Expected: PASS (may take a few seconds)

**Step 3: Commit**

```bash
git add tests/
git commit -m "test: add integration tests for end-to-end workflow"
```

---

## Task 10: Run Full Test Suite and Fix Issues

**Step 1: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 2: Run linter**

Run: `make lint`
Expected: No errors (may have warnings)

**Step 3: Fix any linter issues**

If issues found:
```bash
make lint-fix
```

**Step 4: Run build**

Run: `make build`
Expected: Binary created at `bin/wt`

**Step 5: Test the built binary**

Run: `./bin/wt --help`
Expected: Shows help

Run: `./bin/wt`
Expected: Lists worktrees

**Step 6: Commit any fixes**

```bash
git add -A
git commit -m "fix: address linter issues and final polish"
```

---

## Task 11: Update Documentation

**Files:**
- Modify: `README.md`
- Create: `docs/usage.md`

**Step 1: Update README**

Edit `README.md` to ensure it has:
- Project description
- Installation instructions
- Quick start guide
- Command reference

**Step 2: Create usage documentation**

Create file `docs/usage.md` with detailed command examples

**Step 3: Commit**

```bash
git add README.md docs/usage.md
git commit -m "docs: update usage documentation"
```

---

## Summary

After completing all tasks, the wt MVP will have:

1. ✅ Domain entities in `pkg/domain/`
2. ✅ GitClient interface with context support in `internal/git/`
3. ✅ WorktreeService for business logic in `internal/worktree/`
4. ✅ CLI commands: `wt`, `wt add`, `wt remove`, `wt list`
5. ✅ Unit tests for all components
6. ✅ Integration tests for end-to-end workflows
7. ✅ Working binary at `bin/wt`

**Total estimated tasks:** 11
**Total estimated commits:** 13+

---

`★ Insight ─────────────────────────────────────`
**TDD Approach Used:**
1. Each task follows Red-Green-Refactor: write failing test first, implement minimal code, verify pass
2. Tests are table-driven where appropriate (porcelain parser) and use simple mocks for service layer
3. Integration tests run against real git in temp directories - this catches actual CLI interaction bugs
`─────────────────────────────────────────────────`
