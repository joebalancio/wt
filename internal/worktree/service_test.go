package worktree

import (
	"context"
	"testing"

	"github.com/user/wt/internal/git"
	"github.com/user/wt/pkg/domain"
)

// Compile-time interface compliance check
var _ git.GitClient = (*mockGitClient)(nil)

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
