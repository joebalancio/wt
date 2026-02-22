// Package git provides a client wrapper for git and gh CLI operations.
package git

import (
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
