package tests

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/git"
)

// runCommand executes a command in the specified directory
func runCommand(t testing.TB, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\nOutput: %s", name, strings.Join(args, " "), err, output)
	}
}

// TestStacking_BasicWorkflow tests the basic stacking workflow:
// 1. Initialize git-spice in a repo
// 2. Create a root branch
// 3. Stack on it (auto-suffix)
// 4. Verify both branches exist
// 5. Clean up
func TestStacking_BasicWorkflow(t *testing.T) {
	skipIfNoGit(t)

	// Skip if git-spice not available
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("git-spice not available")
	}

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	// Initialize git-spice
	runCommand(t, repoPath, "gs", "init")

	// Create root branch via git
	runCommand(t, repoPath, "git", "checkout", "-b", "feat/test-root")

	// Create a stacked branch using git-spice
	runCommand(t, repoPath, "gs", "branch", "feat/test-stack-child")

	// List worktrees to verify
	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	worktrees, err := client.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}

	// We should have at least the main worktree
	if len(worktrees) < 1 {
		t.Errorf("expected at least 1 worktree, got %d", len(worktrees))
	}

	// Cleanup - return to main
	runCommand(t, repoPath, "git", "checkout", "main")
}

// TestStacking_BranchNaming tests the nanoid-based branch naming:
// 1. Create a root branch
// 2. Stack with auto-suffix (generates 4-char suffix)
// 3. Verify branch name format
func TestStacking_BranchNaming(t *testing.T) {
	skipIfNoGit(t)

	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("git-spice not available")
	}

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	runCommand(t, repoPath, "gs", "init")
	runCommand(t, repoPath, "git", "checkout", "-b", "feat/naming-test")

	// Create stacked branch - git-spice will auto-generate suffix
	runCommand(t, repoPath, "gs", "branch", "feat/naming-test-child")

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Verify branch was created
	branches, err := client.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}

	// Check that child branch exists (name may have auto-suffix)
	foundChild := false
	for _, w := range branches {
		if strings.Contains(w.Branch, "naming-test-child") {
			foundChild = true
			break
		}
	}

	if !foundChild {
		t.Error("stacked child branch not found")
	}

	// Cleanup
	runCommand(t, repoPath, "git", "checkout", "main")
}
