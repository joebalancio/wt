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

func withStubbedFuzzyRunner(t *testing.T, stub func(context.Context, string, []FuzzyItem, []FuzzyItem) (*FuzzyItem, error)) {
	t.Helper()

	previous := runFuzzySelect
	runFuzzySelect = stub
	t.Cleanup(func() {
		runFuzzySelect = previous
	})
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

func TestIsTerminal(_ *testing.T) {
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

func TestPicker_SelectWorktree_UsesFuzzySelect(t *testing.T) {
	mock := &mockBranchLister{
		listWorktreesFunc: func(_ context.Context) ([]*domain.Worktree, error) {
			return []*domain.Worktree{
				{Path: "/repo", Branch: ""},
				{Path: "/repo/.worktrees/auth", Branch: "feat/auth"},
			}, nil
		},
	}
	picker := NewPicker(mock)

	withStubbedFuzzyRunner(t, func(_ context.Context, title string, items []FuzzyItem, pinned []FuzzyItem) (*FuzzyItem, error) {
		if title != "Select worktree to remove:" {
			t.Fatalf("title = %q, want %q", title, "Select worktree to remove:")
		}
		if len(pinned) != 0 {
			t.Fatalf("len(pinned) = %d, want 0", len(pinned))
		}
		if len(items) != 1 {
			t.Fatalf("len(items) = %d, want 1", len(items))
		}
		if items[0].Label != "feat/auth -> /repo/.worktrees/auth" {
			t.Fatalf("item label = %q", items[0].Label)
		}
		return &FuzzyItem{Label: items[0].Label, Value: items[0].Value}, nil
	})

	got, err := picker.SelectWorktree(context.Background())
	if err != nil {
		t.Fatalf("SelectWorktree() error = %v", err)
	}
	if got != "/repo/.worktrees/auth" {
		t.Fatalf("SelectWorktree() = %q, want %q", got, "/repo/.worktrees/auth")
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

func TestPicker_SelectBranch_ReturnsExistingBranchFromFuzzySelect(t *testing.T) {
	mock := &mockBranchLister{
		listAllBranchesFunc: func(_ context.Context) ([]string, error) {
			return []string{"main", "develop"}, nil
		},
	}
	picker := NewPicker(mock)

	withStubbedFuzzyRunner(t, func(_ context.Context, title string, items []FuzzyItem, pinned []FuzzyItem) (*FuzzyItem, error) {
		if title != "Select or create a branch:" {
			t.Fatalf("title = %q, want %q", title, "Select or create a branch:")
		}
		if len(pinned) != 1 || pinned[0].Value != newBranchOption {
			t.Fatalf("pinned = %#v, want create-new option", pinned)
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
		return &FuzzyItem{Label: "develop", Value: "develop"}, nil
	})

	got, err := picker.SelectBranch(context.Background())
	if err != nil {
		t.Fatalf("SelectBranch() error = %v", err)
	}
	if got.Branch != "develop" || got.IsNew {
		t.Fatalf("SelectBranch() = %#v, want existing branch result", got)
	}
}

func TestPicker_SelectBranch_PropagatesCancellation(t *testing.T) {
	mock := &mockBranchLister{
		listAllBranchesFunc: func(_ context.Context) ([]string, error) {
			return []string{"main"}, nil
		},
	}
	picker := NewPicker(mock)

	withStubbedFuzzyRunner(t, func(_ context.Context, _ string, _ []FuzzyItem, _ []FuzzyItem) (*FuzzyItem, error) {
		return nil, ErrCanceled
	})

	_, err := picker.SelectBranch(context.Background())
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
}

func TestPicker_PromptNewBranch_UsesFuzzySelectForBaseBranch(t *testing.T) {
	previousInput := runBranchNameInput
	runBranchNameInput = func(_ []string) (string, error) {
		return "feat/new-search", nil
	}
	t.Cleanup(func() {
		runBranchNameInput = previousInput
	})

	picker := NewPicker(&mockBranchLister{})
	withStubbedFuzzyRunner(t, func(_ context.Context, title string, items []FuzzyItem, pinned []FuzzyItem) (*FuzzyItem, error) {
		if title != "Select base branch:" {
			t.Fatalf("title = %q, want %q", title, "Select base branch:")
		}
		if len(pinned) != 0 {
			t.Fatalf("len(pinned) = %d, want 0", len(pinned))
		}
		if len(items) != 2 {
			t.Fatalf("len(items) = %d, want 2", len(items))
		}
		return &FuzzyItem{Label: "develop", Value: "develop"}, nil
	})

	got, err := picker.promptNewBranch(context.Background(), []string{"main", "develop"})
	if err != nil {
		t.Fatalf("promptNewBranch() error = %v", err)
	}
	if got.Branch != "feat/new-search" || got.BaseBranch != "develop" || !got.IsNew {
		t.Fatalf("promptNewBranch() = %#v, want new branch result", got)
	}
}
