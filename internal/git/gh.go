// Package git provides a client wrapper for git and gh CLI operations.
package git

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// GhClient wraps the GitHub CLI (gh) for repository operations.
type GhClient struct {
	ghPath string
}

// NewGhClient creates a new GitHub CLI client.
// Returns error if gh is not found in PATH.
func NewGhClient() (*GhClient, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return nil, fmt.Errorf("gh not found in PATH: %w", err)
	}
	return &GhClient{ghPath: path}, nil
}

// IsAvailable returns true if the gh client is usable.
func (c *GhClient) IsAvailable() bool {
	return c.ghPath != ""
}

// IsBranchPRMerged checks if a branch has an associated PR that was merged.
func (c *GhClient) IsBranchPRMerged(ctx context.Context, branch string) (bool, error) {
	if !c.IsAvailable() {
		return false, fmt.Errorf("gh client not available")
	}

	cmd := exec.CommandContext(ctx, c.ghPath, "pr", "view", branch, "--json", "state")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("gh pr view: %w", err)
	}

	state, err := parsePRStateJSON(string(output))
	if err != nil {
		return false, fmt.Errorf("parsing PR state: %w", err)
	}

	return state == "MERGED", nil
}

// parsePRStateJSON extracts the state field from gh pr view --json state output.
func parsePRStateJSON(input string) (string, error) {
	var result struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return "", fmt.Errorf("unmarshaling JSON: %w", err)
	}
	return result.State, nil
}
