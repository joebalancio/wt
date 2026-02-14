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

// TestIntegration_Done_Basic tests the basic done workflow:
// 1. Create a worktree for a new branch
// 2. Make changes and commit in the worktree
// 3. Run Done to merge and remove the worktree
// 4. Verify the branch is merged and worktree is removed
func TestIntegration_Done_Basic(t *testing.T) {
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

	// Step 1: Create a feature worktree
	featureBranch := "feature/test-feature"
	featurePath := filepath.Join(repoPath, "feature-test")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	addedWorktree, err := service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Step 2: Make changes and commit in the feature worktree
	featureFile := filepath.Join(featurePath, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("Feature content\n"), 0o644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}

	runGitCommand(t, featurePath, "add", "feature.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add feature")

	// Step 3: Run Done to merge and remove the worktree
	// We need to switch back to main branch first
	runGitCommand(t, repoPath, "checkout", "main")

	err = service.Done(ctx, addedWorktree.Path, featureBranch, false)
	if err != nil {
		t.Fatalf("failed to complete worktree: %v", err)
	}

	// Step 4: Verify the worktree is removed
	worktrees, err := service.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list worktrees after done: %v", err)
	}

	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree after done, got %d", len(worktrees))
	}

	// Step 5: Verify the branch is deleted
	exists, err := client.BranchExists(ctx, featureBranch)
	if err != nil {
		t.Fatalf("failed to check branch existence: %v", err)
	}
	if exists {
		t.Error("feature branch still exists after done")
	}

	// Step 6: Verify the merge commit exists in main
	// Check that the feature file exists in main
	mergedFile := filepath.Join(repoPath, "feature.txt")
	content, err := os.ReadFile(mergedFile)
	if err != nil {
		t.Errorf("feature file not found in main after merge: %v", err)
	}
	if string(content) != "Feature content\n" {
		t.Errorf("unexpected content in merged file: %q", string(content))
	}
}
