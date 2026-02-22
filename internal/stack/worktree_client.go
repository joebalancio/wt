package stack

import (
	"context"

	"github.com/joebalancio/wt/pkg/domain"
)

// WorktreeClient defines the worktree operations stack service needs.
type WorktreeClient interface {
	Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	ResolvePath(ctx context.Context, branch, explicitPath string) (string, error)
}
