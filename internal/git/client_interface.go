// Package git provides a client for interacting with git worktrees via the git CLI.
//
// The GitClient interface defines operations for managing worktrees, with
// support for context cancellation and proper error handling.
package git

import (
	"context"

	"github.com/user/wt/pkg/domain"
)

// GitClient defines the interface for git operations
// revive:disable:exported Type name stutter is acceptable for clarity
type GitClient interface {
	ListWorktrees(ctx context.Context) ([]*domain.Worktree, error)
	AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	RemoveWorktree(ctx context.Context, path string, force bool) error
	GetRepoInfo(ctx context.Context) (*domain.GitRepo, error)
	BranchExists(ctx context.Context, branch string) (bool, error)
}
