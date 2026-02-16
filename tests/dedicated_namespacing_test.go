package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
)

func TestIntegration_DedicatedMode_Namespacing(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	// Create two repos with same branch name
	repoA, _ := setupTestRepo(t)
	repoB, _ := setupTestRepo(t)
	defer os.RemoveAll(repoA)
	defer os.RemoveAll(repoB)

	// Create a shared worktrees directory
	worktreesDir := t.TempDir()

	// Create worktree from repo A
	cfgA := config.DefaultConfig()
	cfgA.Worktree.Location = "dedicated"
	cfgA.Worktree.DedicatedPath = worktreesDir

	gitClientA, _ := git.NewClient()
	svcA, _ := worktree.NewService(gitClientA, cfgA)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(repoA)

	pathA, err := svcA.ResolvePath(context.Background(), "feature/test", "")
	if err != nil {
		t.Fatalf("ResolvePath from repo A failed: %v", err)
	}

	// Create worktree from repo B
	cfgB := config.DefaultConfig()
	cfgB.Worktree.Location = "dedicated"
	cfgB.Worktree.DedicatedPath = worktreesDir

	gitClientB, _ := git.NewClient()
	svcB, _ := worktree.NewService(gitClientB, cfgB)

	os.Chdir(repoB)

	pathB, err := svcB.ResolvePath(context.Background(), "feature/test", "")
	if err != nil {
		t.Fatalf("ResolvePath from repo B failed: %v", err)
	}

	// Paths should be different due to namespacing
	if pathA == pathB {
		t.Errorf("Paths should be different:\n  repo A: %s\n  repo B: %s", pathA, pathB)
	}

	// Paths should include repo names
	repoAName := filepath.Base(repoA)
	repoBName := filepath.Base(repoB)

	expectedA := filepath.Join(worktreesDir, repoAName, "feature/test")
	expectedB := filepath.Join(worktreesDir, repoBName, "feature/test")

	if pathA != expectedA {
		t.Errorf("Path A = %q, want %q", pathA, expectedA)
	}
	if pathB != expectedB {
		t.Errorf("Path B = %q, want %q", pathB, expectedB)
	}
}

func TestIntegration_DefaultIsPerRepo(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	repoPath, _ := setupTestRepo(t)
	defer os.RemoveAll(repoPath)

	cfg := config.DefaultConfig()

	// Default should be per-repo
	if cfg.Worktree.IsDedicated() {
		t.Error("Default config should use per-repo mode, not dedicated")
	}

	gitClient, _ := git.NewClient()
	svc, _ := worktree.NewService(gitClient, cfg)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(repoPath)

	path, err := svc.ResolvePath(context.Background(), "feature/test", "")
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}

	// Path should be per-repo style
	expected := filepath.Join(repoPath, ".worktrees", "feature/test")
	if path != expected {
		t.Errorf("ResolvePath() = %q, want %q", path, expected)
	}
}
