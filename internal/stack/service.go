package stack

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aidarkhanov/nanoid"
	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
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
	git   git.GitClient
	spice SpiceClient
	cfg   *config.Config
}

// NewService creates a new stack service
func NewService(gitClient git.GitClient, spiceClient SpiceClient, cfg *config.Config) (*Service, error) {
	if gitClient == nil {
		return nil, fmt.Errorf("gitClient cannot be nil")
	}
	if spiceClient == nil {
		return nil, fmt.Errorf("spiceClient cannot be nil")
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	return &Service{
		git:   gitClient,
		spice: spiceClient,
		cfg:   cfg,
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
	suffix := s.GenerateBranchSuffix()
	if suffixName == "" {
		// Auto-suffix: feat/auth -> feat/auth-xY7k
		return fmt.Sprintf("%s-%s", currentBranch, suffix)
	}
	// Named suffix: feat/auth -> feat/auth-api-k9P2
	return fmt.Sprintf("%s-%s-%s", currentBranch, suffixName, suffix)
}

// StackBranchSpec defines parameters for creating a stack branch
type StackBranchSpec struct {
	Name string // Optional named suffix (e.g., "api" for feat/auth-api-xxxx)
	Base string // Optional base branch (defaults to current)
}

// CreateStackBranch creates a new stacked branch using git-spice
func (s *Service) CreateStackBranch(ctx context.Context, spec StackBranchSpec) (*domain.StackBranch, error) {
	// Get current branch
	currentBranch, err := s.git.GetCurrentBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting current branch: %w", err)
	}

	// Build the new branch name
	branchName := s.BuildStackBranchName(currentBranch, spec.Name)

	// Create branch via git-spice
	spiceSpec := spice.BranchCreateSpec{
		Name: branchName,
		Base: spec.Base,
	}

	branch, err := s.spice.CreateBranch(ctx, spiceSpec)
	if err != nil {
		return nil, fmt.Errorf("creating stack branch: %w", err)
	}

	// Generate worktree path
	worktreePath := s.getWorktreePath(branch.Name)

	return &domain.StackBranch{
		Name:   branch.Name,
		IsRoot: branch.IsRoot,
		IsHead: branch.IsHead,
		Path:   worktreePath,
	}, nil
}

// GetStack returns the current stack of branches
func (s *Service) GetStack(ctx context.Context) ([]*domain.StackBranch, error) {
	spiceBranches, err := s.spice.GetStack(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting stack: %w", err)
	}

	stackBranches := make([]*domain.StackBranch, 0, len(spiceBranches))
	for _, sb := range spiceBranches {
		stackBranches = append(stackBranches, convertToDomainStackBranch(sb))
	}

	return stackBranches, nil
}

// GetWorktreePathForBranch returns the worktree path for a given branch name
func (s *Service) GetWorktreePathForBranch(branch string) string {
	return s.getWorktreePath(branch)
}

// getWorktreePath returns the worktree path for a branch
func (s *Service) getWorktreePath(branch string) string {
	if s.cfg.Worktree.IsDedicated() {
		return filepath.Join(s.cfg.Worktree.GetDedicatedPath(), branch)
	}
	// per-repo mode
	repoInfo, _ := s.git.GetRepoInfo(context.Background())
	return filepath.Join(repoInfo.RootPath, ".worktrees", branch)
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
