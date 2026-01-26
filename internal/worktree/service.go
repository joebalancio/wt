package worktree

import (
	"context"
	"fmt"

	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/pkg/domain"
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
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}

	worktree, err := s.git.AddWorktree(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("adding worktree: %w", err)
	}

	return worktree, nil
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
