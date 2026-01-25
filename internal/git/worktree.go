package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/user/wt/pkg/domain"
)

// Client wraps git operations
type Client struct {
	gitPath string
}

// NewClient creates a new git client
func NewClient() (*Client, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git not found in PATH: %w", err)
	}
	return &Client{gitPath: path}, nil
}

// ListWorktrees returns all worktrees for the current repository
func (c *Client) ListWorktrees(ctx context.Context) ([]*domain.Worktree, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "worktree", "list", "--porcelain")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("listing worktrees: %w", err)
	}

	return parseWorktreeOutput(stdout.String())
}

// AddWorktree creates a new worktree
func (c *Client) AddWorktree(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid spec: %w", err)
	}

	args := []string{"worktree", "add"}

	if spec.Force {
		args = append(args, "--force")
	}

	if spec.Base != "" {
		args = append(args, "-b", spec.Branch, spec.Base)
	} else {
		args = append(args, spec.Branch)
	}

	path := spec.Path
	if path == "" {
		// Auto-generate path from branch name
		path = filepath.Join(".", spec.Branch)
	}

	args = append(args, path)

	if !spec.Checkout {
		args = append(args, "--no-checkout")
	}

	if spec.Track != nil {
		args = append(args, "--track", *spec.Track)
	}

	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("adding worktree: %w", err)
	}

	// Convert path to absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	return &domain.Worktree{
		Path:   absPath,
		Branch: spec.Branch,
	}, nil
}

// RemoveWorktree removes a worktree
func (c *Client) RemoveWorktree(ctx context.Context, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing worktree: %w", err)
	}
	return nil
}

// GetRepoInfo returns information about the git repository
func (c *Client) GetRepoInfo(ctx context.Context) (*domain.GitRepo, error) {
	// Get root path
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "rev-parse", "--show-toplevel")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("getting repo root: %w", err)
	}

	rootPath := strings.TrimSpace(stdout.String())

	// Get default branch
	stdout.Reset()
	cmd = exec.CommandContext(ctx, c.gitPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	cmd.Stdout = &stdout

	// Fallback to "main" if no origin/HEAD configured (e.g., fresh repo with no remote)
	defaultBranch := "main"
	if err := cmd.Run(); err == nil {
		// Format: refs/remotes/origin/main
		ref := strings.TrimSpace(stdout.String())
		parts := strings.Split(ref, "/")
		if len(parts) > 0 {
			defaultBranch = parts[len(parts)-1]
		}
	}

	return &domain.GitRepo{
		RootPath:      rootPath,
		DefaultBranch: defaultBranch,
		IsBare:        false,
	}, nil
}

// BranchExists checks if a branch exists
func (c *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// rev-parse returns non-zero if branch doesn't exist
		// Check if the error is specifically about the branch not existing
		errMsg := stderr.String()
		if strings.Contains(errMsg, "unknown revision") || strings.Contains(errMsg, "needed but is an unborn ref") {
			return false, nil
		}
		// Some other error occurred (e.g., git not found, not in a git repo)
		return false, fmt.Errorf("checking if branch %q exists: %w", branch, err)
	}
	return true, nil
}

// PruneWorktrees removes stale worktrees.
//
// Note: This is a utility method that is not part of the GitClient interface.
// It can be called directly for maintenance operations but is not required
// for core worktree operations.
func (c *Client) PruneWorktrees(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.gitPath, "worktree", "prune")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pruning worktrees: %w", err)
	}
	return nil
}

func parseWorktreeOutput(output string) ([]*domain.Worktree, error) {
	var worktrees []*domain.Worktree
	var currentIndex int = -1

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		switch key {
		case "worktree":
			worktrees = append(worktrees, &domain.Worktree{Path: value})
			currentIndex = len(worktrees) - 1
		case "branch":
			if currentIndex >= 0 {
				worktrees[currentIndex].Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "HEAD":
			if currentIndex >= 0 {
				worktrees[currentIndex].Head = value
			}
		case "detached":
			if currentIndex >= 0 {
				worktrees[currentIndex].Head = "detached"
			}
		}
	}

	return worktrees, nil
}
