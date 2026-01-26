package spice

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps git-spice operations
type Client struct {
	gsPath string
}

// NewClient creates a new git-spice client
func NewClient() (*Client, error) {
	// Try "git-spice" first (more specific, avoids Ghostscript conflict)
	// then fall back to "gs" alias if it's actually git-spice
	path, err := findGitSpice()
	if err != nil {
		return nil, err
	}
	return &Client{gsPath: path}, nil
}

// findGitSpice locates the git-spice executable, trying "git-spice" first,
// then "gs" if it's actually git-spice (not Ghostscript).
func findGitSpice() (string, error) {
	// Try "git-spice" first
	if path, err := exec.LookPath("git-spice"); err == nil {
		return path, nil
	}

	// Try "gs" but verify it's git-spice, not Ghostscript
	if path, err := exec.LookPath("gs"); err == nil {
		// Verify by checking version
		cmd := exec.Command(path, "--version")
		output, _ := cmd.CombinedOutput()
		if strings.Contains(string(output), "git-spice") {
			return path, nil
		}
	}

	return "", fmt.Errorf("git-spice not found in PATH (tried git-spice and gs)")
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
