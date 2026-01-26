package worktree

import (
	"context"
	"fmt"
	"testing"

	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/pkg/domain"
)

// Compile-time interface compliance check
var _ git.GitClient = (*mockGitClient)(nil)

// mockGitClient is a simple mock for testing
type mockGitClient struct {
	listWorktreesFunc    func(ctx context.Context) ([]*domain.Worktree, error)
	addWorktreeFunc      func(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	removeWorktreeFunc   func(ctx context.Context, path string, force bool) error
	getRepoInfoFunc      func(ctx context.Context) (*domain.GitRepo, error)
	branchExistsFunc     func(ctx context.Context, branch string) (bool, error)
	getCurrentBranchFunc func(ctx context.Context) (string, error)
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

func (m *mockGitClient) GetCurrentBranch(ctx context.Context) (string, error) {
	if m.getCurrentBranchFunc != nil {
		return m.getCurrentBranchFunc(ctx)
	}
	return "main", nil
}

func TestService_List(t *testing.T) {
	t.Run("returns all worktrees", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/main", Branch: "main"},
					{Path: "/feature", Branch: "feature"},
				}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		worktrees, err := svc.List(context.Background(), nil)

		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(worktrees) != 2 {
			t.Errorf("got %d worktrees, want 2", len(worktrees))
		}
	})

	t.Run("returns error when git client fails", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return nil, fmt.Errorf("git command failed")
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		_, err = svc.List(context.Background(), nil)

		if err == nil {
			t.Fatal("List() expected error, got nil")
		}
		if err == nil || err.Error() == "" {
			t.Errorf("expected error message, got empty")
		}
	})

	t.Run("returns error when NewService called with nil", func(t *testing.T) {
		_, err := NewService(nil, config.DefaultConfig())

		if err == nil {
			t.Fatal("NewService(nil) expected error, got nil")
		}
		if err.Error() != "gitClient cannot be nil" {
			t.Errorf("expected 'gitClient cannot be nil', got %q", err.Error())
		}
	})

	t.Run("filters by branch name", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/main", Branch: "main"},
					{Path: "/feature", Branch: "feature"},
				}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
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
			addWorktreeFunc: func(_ context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
				return &domain.Worktree{
					Path:   "/test/" + spec.Branch,
					Branch: spec.Branch,
				}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
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

	t.Run("returns error when spec validation fails", func(t *testing.T) {
		mock := &mockGitClient{}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		// Invalid spec - empty branch
		spec := domain.WorktreeCreateSpec{
			Branch: "",
			Base:   "main",
		}

		_, err = svc.Add(context.Background(), spec)
		if err == nil {
			t.Fatal("Add() expected error for invalid spec, got nil")
		}
	})

	t.Run("returns error when git client fails", func(t *testing.T) {
		mock := &mockGitClient{
			addWorktreeFunc: func(_ context.Context, _ domain.WorktreeCreateSpec) (*domain.Worktree, error) {
				return nil, fmt.Errorf("git worktree add failed")
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		spec := domain.WorktreeCreateSpec{
			Branch: "new-feature",
			Base:   "main",
		}

		_, err = svc.Add(context.Background(), spec)
		if err == nil {
			t.Fatal("Add() expected error when git client fails, got nil")
		}
	})
}

func TestService_Remove(t *testing.T) {
	t.Run("removes worktree", func(t *testing.T) {
		mock := &mockGitClient{
			removeWorktreeFunc: func(_ context.Context, _ string, _ bool) error {
				return nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Remove(context.Background(), "/test/worktree", false)
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
	})

	t.Run("returns error when path is empty", func(t *testing.T) {
		mock := &mockGitClient{}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Remove(context.Background(), "", false)
		if err == nil {
			t.Fatal("Remove() expected error for empty path, got nil")
		}
		if err.Error() != "path is required" {
			t.Errorf("expected 'path is required', got %q", err.Error())
		}
	})

	t.Run("returns error when git client fails", func(t *testing.T) {
		mock := &mockGitClient{
			removeWorktreeFunc: func(_ context.Context, _ string, _ bool) error {
				return fmt.Errorf("git worktree remove failed")
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Remove(context.Background(), "/test/worktree", false)
		if err == nil {
			t.Fatal("Remove() expected error when git client fails, got nil")
		}
	})
}
