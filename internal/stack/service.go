package stack

import (
	"context"
	"fmt"

	"github.com/aidarkhanov/nanoid"
	"github.com/user/wt/internal/spice"
	"github.com/user/wt/pkg/domain"
)

// SpiceClient defines the interface for git-spice operations
type SpiceClient interface {
	CreateBranch(ctx context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error)
	GetStack(ctx context.Context) ([]*spice.Branch, error)
}

// Service provides stack-related operations
type Service struct {
	spiceClient SpiceClient
}

// NewService creates a new stack service
func NewService(client SpiceClient) (*Service, error) {
	// If no client provided, create a default one
	if client == nil {
		defaultClient, err := spice.NewClient()
		if err != nil {
			return nil, fmt.Errorf("creating git-spice client: %w", err)
		}
		client = defaultClient
	}

	return &Service{
		spiceClient: client,
	}, nil
}

// GenerateBranchSuffix generates a 4-character nanoid suffix
func (s *Service) GenerateBranchSuffix() string {
	suffix, err := nanoid.Generate(nanoid.DefaultAlphabet, 4)
	if err != nil {
		// Fallback to a simple timestamp-based suffix if nanoid fails
		return "0000"
	}
	return suffix
}

// BuildStackBranchName constructs a stack branch name from current branch and optional suffix name
func (s *Service) BuildStackBranchName(currentBranch, suffixName string) string {
	base := currentBranch
	if suffixName != "" {
		base = fmt.Sprintf("%s-%s", currentBranch, suffixName)
	}
	suffix := s.GenerateBranchSuffix()
	return fmt.Sprintf("%s-%s", base, suffix)
}

// CreateStackBranch creates a new stacked branch using git-spice
func (s *Service) CreateStackBranch(ctx context.Context, currentBranch, suffixName string) (*domain.StackBranch, error) {
	branchName := s.BuildStackBranchName(currentBranch, suffixName)

	spec := spice.BranchCreateSpec{
		Name: branchName,
		// Base defaults to current branch in git-spice
	}

	branch, err := s.spiceClient.CreateBranch(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("creating stack branch: %w", err)
	}

	return &domain.StackBranch{
		Name:   branch.Name,
		IsRoot: branch.IsRoot,
		IsHead: branch.IsHead,
	}, nil
}

// GetStack returns the current stack of branches
func (s *Service) GetStack(ctx context.Context) ([]*domain.StackBranch, error) {
	spiceBranches, err := s.spiceClient.GetStack(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting stack: %w", err)
	}

	stackBranches := make([]*domain.StackBranch, 0, len(spiceBranches))
	for _, sb := range spiceBranches {
		stackBranches = append(stackBranches, convertToDomainStackBranch(sb))
	}

	return stackBranches, nil
}

// convertToDomainStackBranch converts a spice.Branch to domain.StackBranch
func convertToDomainStackBranch(sb *spice.Branch) *domain.StackBranch {
	if sb == nil {
		return nil
	}

	children := make([]*domain.StackBranch, 0, len(sb.Children))
	for _, child := range sb.Children {
		children = append(children, convertToDomainStackBranch(child))
	}

	return &domain.StackBranch{
		Name:     sb.Name,
		IsRoot:   sb.IsRoot,
		IsHead:   sb.IsHead,
		Path:     "", // Path is determined by worktree lookup, not from spice
		Children: children,
	}
}
