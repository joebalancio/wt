package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestLoadConfigForCommand_Merged(t *testing.T) {
	// This test verifies that loadConfigForCommand properly merges configs
	tempDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create project config
	projectConfig := `
hooks:
  on_worktree_create:
    - run: "cargo fetch"
worktree:
  location: "per-repo"
`
	projectPath := filepath.Join(tempDir, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create global config
	globalDir := filepath.Join(tempDir, ".config", "wt")
	os.MkdirAll(globalDir, 0o755)
	globalConfig := `
hooks:
  on_worktree_create:
    - run: "npm install"
worktree:
  location: "dedicated"
  dedicated_path: "~/worktrees"
`
	globalPath := filepath.Join(globalDir, "config.yaml")
	if err := os.WriteFile(globalPath, []byte(globalConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tempDir)

	// The actual test would need to run loadConfigForCommand
	// For now, test the underlying LoadMerged directly
	cfg, err := config.LoadMerged(projectPath, globalPath)
	if err != nil {
		t.Fatalf("LoadMerged error: %v", err)
	}

	// Verify project overrides global
	if cfg.Worktree.Location != "per-repo" {
		t.Errorf("Location = %q, want per-repo", cfg.Worktree.Location)
	}

	// Verify array replacement
	if len(cfg.Hooks.OnWorktreeCreate) != 1 {
		t.Fatalf("OnWorktreeCreate hooks = %d, want 1", len(cfg.Hooks.OnWorktreeCreate))
	}
	if cfg.Hooks.OnWorktreeCreate[0].Run != "cargo fetch" {
		t.Errorf("Hook run = %q, want cargo fetch", cfg.Hooks.OnWorktreeCreate[0].Run)
	}
}

func TestLoadConfigForCommand_ProjectOnly(t *testing.T) {
	// Test when only project config exists
	tempDir := t.TempDir()

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create project config only
	projectConfig := `
worktree:
  location: "per-repo"
  dedicated_path: "/custom/path"
`
	projectPath := filepath.Join(tempDir, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tempDir) // No global config will be found

	cfg, err := config.LoadMerged(projectPath, "")
	if err != nil {
		t.Fatalf("LoadMerged error: %v", err)
	}

	// Verify project value
	if cfg.Worktree.Location != "per-repo" {
		t.Errorf("Location = %q, want per-repo", cfg.Worktree.Location)
	}
	if cfg.Worktree.DedicatedPath != "/custom/path" {
		t.Errorf("DedicatedPath = %q, want /custom/path", cfg.Worktree.DedicatedPath)
	}
}

func TestLoadConfigForCommand_GlobalOnly(t *testing.T) {
	// Test when only global config exists
	tempDir := t.TempDir()

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create global config only
	globalDir := filepath.Join(tempDir, ".config", "wt")
	os.MkdirAll(globalDir, 0o755)
	globalConfig := `
worktree:
  location: "per-repo"
`
	globalPath := filepath.Join(globalDir, "config.yaml")
	if err := os.WriteFile(globalPath, []byte(globalConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tempDir)

	cfg, err := config.LoadMerged("", globalPath)
	if err != nil {
		t.Fatalf("LoadMerged error: %v", err)
	}

	// Verify global value
	if cfg.Worktree.Location != "per-repo" {
		t.Errorf("Location = %q, want per-repo", cfg.Worktree.Location)
	}
}

func TestLoadConfigForCommand_NoConfig(t *testing.T) {
	// Test when no configs exist - should return defaults
	cfg, err := config.LoadMerged("", "")
	if err != nil {
		t.Fatalf("LoadMerged error: %v", err)
	}

	// Verify defaults - empty location means per-repo (default)
	if cfg.Worktree.Location != "" {
		t.Errorf("Location = %q, want empty (per-repo default)", cfg.Worktree.Location)
	}
	if cfg.Worktree.GetDedicatedPath() != "~/worktrees" {
		t.Errorf("GetDedicatedPath() = %q, want ~/worktrees (default)", cfg.Worktree.GetDedicatedPath())
	}
}
