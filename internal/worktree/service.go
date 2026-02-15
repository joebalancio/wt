// Package worktree provides the core service layer for worktree management operations.
// It orchestrates git worktree operations with configuration and hook execution,
// handling add, list, remove, and done workflows.
package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/joebalancio/wt/pkg/executor"
)

// Service provides worktree management operations
type Service struct {
	git git.GitClient
	cfg *config.Config
}

// NewService creates a new worktree service
func NewService(gitClient git.GitClient, cfg *config.Config) (*Service, error) {
	if gitClient == nil {
		return nil, fmt.Errorf("gitClient cannot be nil")
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Service{
		git: gitClient,
		cfg: cfg,
	}, nil
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
	// Resolve path if not specified
	if spec.Path == "" {
		resolvedPath, err := s.ResolvePath(ctx, spec.Branch, "")
		if err != nil {
			return nil, err
		}
		spec.Path = resolvedPath
	}

	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}

	worktree, err := s.git.AddWorktree(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("adding worktree: %w", err)
	}

	return worktree, nil
}

// ResolvePath returns the worktree path for a branch.
// If explicitPath is provided, it's used as-is.
// Otherwise, path is resolved from config based on worktree.location setting.
func (s *Service) ResolvePath(ctx context.Context, branch string, explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}

	if s.cfg.Worktree.IsDedicated() {
		dedicatedPath := s.cfg.Worktree.GetDedicatedPath()
		// Expand ~ to home directory
		if strings.HasPrefix(dedicatedPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("getting home directory: %w", err)
			}
			dedicatedPath = filepath.Join(home, dedicatedPath[2:])
		}
		return filepath.Join(dedicatedPath, branch), nil
	}

	// per-repo mode
	repoInfo, err := s.git.GetRepoInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("getting repo info: %w", err)
	}
	return filepath.Join(repoInfo.RootPath, ".worktrees", branch), nil
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

// Done completes a worktree by merging it, creating a commit, and removing it
func (s *Service) Done(ctx context.Context, worktreePath, branch string, force bool) error {
	// Validate branch exists
	exists, err := s.git.BranchExists(ctx, branch)
	if err != nil {
		return fmt.Errorf("checking branch existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("branch %q does not exist", branch)
	}

	// Check if worktree is dirty BEFORE merging
	dirty, err := s.git.IsWorktreeDirty(ctx, worktreePath)
	if err != nil {
		return fmt.Errorf("checking worktree status: %w", err)
	}
	if dirty && !force {
		return fmt.Errorf("worktree is dirty (use --force to proceed anyway)")
	}

	// Perform squash merge
	if err := s.git.SquashMerge(ctx, branch); err != nil {
		return fmt.Errorf("squash merge failed: %w", err)
	}

	// Create the merge commit
	commitMessage := fmt.Sprintf("Merge %s", branch)
	if err := s.git.CreateSquashCommit(ctx, commitMessage); err != nil {
		return fmt.Errorf("creating merge commit failed: %w", err)
	}

	// Run done hooks with template variables
	if len(s.cfg.Hooks.OnWorktreeDone) > 0 {
		templateVars := map[string]string{
			"branch":        branch,
			"worktree_path": worktreePath,
		}
		runner := executor.NewHookRunner(worktreePath, templateVars)
		if err := runner.RunHooks(ctx, s.cfg.Hooks.OnWorktreeDone); err != nil {
			// Log hook failures as warnings but don't block cleanup
			fmt.Fprintf(os.Stderr, "Warning: done hooks failed: %v\n", err)
		}
	}

	// Remove the worktree (always use force since we just merged and want to clean up)
	if err := s.git.RemoveWorktree(ctx, worktreePath, true); err != nil {
		return fmt.Errorf("removing worktree: %w", err)
	}

	// Delete the branch (use force flag: if worktree was dirty, user explicitly confirmed with --force)
	if err := s.git.DeleteBranch(ctx, branch, true); err != nil {
		return fmt.Errorf("deleting branch: %w", err)
	}

	// Run remove hooks with template variables
	if len(s.cfg.Hooks.OnWorktreeRemove) > 0 {
		templateVars := map[string]string{
			"branch":        branch,
			"worktree_path": worktreePath,
		}
		// Note: worktree is gone, so use empty working dir
		runner := executor.NewHookRunner("", templateVars)
		if err := runner.RunHooks(ctx, s.cfg.Hooks.OnWorktreeRemove); err != nil {
			// Log hook failures as warnings but don't fail
			fmt.Fprintf(os.Stderr, "Warning: remove hooks failed: %v\n", err)
		}
	}

	return nil
}
