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

// TestIntegration_Done_DirtyWorktree tests the done workflow with dirty worktree:
// 1. Create a worktree for a new branch
// 2. Make uncommitted changes in the worktree
// 3. Run Done without --force and verify it fails
// 4. Run Done with --force and verify it succeeds
func TestIntegration_Done_DirtyWorktree(t *testing.T) {
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

	// Create a feature worktree
	featureBranch := "feature/test-dirty"
	featurePath := filepath.Join(repoPath, "feature-dirty")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	addedWorktree, err := service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Make committed changes
	featureFile := filepath.Join(featurePath, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("Feature content\n"), 0o644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}
	runGitCommand(t, featurePath, "add", "feature.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add feature")

	// Make uncommitted changes (dirty worktree)
	dirtyFile := filepath.Join(featurePath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("Uncommitted content\n"), 0o644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Switch back to main branch
	runGitCommand(t, repoPath, "checkout", "main")

	// Test 1: Run Done without --force - should fail
	err = service.Done(ctx, addedWorktree.Path, featureBranch, false)
	if err == nil {
		t.Error("expected error for dirty worktree without force, got nil")
	}
	if !strings.Contains(err.Error(), "dirty") {
		t.Errorf("expected error message to mention 'dirty', got: %v", err)
	}

	// Verify the worktree still exists
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
		t.Error("worktree was removed despite being dirty")
	}

	// Test 2: Run Done with --force - should succeed
	err = service.Done(ctx, addedWorktree.Path, featureBranch, true)
	if err != nil {
		t.Fatalf("failed to complete worktree with force: %v", err)
	}

	// Verify the worktree is removed
	worktreesAfterDone, err := service.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list worktrees after done: %v", err)
	}

	if len(worktreesAfterDone) != 1 {
		t.Errorf("expected 1 worktree after done with force, got %d", len(worktreesAfterDone))
	}

	// Verify the branch is deleted
	exists, err := client.BranchExists(ctx, featureBranch)
	if err != nil {
		t.Fatalf("failed to check branch existence: %v", err)
	}
	if exists {
		t.Error("feature branch still exists after done with force")
	}
}
