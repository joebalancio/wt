// Package worktree provides the core service layer for worktree management operations.
// It orchestrates git worktree operations with configuration and hook execution,
// handling add, list, remove, and done workflows.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/joebalancio/wt/pkg/executor"
)

// ErrPathCollision is returned when a worktree path already exists from a different repo
var ErrPathCollision = errors.New("path collision detected")

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

		// Get repo info for namespace
		repoInfo, err := s.git.GetRepoInfo(ctx)
		if err != nil {
			return "", fmt.Errorf("getting repo info: %w", err)
		}
		repoName := filepath.Base(repoInfo.RootPath)
		targetPath := filepath.Join(dedicatedPath, repoName, branch)

		// Check for collision
		if err := s.checkPathCollision(targetPath, repoInfo.RootPath); err != nil {
			return "", err
		}

		return targetPath, nil
	}

	// per-repo mode
	repoInfo, err := s.git.GetRepoInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("getting repo info: %w", err)
	}
	return filepath.Join(repoInfo.RootPath, ".worktrees", branch), nil
}

// checkPathCollision verifies the target path doesn't exist from a different repo
func (s *Service) checkPathCollision(targetPath, currentRepoRoot string) error {
	// If path doesn't exist, no collision
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil
	}

	// Path exists - check if it's from the same repo
	existingRepo, err := s.getWorktreeOriginRepo(targetPath)
	if err != nil {
		// Can't determine origin, allow it (existing behavior)
		return nil
	}

	// Compare repo roots
	if existingRepo != "" && existingRepo != currentRepoRoot {
		return fmt.Errorf("%w: %s already exists from another repo (%s)\n\nOptions:\n  --path <explicit-path>  # specify a different path\n  wt config set worktree.location per-repo  # use per-repo mode",
			ErrPathCollision, targetPath, existingRepo)
	}

	return nil
}

// getWorktreeOriginRepo reads the .git file in a worktree to find the origin repo path
func (s *Service) getWorktreeOriginRepo(worktreePath string) (string, error) {
	gitFile := filepath.Join(worktreePath, ".git")
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return "", err
	}

	// Parse gitdir line: "gitdir: /path/to/repo/.git/worktrees/branch"
	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "gitdir: ") {
		return "", fmt.Errorf("invalid .git file format")
	}

	gitdir := strings.TrimPrefix(content, "gitdir: ")

	// Extract repo root from gitdir
	// gitdir is typically: /path/to/repo/.git/worktrees/branch
	// We need: /path/to/repo
	parts := strings.Split(gitdir, string(filepath.Separator))
	for i, part := range parts {
		if part == ".git" {
			repoPath := filepath.Join(parts[:i]...)
			if repoPath == "" {
				return "/", nil
			}
			return "/" + repoPath, nil
		}
	}

	return "", fmt.Errorf("could not parse repo path from gitdir: %s", gitdir)
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

// ResolveFromCWD resolves the worktree containing the given current working directory.
func (s *Service) ResolveFromCWD(ctx context.Context, cwd string) (*domain.Worktree, error) {
	worktrees, err := s.git.ListWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	var bestMatch *domain.Worktree
	for _, wt := range worktrees {
		if strings.HasPrefix(cwd, wt.Path) {
			if bestMatch == nil || len(wt.Path) > len(bestMatch.Path) {
				bestMatch = wt
			}
		}
	}

	if bestMatch == nil {
		return nil, errors.New("not in a worktree")
	}

	return bestMatch, nil
}

