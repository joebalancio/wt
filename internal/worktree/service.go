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
