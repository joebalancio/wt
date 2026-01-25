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
		expected := "/path/to/worktree [feature-branch]"
		if result != expected {
			t.Errorf("String() = %q, want %q", result, expected)
		}
	})

	t.Run("returns formatted string for bare worktree", func(t *testing.T) {
		w := &domain.Worktree{
			Path: "/path/to/bare",
			Bare: true,
		}
		result := w.String()
		expected := "/path/to/bare (bare)"
		if result != expected {
			t.Errorf("String() = %q, want %q", result, expected)
		}
	})

	t.Run("returns formatted string for detached HEAD", func(t *testing.T) {
		w := &domain.Worktree{
			Path: "/path/to/worktree",
			Head: "abc123def",
		}
		result := w.String()
		expected := "/path/to/worktree [detached abc123def]"
		if result != expected {
			t.Errorf("String() = %q, want %q", result, expected)
		}
	})

	t.Run("returns <nil> for nil receiver", func(t *testing.T) {
		var w *domain.Worktree
		result := w.String()
		if result != "<nil>" {
			t.Errorf("String() = %q, want %q", result, "<nil>")
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

func TestWorktreeFilter_Matches(t *testing.T) {
	t.Run("empty filter matches all worktrees", func(t *testing.T) {
		filter := &domain.WorktreeFilter{}
		w := &domain.Worktree{
			Path:   "/any/path",
			Branch: "any-branch",
		}
		if !filter.Matches(w) {
			t.Error("empty filter should match all worktrees")
		}
	})

	t.Run("matches by path prefix", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			PathPrefix: "/home/user",
		}
		w := &domain.Worktree{
			Path:   "/home/user/project",
			Branch: "feature-branch",
		}
		if !filter.Matches(w) {
			t.Error("should match worktree with matching path prefix")
		}
	})

	t.Run("does not match by different path prefix", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			PathPrefix: "/home/user",
		}
		w := &domain.Worktree{
			Path:   "/other/path/project",
			Branch: "feature-branch",
		}
		if filter.Matches(w) {
			t.Error("should not match worktree with different path prefix")
		}
	})

	t.Run("matches exact path prefix", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			PathPrefix: "/home/user",
		}
		w := &domain.Worktree{
			Path:   "/home/user",
			Branch: "feature-branch",
		}
		if !filter.Matches(w) {
			t.Error("should match worktree with exact path prefix")
		}
	})

	t.Run("excludes bare worktrees when IncludeBare is false", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			IncludeBare: false,
		}
		w := &domain.Worktree{
			Path: "/some/path",
			Bare: true,
		}
		if filter.Matches(w) {
			t.Error("should exclude bare worktree when IncludeBare is false")
		}
	})

	t.Run("includes bare worktrees when IncludeBare is true", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			IncludeBare: true,
		}
		w := &domain.Worktree{
			Path: "/some/path",
			Bare: true,
		}
		if !filter.Matches(w) {
			t.Error("should include bare worktree when IncludeBare is true")
		}
	})

	t.Run("matches by single branch", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			Branches: []string{"feature-branch"},
		}
		w := &domain.Worktree{
			Path:   "/some/path",
			Branch: "feature-branch",
		}
		if !filter.Matches(w) {
			t.Error("should match worktree with matching branch")
		}
	})

	t.Run("does not match by different branch", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			Branches: []string{"main"},
		}
		w := &domain.Worktree{
			Path:   "/some/path",
			Branch: "feature-branch",
		}
		if filter.Matches(w) {
			t.Error("should not match worktree with different branch")
		}
	})

	t.Run("matches by multiple branches", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			Branches: []string{"main", "develop", "feature-branch"},
		}
		w := &domain.Worktree{
			Path:   "/some/path",
			Branch: "develop",
		}
		if !filter.Matches(w) {
			t.Error("should match worktree with branch in multiple branch list")
		}
	})

	t.Run("does not match when branch not in multiple branch list", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			Branches: []string{"main", "develop"},
		}
		w := &domain.Worktree{
			Path:   "/some/path",
			Branch: "feature-branch",
		}
		if filter.Matches(w) {
			t.Error("should not match worktree with branch not in branch list")
		}
	})

	t.Run("combines path prefix and branch filters", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			PathPrefix: "/home/user",
			Branches:   []string{"feature-branch"},
		}
		w := &domain.Worktree{
			Path:   "/home/user/project",
			Branch: "feature-branch",
		}
		if !filter.Matches(w) {
			t.Error("should match when both path prefix and branch match")
		}
	})

	t.Run("fails when path prefix matches but branch does not", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			PathPrefix: "/home/user",
			Branches:   []string{"main"},
		}
		w := &domain.Worktree{
			Path:   "/home/user/project",
			Branch: "feature-branch",
		}
		if filter.Matches(w) {
			t.Error("should not match when branch filter is not satisfied")
		}
	})

	t.Run("fails when branch matches but path prefix does not", func(t *testing.T) {
		filter := &domain.WorktreeFilter{
			PathPrefix: "/home/user",
			Branches:   []string{"feature-branch"},
		}
		w := &domain.Worktree{
			Path:   "/other/path/project",
			Branch: "feature-branch",
		}
		if filter.Matches(w) {
			t.Error("should not match when path prefix filter is not satisfied")
		}
	})
}
