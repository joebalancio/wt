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

// TestIntegration_RemoveEnhanced_Basic tests the enhanced remove workflow.
func TestIntegration_RemoveEnhanced_Basic(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	featureBranch := "feature/remove-test"
	featurePath := filepath.Join(repoPath, "feature-remove")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main",
		Path:   featurePath,
	}

	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	featureFile := filepath.Join(featurePath, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("Feature content\n"), 0o644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}
	runGitCommand(t, featurePath, "add", "feature.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add feature")

	runGitCommand(t, repoPath, "checkout", "main")
	runGitCommand(t, repoPath, "merge", featureBranch)

	err = service.RemoveEnhanced(ctx, featurePath, domain.ForceNone)
	if err != nil {
		t.Fatalf("RemoveEnhanced() error = %v", err)
	}

	worktrees, err := service.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree after remove, got %d", len(worktrees))
	}

	exists, err := client.BranchExists(ctx, featureBranch)
	if err != nil {
		t.Fatalf("failed to check branch existence: %v", err)
	}
	if exists {
		t.Error("feature branch still exists after RemoveEnhanced")
	}
}

// TestIntegration_RemoveEnhanced_UnmergedFails tests that unmerged branches fail without force.
func TestIntegration_RemoveEnhanced_UnmergedFails(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	featureBranch := "feature/unmerged"
	featurePath := filepath.Join(repoPath, "feature-unmerged")

	spec := domain.WorktreeCreateSpec{Branch: featureBranch, Base: "main", Path: featurePath}
	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	featureFile := filepath.Join(featurePath, "feature.txt")
	if err := os.WriteFile(featureFile, []byte("Unmerged content\n"), 0o644); err != nil {
		t.Fatalf("failed to create feature file: %v", err)
	}
	runGitCommand(t, featurePath, "add", "feature.txt")
	runGitCommand(t, featurePath, "commit", "-m", "Add unmerged feature")

	runGitCommand(t, repoPath, "checkout", "main")

	err = service.RemoveEnhanced(ctx, featurePath, domain.ForceNone)
	if err == nil {
		t.Fatal("RemoveEnhanced() expected error for unmerged branch, got nil")
	}

	err = service.RemoveEnhanced(ctx, featurePath, domain.ForceLocal)
	if err != nil {
		t.Fatalf("RemoveEnhanced() with force error = %v", err)
	}
}

// TestIntegration_RemoveEnhanced_DirtyFails tests that dirty worktrees fail without force.
func TestIntegration_RemoveEnhanced_DirtyFails(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	featureBranch := "feature/dirty"
	featurePath := filepath.Join(repoPath, "feature-dirty")

	spec := domain.WorktreeCreateSpec{Branch: featureBranch, Base: "main", Path: featurePath}
	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	featureFile := filepath.Join(featurePath, "uncommitted.txt")
	if err := os.WriteFile(featureFile, []byte("Uncommitted content\n"), 0o644); err != nil {
		t.Fatalf("failed to create uncommitted file: %v", err)
	}

	runGitCommand(t, repoPath, "checkout", "main")

	err = service.RemoveEnhanced(ctx, featurePath, domain.ForceNone)
	if err == nil {
		t.Fatal("RemoveEnhanced() expected error for dirty worktree, got nil")
	}

	err = service.RemoveEnhanced(ctx, featurePath, domain.ForceLocal)
	if err != nil {
		t.Fatalf("RemoveEnhanced() with force error = %v", err)
	}
}

// TestIntegration_RemoveEnhanced_DefaultBranchFails tests that default branch cannot be removed.
func TestIntegration_RemoveEnhanced_DefaultBranchFails(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	err = service.RemoveEnhanced(ctx, repoPath, domain.ForceLocal)
	if err == nil {
		t.Fatal("RemoveEnhanced() expected error for default branch, got nil")
	}
}

// TestIntegration_RemoveEnhanced_CWDResolution tests removing from inside a worktree.
func TestIntegration_RemoveEnhanced_CWDResolution(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	featureBranch := "feature/cwd-test"
	featurePath := filepath.Join(repoPath, "feature-cwd")

	spec := domain.WorktreeCreateSpec{Branch: featureBranch, Base: "main", Path: featurePath}
	_, err = service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	runGitCommand(t, featurePath, "commit", "--allow-empty", "-m", "Empty commit")
	runGitCommand(t, repoPath, "checkout", "main")
	runGitCommand(t, repoPath, "merge", featureBranch)

	subDir := filepath.Join(featurePath, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("failed to change to subdir: %v", err)
	}

	cwd, _ := os.Getwd()
	resolved, err := service.ResolveFromCWD(ctx, cwd)
	if err != nil {
		t.Fatalf("ResolveFromCWD() error = %v", err)
	}
	if resolved.Branch != featureBranch {
		t.Errorf("resolved branch = %s, want %s", resolved.Branch, featureBranch)
	}

	err = service.RemoveEnhanced(ctx, resolved.Path, domain.ForceNone)
	if err != nil {
		t.Fatalf("RemoveEnhanced() error = %v", err)
	}
}