// RemoveEnhanced removes a worktree and its associated branch with safety checks.
func (s *Service) RemoveEnhanced(ctx context.Context, path string, force domain.ForceLevel) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	worktrees, err := s.git.ListWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("listing worktrees: %w", err)
	}

	targetWorktree, err := findWorktreeByPath(worktrees, path)
	if err != nil {
		return err
	}

	repoInfo, err := s.git.GetRepoInfo(ctx)
	if err != nil {
		return fmt.Errorf("getting repo info: %w", err)
	}
	if err := s.validateRemoveSafety(ctx, targetWorktree, path, repoInfo.DefaultBranch, force); err != nil {
		return err
	}

	safeDir := repoInfo.RootPath
	if samePathOrSubpath(safeDir, path) {
		for _, wt := range worktrees {
			if wt.Path != path && !samePathOrSubpath(wt.Path, path) {
				safeDir = wt.Path
				break
			}
		}
	}

	if err := chdirIfInsidePath(path, safeDir); err != nil {
		return err
	}

	forceRemove := force != domain.ForceNone
	if err := s.git.RemoveWorktree(ctx, path, forceRemove); err != nil {
		return fmt.Errorf("removing worktree: %w", err)
	}

	if err := s.git.DeleteBranch(ctx, targetWorktree.Branch, true); err != nil {
		return fmt.Errorf("deleting branch: %w", err)
	}

	return s.deleteRemoteBranchIfRequested(ctx, force, targetWorktree.Branch)
}

func findWorktreeByPath(worktrees []*domain.Worktree, path string) (*domain.Worktree, error) {
	for _, wt := range worktrees {
		if wt.Path == path {
			if wt.Detached() {
				return nil, errors.New("cannot remove: detached HEAD (no branch)")
			}
			return wt, nil
		}
	}
	return nil, fmt.Errorf("worktree at %q not found", path)
}

func (s *Service) validateRemoveSafety(ctx context.Context, target *domain.Worktree, path, defaultBranch string, force domain.ForceLevel) error {
	if target.Branch == defaultBranch {
		return fmt.Errorf("cannot remove default branch %q", target.Branch)
	}

	dirty, err := s.git.IsWorktreeDirty(ctx, path)
	if err != nil {
		return fmt.Errorf("checking worktree status: %w", err)
	}
	if dirty && force == domain.ForceNone {
		return errors.New("worktree has uncommitted changes. Use --force to remove anyway")
	}

	merged, err := s.git.IsBranchMerged(ctx, target.Branch)
	if err != nil {
		return fmt.Errorf("checking branch merge status: %w", err)
	}
	if !merged && force == domain.ForceNone {
		return fmt.Errorf("branch %q is not merged. Use --force to delete anyway", target.Branch)
	}
	return nil
}

func chdirIfInsidePath(targetPath, safeDir string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	cwdAbs, cwdErr := filepath.Abs(cwd)
	pathAbs, pathErr := filepath.Abs(targetPath)
	if cwdErr != nil || pathErr != nil {
		return nil
	}

	rel, relErr := filepath.Rel(pathAbs, cwdAbs)
	if relErr != nil {
		return nil
	}

	inside := rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
	if inside {
		if err := os.Chdir(safeDir); err != nil {
			return fmt.Errorf("changing directory to safe location: %w", err)
		}
	}
	return nil
}

func (s *Service) deleteRemoteBranchIfRequested(ctx context.Context, force domain.ForceLevel, branch string) error {
	if force != domain.ForceRemote {
		return nil
	}

	remoteExists, err := s.git.RemoteBranchExists(ctx, "origin", branch)
	if err != nil || !remoteExists {
		return nil
	}

	if err := s.git.DeleteRemoteBranch(ctx, "origin", branch); err != nil {
		return fmt.Errorf("deleting remote branch: %w", err)
	}
	return nil
}

func samePathOrSubpath(candidatePath, targetPath string) bool {
	candidateAbs, cErr := filepath.Abs(candidatePath)
	targetAbs, tErr := filepath.Abs(targetPath)
	if cErr != nil || tErr != nil {
		return candidatePath == targetPath
	}
	rel, err := filepath.Rel(targetAbs, candidateAbs)
	if err != nil {
		return candidateAbs == targetAbs
	}
	return rel == "." || rel == "" || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
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
