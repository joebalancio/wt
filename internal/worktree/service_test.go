package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/pkg/domain"
)

// Compile-time interface compliance check
var _ git.GitClient = (*mockGitClient)(nil)

// mockGitClient is a simple mock for testing
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
	listAllBranchesFunc    func(ctx context.Context) ([]string, error)
}

type mockGitClientWithDetection struct {
	*mockGitClient
	isBranchMergedWithDetectionFunc func(ctx context.Context, branch string, ghClient *git.GhClient) (bool, error)
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

func (m *mockGitClient) IsInWorktree(ctx context.Context) (bool, string, error) {
	if m.isInWorktreeFunc != nil {
		return m.isInWorktreeFunc(ctx)
	}
	return false, "/repo", nil
}

func (m *mockGitClient) ListAllBranches(ctx context.Context) ([]string, error) {
	if m.listAllBranchesFunc != nil {
		return m.listAllBranchesFunc(ctx)
	}
	return []string{"main"}, nil
}

func (m *mockGitClientWithDetection) IsBranchMergedWithDetection(ctx context.Context, branch string, ghClient *git.GhClient) (bool, error) {
	if m.isBranchMergedWithDetectionFunc != nil {
		return m.isBranchMergedWithDetectionFunc(ctx, branch, ghClient)
	}
	return false, nil
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
		if err.Error() == "" {
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

func TestService_GetOrCreate(t *testing.T) {
	t.Run("returns error when branch is empty", func(t *testing.T) {
		mock := &mockGitClient{}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		spec := domain.WorktreeCreateSpec{Branch: ""}
		_, _, err = svc.GetOrCreate(context.Background(), spec)
		if err == nil {
			t.Fatal("GetOrCreate() expected error for empty branch, got nil")
		}
		if err.Error() != "branch is required" {
			t.Errorf("expected 'branch is required', got %q", err.Error())
		}
	})

	t.Run("returns existing worktree when branch already has one", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/existing-feature", Branch: "existing-feature"},
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

		spec := domain.WorktreeCreateSpec{Branch: "existing-feature"}
		worktree, created, err := svc.GetOrCreate(context.Background(), spec)
		if err != nil {
			t.Fatalf("GetOrCreate() error = %v", err)
		}
		if created {
			t.Error("expected created=false for existing worktree, got true")
		}
		if worktree.Branch != "existing-feature" {
			t.Errorf("got branch %s, want existing-feature", worktree.Branch)
		}
		if worktree.Path != "/worktrees/existing-feature" {
			t.Errorf("got path %s, want /worktrees/existing-feature", worktree.Path)
		}
	})

	t.Run("creates new worktree when branch has none", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
				}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
			addWorktreeFunc: func(_ context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
				return &domain.Worktree{
					Path:   "/worktrees/" + spec.Branch,
					Branch: spec.Branch,
				}, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		spec := domain.WorktreeCreateSpec{Branch: "new-feature"}
		worktree, created, err := svc.GetOrCreate(context.Background(), spec)
		if err != nil {
			t.Fatalf("GetOrCreate() error = %v", err)
		}
		if !created {
			t.Error("expected created=true for new worktree, got false")
		}
		if worktree.Branch != "new-feature" {
			t.Errorf("got branch %s, want new-feature", worktree.Branch)
		}
	})

	t.Run("returns error when listing worktrees fails", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return nil, fmt.Errorf("git error")
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		spec := domain.WorktreeCreateSpec{Branch: "some-branch"}
		_, _, err = svc.GetOrCreate(context.Background(), spec)
		if err == nil {
			t.Fatal("GetOrCreate() expected error when listing fails, got nil")
		}
	})

	t.Run("returns error when creating worktree fails", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{{Path: "/repo", Branch: "main"}}, nil
			},
			getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
				return &domain.GitRepo{RootPath: "/repo", DefaultBranch: "main"}, nil
			},
			addWorktreeFunc: func(_ context.Context, _ domain.WorktreeCreateSpec) (*domain.Worktree, error) {
				return nil, fmt.Errorf("git worktree add failed")
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		spec := domain.WorktreeCreateSpec{Branch: "new-feature"}
		_, _, err = svc.GetOrCreate(context.Background(), spec)
		if err == nil {
			t.Fatal("GetOrCreate() expected error when creation fails, got nil")
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
			removeWorktreeFunc: func(_ context.Context, _ string, force bool) error {
				removeCalled = true
				if !force {
					t.Error("expected forced remove")
				}
				return nil
			},
			deleteBranchFunc: func(_ context.Context, _ string, force bool) error {
				deleteCalled = true
				if !force {
					t.Error("expected forced branch delete")
				}
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

	t.Run("uses enhanced merge detection when available", func(t *testing.T) {
		var removeCalled bool
		base := &mockGitClient{
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
			// Legacy check fails for squash merges.
			isBranchMergedFunc: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			removeWorktreeFunc: func(_ context.Context, _ string, _ bool) error {
				removeCalled = true
				return nil
			},
			deleteBranchFunc: func(_ context.Context, _ string, _ bool) error {
				return nil
			},
		}
		mock := &mockGitClientWithDetection{
			mockGitClient: base,
			isBranchMergedWithDetectionFunc: func(_ context.Context, _ string, _ *git.GhClient) (bool, error) {
				return true, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.RemoveEnhanced(context.Background(), "/worktrees/feat", domain.ForceNone)
		if err != nil {
			t.Fatalf("RemoveEnhanced() error = %v", err)
		}
		if !removeCalled {
			t.Error("RemoveWorktree was not called")
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
			remoteBranchExistsFunc: func(_ context.Context, remote, branch string) (bool, error) {
				if remote != "origin" || branch != "feat" {
					t.Errorf("unexpected remote/branch: %s/%s", remote, branch)
				}
				return true, nil
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

	t.Run("detached HEAD worktree returns error", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main", Head: "abc"},
					{Path: "/worktrees/detached", Branch: "", Head: "123456"},
				}, nil
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

	t.Run("non-existent worktree path returns error", func(t *testing.T) {
		mock := &mockGitClient{
			listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
				return []*domain.Worktree{
					{Path: "/repo", Branch: "main"},
					{Path: "/worktrees/feat", Branch: "feat"},
				}, nil
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

func TestResolvePath_Dedicated_addsRepoName(t *testing.T) {
	mockGit := &mockGitClient{
		getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
			return &domain.GitRepo{RootPath: "/home/user/projects/my-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "dedicated"
	cfg.Worktree.DedicatedPath = "/tmp/worktrees"

	svc, err := NewService(mockGit, cfg)
	if err != nil {
		t.Fatal(err)
	}

	path, err := svc.ResolvePath(context.Background(), "feature/auth", "")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}

	// Path should include repo name: /tmp/worktrees/my-repo/feature/auth
	expected := "/tmp/worktrees/my-repo/feature/auth"
	if path != expected {
		t.Errorf("ResolvePath() = %q, want %q", path, expected)
	}
}

func TestResolvePath_PerRepo_unchanged(t *testing.T) {
	mockGit := &mockGitClient{
		getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
			return &domain.GitRepo{RootPath: "/home/user/projects/my-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig() // per-repo is default

	svc, err := NewService(mockGit, cfg)
	if err != nil {
		t.Fatal(err)
	}

	path, err := svc.ResolvePath(context.Background(), "feature/auth", "")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}

	// Path should be per-repo style: <repo>/.worktrees/<branch>
	expected := "/home/user/projects/my-repo/.worktrees/feature/auth"
	if path != expected {
		t.Errorf("ResolvePath() = %q, want %q", path, expected)
	}
}

func TestService_Done(t *testing.T) {
	t.Run("successful done workflow", func(t *testing.T) {
		mergeCalled := false
		commitCalled := false
		dirtyChecked := false
		removeCalled := false
		deleteCalled := false

		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, sourceBranch string) error {
				mergeCalled = true
				if sourceBranch != "feature-branch" {
					t.Errorf("expected sourceBranch 'feature-branch', got %s", sourceBranch)
				}
				return nil
			},
			createSquashCommitFunc: func(_ context.Context, message string) error {
				commitCalled = true
				if message != "Merge feature-branch" {
					t.Errorf("expected commit message 'Merge feature-branch', got %s", message)
				}
				return nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, path string) (bool, error) {
				dirtyChecked = true
				if path != "/worktrees/feature-branch" {
					t.Errorf("expected path '/worktrees/feature-branch', got %s", path)
				}
				return false, nil
			},
			removeWorktreeFunc: func(_ context.Context, path string, force bool) error {
				removeCalled = true
				if path != "/worktrees/feature-branch" {
					t.Errorf("expected path '/worktrees/feature-branch', got %s", path)
				}
				if !force {
					t.Errorf("expected force=true, got false")
				}
				return nil
			},
			deleteBranchFunc: func(_ context.Context, branch string, force bool) error {
				deleteCalled = true
				if branch != "feature-branch" {
					t.Errorf("expected branch 'feature-branch', got %s", branch)
				}
				if !force {
					t.Errorf("expected force=true, got false")
				}
				return nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "/worktrees/feature-branch", "feature-branch", false)
		if err != nil {
			t.Fatalf("Done() error = %v", err)
		}

		if !mergeCalled {
			t.Error("SquashMerge was not called")
		}
		if !commitCalled {
			t.Error("CreateSquashCommit was not called")
		}
		if !dirtyChecked {
			t.Error("IsWorktreeDirty was not called")
		}
		if !removeCalled {
			t.Error("RemoveWorktree was not called")
		}
		if !deleteCalled {
			t.Error("DeleteBranch was not called")
		}
	})

	t.Run("merge failure", func(t *testing.T) {
		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, _ string) error {
				return fmt.Errorf("merge conflict")
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "/worktrees/feature-branch", "feature-branch", false)
		if err == nil {
			t.Fatal("Done() expected error for merge failure, got nil")
		}
	})

	t.Run("branch not found", func(t *testing.T) {
		mock := &mockGitClient{
			branchExistsFunc: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "/worktrees/feature-branch", "feature-branch", false)
		if err == nil {
			t.Fatal("Done() expected error for branch not found, got nil")
		}
	})

	t.Run("dirty worktree without force", func(t *testing.T) {
		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, _ string) error {
				return nil
			},
			createSquashCommitFunc: func(_ context.Context, _ string) error {
				return nil
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

		err = svc.Done(context.Background(), "/worktrees/feature-branch", "feature-branch", false)
		if err == nil {
			t.Fatal("Done() expected error for dirty worktree without force, got nil")
		}
	})

	t.Run("dirty worktree with force", func(t *testing.T) {
		mock := &mockGitClient{
			squashMergeFunc: func(_ context.Context, _ string) error {
				return nil
			},
			createSquashCommitFunc: func(_ context.Context, _ string) error {
				return nil
			},
			isWorktreeDirtyFunc: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			removeWorktreeFunc: func(_ context.Context, _ string, force bool) error {
				if !force {
					t.Errorf("expected force=true, got false")
				}
				return nil
			},
			deleteBranchFunc: func(_ context.Context, _ string, force bool) error {
				if !force {
					t.Errorf("expected force=true, got false")
				}
				return nil
			},
		}

		cfg := config.DefaultConfig()
		svc, err := NewService(mock, cfg)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		err = svc.Done(context.Background(), "/worktrees/feature-branch", "feature-branch", true)
		if err != nil {
			t.Fatalf("Done() error = %v", err)
		}
	})
}

func TestResolvePath_CollisionDetection(t *testing.T) {
	// Create a temp directory to simulate existing worktree from different repo
	tempDir := t.TempDir()
	// The target path will be <dedicatedPath>/<repoName>/<branch>
	// If the repo name is "my-repo" and branch is "feature/auth",
	// the path will be <tempDir>/my-repo/feature/auth
	existingPath := filepath.Join(tempDir, "my-repo", "feature", "auth")
	if err := os.MkdirAll(existingPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a .git file to simulate worktree pointing to different repo
	// The .git file points to a DIFFERENT repo than the current one
	gitFile := filepath.Join(existingPath, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: /some/other/path/my-repo/.git/worktrees/feature/auth"), 0o644); err != nil {
		t.Fatal(err)
	}

	mockGit := &mockGitClient{
		getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
			// Current repo is different from the one that created the worktree
			// The repoName extracted from this path will be "my-repo"
			// So targetPath will be <tempDir>/my-repo/feature/auth
			return &domain.GitRepo{RootPath: "/home/user/projects/my-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "dedicated"
	cfg.Worktree.DedicatedPath = tempDir

	svc, err := NewService(mockGit, cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.ResolvePath(context.Background(), "feature/auth", "")
	if err == nil {
		t.Error("ResolvePath() expected collision error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "collision") {
		t.Errorf("ResolvePath() error should mention collision, got: %v", err)
	}
}

func TestResolvePath_SameRepo_NoCollision(t *testing.T) {
	// Create a temp directory to simulate existing worktree from SAME repo
	tempDir := t.TempDir()
	// The target path will be <tempDir>/my-repo/feature/auth
	existingPath := filepath.Join(tempDir, "my-repo", "feature", "auth")
	if err := os.MkdirAll(existingPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a .git file pointing to the SAME repo as current
	gitFile := filepath.Join(existingPath, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: /home/user/projects/my-repo/.git/worktrees/feature/auth"), 0o644); err != nil {
		t.Fatal(err)
	}

	mockGit := &mockGitClient{
		getRepoInfoFunc: func(_ context.Context) (*domain.GitRepo, error) {
			// Current repo is the SAME as the one that created the worktree
			return &domain.GitRepo{RootPath: "/home/user/projects/my-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "dedicated"
	cfg.Worktree.DedicatedPath = tempDir

	svc, err := NewService(mockGit, cfg)
	if err != nil {
		t.Fatal(err)
	}

	path, err := svc.ResolvePath(context.Background(), "feature/auth", "")
	if err != nil {
		t.Errorf("ResolvePath() unexpected error: %v", err)
	}
	if path != existingPath {
		t.Errorf("ResolvePath() = %q, want %q", path, existingPath)
	}
}
