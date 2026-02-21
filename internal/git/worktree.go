// Package git provides a client wrapper for git worktree operations.
// It wraps the git CLI to provide operations like listing, adding, and removing worktrees,
// as well as branch management and repository information retrieval.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joebalancio/wt/pkg/domain"
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

	// Check if branch already exists to determine correct syntax:
	// - Existing branch: git worktree add <path> <branch>
	// - New branch: git worktree add -b <branch> <path> [<start-point>]
	branchExists, err := c.BranchExists(ctx, spec.Branch)
	if err != nil {
		return nil, fmt.Errorf("checking if branch exists: %w", err)
	}

	if !branchExists {
		args = append(args, "-b", spec.Branch)
	}

	if spec.Path == "" {
		return nil, fmt.Errorf("spec.Path is required")
	}
	args = append(args, spec.Path)
	path := spec.Path

	if branchExists {
		// Existing branch: pass branch name as commit-ish
		args = append(args, spec.Branch)
	} else {
		// New branch: optionally pass base as start point
		if spec.Base != "" {
			args = append(args, spec.Base)
		}
	}

	if !spec.Checkout {
		args = append(args, "--no-checkout")
	}

	// --track only applies when creating a new branch
	if !branchExists && spec.Track != nil {
		args = append(args, "--track", *spec.Track)
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("adding worktree: %w: %s", err, stderr.String())
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

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("removing worktree: %w: %s", err, stderr.String())
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
		if strings.Contains(errMsg, "unknown revision") ||
			strings.Contains(errMsg, "needed but is an unborn ref") ||
			strings.Contains(errMsg, "Needed a single revision") {
			return false, nil
		}
		// Some other error occurred (e.g., git not found, not in a git repo)
		return false, fmt.Errorf("checking if branch %q exists: %w", branch, err)
	}
	return true, nil
}

// GetCurrentBranch returns the current branch name
func (c *Client) GetCurrentBranch(ctx context.Context) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, "symbolic-ref", "--short", "HEAD")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Might be detached HEAD
		return "", fmt.Errorf("not on any branch: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
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

// DeleteBranch deletes a git branch.
// The force flag determines whether to use -D (force) or -d (safe) deletion.
func (c *Client) DeleteBranch(ctx context.Context, branch string, force bool) error {
	args := []string{"branch"}
	if force {
		args = append(args, "-D")
	} else {
		args = append(args, "-d")
	}
	args = append(args, branch)
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deleting branch %q: %w: %s", branch, err, stderr.String())
	}
	return nil
}

// SquashMerge performs a squash merge from the source branch into the current branch.
// This stages the changes without creating a commit.
func (c *Client) SquashMerge(ctx context.Context, sourceBranch string) error {
	args := []string{"merge", "--squash", sourceBranch}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("squash merge %q: %w: %s", sourceBranch, err, stderr.String())
	}
	return nil
}

// CreateSquashCommit creates a commit with the given message for staged squash merge changes.
func (c *Client) CreateSquashCommit(ctx context.Context, message string) error {
	args := []string{"commit", "-m", message}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating squash commit: %w: %s", err, stderr.String())
	}
	return nil
}

// IsWorktreeDirty checks if a worktree has any uncommitted changes.
// Returns true if there are modified, staged, or untracked files.
func (c *Client) IsWorktreeDirty(ctx context.Context, path string) (bool, error) {
	args := []string{"-C", path, "status", "--porcelain"}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("checking worktree status: %w: %s", err, stderr.String())
	}
	return stdout.Len() > 0, nil
}

// IsBranchMerged checks if a branch is merged into the default branch.
func (c *Client) IsBranchMerged(ctx context.Context, branch string) (bool, error) {
	repoInfo, err := c.GetRepoInfo(ctx)
	if err != nil {
		return false, fmt.Errorf("getting repo info: %w", err)
	}

	// merge-base --is-ancestor exits 0 when branch is merged into defaultBranch.
	args := []string{"merge-base", "--is-ancestor", branch, repoInfo.DefaultBranch}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Exit status 1 from --is-ancestor means "not merged", not execution error.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("checking branch merge status %q into %q: %w: %s", branch, repoInfo.DefaultBranch, err, stderr.String())
	}
	return true, nil
}

// RemoteBranchExists checks if a branch exists on the remote.
func (c *Client) RemoteBranchExists(ctx context.Context, remote, branch string) (bool, error) {
	remoteRef := fmt.Sprintf("refs/remotes/%s/%s", remote, branch)
	args := []string{"rev-parse", "--verify", remoteRef}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "unknown revision") || strings.Contains(errMsg, "Needed a single revision") {
			return false, nil
		}
		return false, fmt.Errorf("checking remote branch %s/%s: %w", remote, branch, err)
	}
	return true, nil
}

// DeleteRemoteBranch deletes a branch from the remote.
func (c *Client) DeleteRemoteBranch(ctx context.Context, remote, branch string) error {
	args := []string{"push", remote, "--delete", branch}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deleting remote branch %s/%s: %w: %s", remote, branch, err, stderr.String())
	}
	return nil
}

// getOutput runs a git command and returns its trimmed stdout
func (c *Client) getOutput(ctx context.Context, args ...string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// IsInWorktree checks if the current directory is inside a git worktree.
// Returns true if in a worktree, false if in main repo.
// Also returns the main repository root path.
func (c *Client) IsInWorktree(ctx context.Context) (bool, string, error) {
	// Get current toplevel (worktree root or main repo root)
	toplevel, err := c.getOutput(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return false, "", fmt.Errorf("getting toplevel: %w", err)
	}

	// Get common git dir (always points to main repo's .git)
	gitCommonDir, err := c.getOutput(ctx, "rev-parse", "--git-common-dir")
	if err != nil {
		return false, "", fmt.Errorf("getting git-common-dir: %w", err)
	}

	// Convert gitCommonDir to absolute path if it's relative
	if !filepath.IsAbs(gitCommonDir) {
		// gitCommonDir is relative to toplevel
		gitCommonDir = filepath.Join(toplevel, gitCommonDir)
	}

	// Main repo root is parent of .git directory
	mainRepoRoot := filepath.Dir(gitCommonDir)

	// Clean paths for comparison
	toplevel = filepath.Clean(toplevel)
	mainRepoRoot = filepath.Clean(mainRepoRoot)

	// If toplevel differs from main repo root, we're in a worktree
	inWorktree := toplevel != mainRepoRoot
	return inWorktree, mainRepoRoot, nil
}

func parseWorktreeOutput(output string) ([]*domain.Worktree, error) {
	var worktrees []*domain.Worktree
	currentIndex := -1

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
