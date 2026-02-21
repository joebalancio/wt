package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/pkg/domain"
)

// TestIntegration_AddFromWorktree_Error tests that wt add fails
// with a helpful error when run from inside a worktree
func TestIntegration_AddFromWorktree_Error(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Build the wt binary first (from project root)
	// Get project root by finding go.mod
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("failed to get project root: %v", err)
	}

	binPath := filepath.Join(projectRoot, "bin", "wt")
	buildCmd := exec.Command("make", "build")
	buildCmd.Dir = projectRoot
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build wt binary: %v\nOutput: %s", err, buildOutput)
	}

	// Create main repo
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	// Create a worktree
	worktreePath := filepath.Join(repoPath, "existing-worktree")
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/existing",
		Path:   worktreePath,
		Base:   "main",
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	_, err = client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer client.RemoveWorktree(ctx, worktreePath, true)

	// Change to the worktree directory
	if err := os.Chdir(worktreePath); err != nil {
		t.Fatalf("failed to change to worktree directory: %v", err)
	}

	// Try to run wt add (should fail with helpful error)
	cmd := exec.Command(binPath, "add", "feature/nested")
	output, err := cmd.CombinedOutput()

	// Should have failed
	if err == nil {
		t.Error("wt add should fail when run from inside a worktree")
	}

	// Check error message contains key information
	errMsg := string(output)
	if !strings.Contains(errMsg, "cannot add worktree from inside") {
		t.Errorf("error message should contain 'cannot add worktree from inside', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, repoPath) {
		t.Errorf("error message should show main repo path %q, got: %s", repoPath, errMsg)
	}
}
