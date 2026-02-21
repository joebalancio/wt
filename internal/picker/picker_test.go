package picker

import (
	"context"
	"errors"
	"testing"

	"github.com/joebalancio/wt/pkg/domain"
)

// mockBranchLister is a mock for git.BranchLister.
type mockBranchLister struct {
	listWorktreesFunc   func(ctx context.Context) ([]*domain.Worktree, error)
	listAllBranchesFunc func(ctx context.Context) ([]string, error)
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

func TestIsTerminal(t *testing.T) {
	_ = IsTerminal()
}

func TestNewPicker(t *testing.T) {
	picker := NewPicker(nil)
	if picker == nil {
		t.Error("NewPicker() should not return nil")
	}
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
