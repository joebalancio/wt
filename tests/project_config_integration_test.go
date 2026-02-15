//go:build integration
// +build integration

package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

// TestIntegration_ProjectConfig_WorktreeSharing verifies that worktrees
// get the same project config (committed to repo)
func TestIntegration_ProjectConfig_WorktreeSharing(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create main repo with project config
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Add .wt.yaml to repo
	projectConfig := `
hooks:
  on_worktree_create:
    - run: "echo project-hook"
tmux:
  attach_on_create: false
`
	projectPath := filepath.Join(repoPath, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit the project config
	runGitCommand(t, repoPath, "add", ".wt.yaml")
	runGitCommand(t, repoPath, "commit", "-m", "Add project config")

	// Change to repo for config discovery
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(repoPath)

	// Verify project config is found
	foundProject, _, err := config.FindConfigs("")
	if err != nil {
		t.Fatalf("FindConfigs error: %v", err)
	}
	if foundProject == "" {
		t.Fatal("Expected project config to be found")
	}

	// Load and verify
	cfg, err := config.LoadMerged(foundProject, "")
	if err != nil {
		t.Fatalf("LoadMerged error: %v", err)
	}

	if len(cfg.Hooks.OnWorktreeCreate) != 1 {
		t.Errorf("Expected 1 hook, got %d", len(cfg.Hooks.OnWorktreeCreate))
	}
	if cfg.Hooks.OnWorktreeCreate[0].Run != "echo project-hook" {
		t.Errorf("Hook run = %q, want 'echo project-hook'", cfg.Hooks.OnWorktreeCreate[0].Run)
	}
}

// TestIntegration_ProjectConfig_GitRootDiscovery verifies that
// config is found when running from a subdirectory
func TestIntegration_ProjectConfig_GitRootDiscovery(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Add .wt.yaml at root
	projectConfig := `tmux:
  layout: test-layout
`
	projectPath := filepath.Join(repoPath, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subdirectory
	subDir := filepath.Join(repoPath, "src", "components")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Change to subdirectory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(subDir)

	// Verify project config is still found via git root
	foundProject, _, err := config.FindConfigs("")
	if err != nil {
		t.Fatalf("FindConfigs error: %v", err)
	}
	if foundProject == "" {
		t.Fatal("Expected project config to be found from subdirectory")
	}

	// Verify it's the config at repo root
	if foundProject != projectPath {
		t.Errorf("Found project path = %q, want %q", foundProject, projectPath)
	}
}
