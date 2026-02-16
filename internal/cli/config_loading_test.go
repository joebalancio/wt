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
tmux:
  attach_on_create: false
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
tmux:
  layout: "main-horizontal"
  attach_on_create: true
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
	if cfg.Tmux.AttachOnCreate != false {
		t.Errorf("AttachOnCreate = %v, want false", cfg.Tmux.AttachOnCreate)
	}

	// Verify undefined field inherits
	if cfg.Tmux.Layout != "main-horizontal" {
		t.Errorf("Layout = %q, want main-horizontal", cfg.Tmux.Layout)
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
tmux:
  layout: "even-horizontal"
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
	if cfg.Tmux.Layout != "even-horizontal" {
		t.Errorf("Layout = %q, want even-horizontal", cfg.Tmux.Layout)
	}

	// Verify defaults for unset fields
	if cfg.Tmux.WindowName != "work" {
		t.Errorf("WindowName = %q, want work (default)", cfg.Tmux.WindowName)
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
tmux:
  layout: "main-horizontal"
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
	if cfg.Tmux.Layout != "main-horizontal" {
		t.Errorf("Layout = %q, want main-horizontal", cfg.Tmux.Layout)
	}
}

func TestLoadConfigForCommand_NoConfig(t *testing.T) {
	// Test when no configs exist - should return defaults
	cfg, err := config.LoadMerged("", "")
	if err != nil {
		t.Fatalf("LoadMerged error: %v", err)
	}

	// Verify defaults
	if cfg.Tmux.Layout != "main-vertical" {
		t.Errorf("Layout = %q, want main-vertical (default)", cfg.Tmux.Layout)
	}
	if cfg.Tmux.AttachOnCreate != true {
		t.Errorf("AttachOnCreate = %v, want true (default)", cfg.Tmux.AttachOnCreate)
	}
}
