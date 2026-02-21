// Package git provides a client for interacting with git worktrees via the git CLI.
//
// The GitClient interface defines operations for managing worktrees, with
// support for context cancellation and proper error handling.
package git

import (
	"context"

	"github.com/joebalancio/wt/pkg/domain"
)

// GitClient defines the interface for git operations
// revive:disable:exported Type name stutter is acceptable for clarity
type GitClient interface {
	ListWorktrees(ctx context.Context) ([]*domain.Worktree, error)
	AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	RemoveWorktree(ctx context.Context, path string, force bool) error
	GetRepoInfo(ctx context.Context) (*domain.GitRepo, error)
	BranchExists(ctx context.Context, branch string) (bool, error)
	GetCurrentBranch(ctx context.Context) (string, error)
	DeleteBranch(ctx context.Context, branch string, force bool) error
	SquashMerge(ctx context.Context, sourceBranch string) error
	CreateSquashCommit(ctx context.Context, message string) error
	IsWorktreeDirty(ctx context.Context, path string) (bool, error)
	IsBranchMerged(ctx context.Context, branch string) (bool, error)
	RemoteBranchExists(ctx context.Context, remote, branch string) (bool, error)
	DeleteRemoteBranch(ctx context.Context, remote, branch string) error
	// IsInWorktree checks if the current directory is inside a git worktree.
	// Returns true if in a worktree, false if in main repo.
	// Also returns the main repository root path.
	IsInWorktree(ctx context.Context) (inWorktree bool, mainRepoRoot string, err error)
	// ListAllBranches returns all local and remote branches, deduplicated.
	ListAllBranches(ctx context.Context) ([]string, error)
}
