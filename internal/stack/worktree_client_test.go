package stack

import (
	"context"
	"testing"

	"github.com/joebalancio/wt/pkg/domain"
)

func TestWorktreeClientInterface(_ *testing.T) {
	var _ WorktreeClient = (*mockWorktreeClient)(nil)
}

type mockWorktreeClient struct {
	addFunc         func(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	resolvePathFunc func(ctx context.Context, branch, explicitPath string) (string, error)
}

func (m *mockWorktreeClient) Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	if m.addFunc != nil {
		return m.addFunc(ctx, spec)
	}
	return &domain.Worktree{Path: "/mock/path", Branch: spec.Branch}, nil
}

func (m *mockWorktreeClient) ResolvePath(ctx context.Context, branch, explicitPath string) (string, error) {
	if m.resolvePathFunc != nil {
		return m.resolvePathFunc(ctx, branch, explicitPath)
	}
	return "/mock/path/" + branch, nil
}
