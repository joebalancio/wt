// Package spice provides a client wrapper for git-spice operations.
// Git-spice is used for branch stack management, enabling wt to create and manage
// stacked branches with unique nanoid suffixes for collision resistance.
package spice

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/joebalancio/wt/internal/config"
)

// Client wraps git-spice operations
type Client struct {
	gsPath string
}

// NewClient creates a new git-spice client with explicit config
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// No auto-detection at runtime
	if cfg.Spice.BinaryPath == "" {
		return nil, fmt.Errorf("git-spice not configured. Run 'wt init' to set up git-spice integration")
	}

	// Verify the configured path exists and is executable
	if _, err := exec.LookPath(cfg.Spice.BinaryPath); err != nil {
		return nil, fmt.Errorf("git-spice binary not found at %s: %w", cfg.Spice.BinaryPath, err)
	}

	// Verify it's actually git-spice
	if err := verifyGitSpice(cfg.Spice.BinaryPath); err != nil {
		return nil, fmt.Errorf("%s is not git-spice: %w", cfg.Spice.BinaryPath, err)
	}

	return &Client{gsPath: cfg.Spice.BinaryPath}, nil
}

// verifyGitSpice checks that the path is actually git-spice
func verifyGitSpice(path string) error {
	cmd := exec.Command(path, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run --version: %w", err)
	}
	if !strings.Contains(string(output), "git-spice") {
		return fmt.Errorf("version output doesn't contain 'git-spice'")
	}
	return nil
}

// GetVersion returns the git-spice version
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gsPath, "--version")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("getting git-spice version: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// CreateBranch creates a new stacked branch via git-spice
func (c *Client) CreateBranch(ctx context.Context, spec BranchCreateSpec) (*Branch, error) {
	args := []string{"branch", spec.Name}

	if spec.Base != "" {
		args = append(args, "--base", spec.Base)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gsPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("creating branch with git-spice: %w: %s", err, stderr.String())
	}

	// Parse the created branch name from output
	// git-spice typically echoes the created branch
	branchName := strings.TrimSpace(stdout.String())
	if branchName == "" {
		branchName = spec.Name
	}

	return &Branch{Name: branchName}, nil
}

// GetStack returns the current stack of branches
func (c *Client) GetStack(ctx context.Context) ([]*Branch, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, c.gsPath, "stack", "list")
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("getting stack: %w", err)
	}

	return parseStackOutput(stdout.String()), nil
}

// BranchCreateSpec defines parameters for creating a branch
type BranchCreateSpec struct {
	Name string // Branch name (required)
	Base string // Base branch (optional, defaults to current)
}

// Branch represents a git-spice branch
type Branch struct {
	Name     string    // Branch name
	IsRoot   bool      // Is this the root of the stack
	IsHead   bool      // Is this the current branch
	Children []*Branch // Child branches in the stack
}

func parseStackOutput(output string) []*Branch {
	// Parse git-spice stack list output
	// Format is tree-like with indentation
	lines := strings.Split(output, "\n")
	branches := make([]*Branch, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Simple parsing - will be enhanced in later tasks
		branches = append(branches, &Branch{Name: line})
	}

	return branches
}

// GetStackLevel returns the stack level (depth) of a branch in the stack
// Root branches (like main) return 0, first stacked branch returns 1, etc.
func (c *Client) GetStackLevel(stack []*Branch, branchName string) (int, error) {
	for i, b := range stack {
		if b.Name == branchName {
			// Stack level is the index (0 for root/main, 1 for first stacked, etc.)
			return i, nil
		}
	}
	return 0, fmt.Errorf("branch %q not found in stack", branchName)
}
