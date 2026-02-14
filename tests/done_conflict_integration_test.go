package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
)

// TestIntegration_Done_MergeConflict tests the done workflow with merge conflicts:
// 1. Create a worktree for a new branch
// 2. Make changes to the same file in both main and feature
// 3. Run Done and verify it handles conflicts appropriately
func TestIntegration_Done_MergeConflict(t *testing.T) {
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

	// Create git client and service
	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create a file in main that will conflict
	conflictFile := filepath.Join(repoPath, "config.txt")
	if err := os.WriteFile(conflictFile, []byte("Main content\n"), 0o644); err != nil {
		t.Fatalf("failed to create conflict file: %v", err)
	}
	runGitCommand(t, repoPath, "add", "config.txt")
	runGitCommand(t, repoPath, "commit", "-m", "Add config in main")

	// Create a feature worktree
	featureBranch := "feature/test-conflict"
	featurePath := filepath.Join(repoPath, "feature-conflict")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	addedWorktree, err := service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Make conflicting changes in the feature worktree
	featureConflictFile := filepath.Join(featurePath, "config.txt")
	if err := os.WriteFile(featureConflictFile, []byte("Feature content\n"), 0o644); err != nil {
		t.Fatalf("failed to create conflicting file: %v", err)
	}

	runGitCommand(t, featurePath, "add", "config.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add config in feature")

	// Switch back to main branch and make more changes
	runGitCommand(t, repoPath, "checkout", "main")
	if err := os.WriteFile(conflictFile, []byte("Main updated content\n"), 0o644); err != nil {
		t.Fatalf("failed to update conflict file: %v", err)
	}
	runGitCommand(t, repoPath, "add", "config.txt")
	runGitCommand(t, repoPath, "commit", "-m", "Update config in main")

	// Run Done - this should fail due to merge conflict
	err = service.Done(ctx, addedWorktree.Path, featureBranch, false)
	if err == nil {
		t.Error("expected error for merge conflict, got nil")
	}
	if !strings.Contains(err.Error(), "merge") && !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected error message to mention merge/conflict, got: %v", err)
	}

	// Verify the worktree still exists (not removed due to failure)
	worktrees, err := service.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}

	var found bool
	for _, w := range worktrees {
		if w.Branch == featureBranch {
			found = true
			break
		}
	}
	if !found {
		t.Error("worktree was removed despite merge conflict")
	}

	// Clean up: force remove the worktree
	_ = client.RemoveWorktree(ctx, addedWorktree.Path, true)
	_ = client.DeleteBranch(ctx, featureBranch, true)
}
