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
