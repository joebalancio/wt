package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
)

// TestIntegration_Done_WithHooks tests the done workflow with hooks:
// 1. Create a worktree with on_worktree_done hook configured
// 2. Make changes and commit in the worktree
// 3. Run Done and verify hooks are executed
func TestIntegration_Done_WithHooks(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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

	// Create a marker file that will be touched by the hook
	markerFile := filepath.Join(repoPath, "done-hook-marker.txt")

	// Create config with done hook
	cfg := config.DefaultConfig()
	cfg.Hooks.OnWorktreeDone = []config.Hook{
		{
			Run: "touch " + markerFile,
		},
	}

	// Create git client and service
	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a feature worktree
	featureBranch := "feature/test-hook"
	featurePath := filepath.Join(repoPath, "feature-hook")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	addedWorktree, err := service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Make changes and commit in the feature worktree
	featureFile := filepath.Join(featurePath, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("Feature content\n"), 0o644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}

	runGitCommand(t, featurePath, "add", "feature.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add feature")

	// Switch back to main branch
	runGitCommand(t, repoPath, "checkout", "main")

	// Run Done - this should trigger the hook
	err = service.Done(ctx, addedWorktree.Path, featureBranch, false)
	if err != nil {
		t.Fatalf("failed to complete worktree: %v", err)
	}

	// Verify the hook was executed by checking for the marker file
	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Error("done hook was not executed - marker file not found")
	}
}
