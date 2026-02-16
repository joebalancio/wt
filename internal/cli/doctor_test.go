package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDoctorCmd(t *testing.T) {
	cmd := NewDoctorCmd()
	if cmd == nil {
		t.Fatal("NewDoctorCmd() returned nil")
	}
	if cmd.Use != "doctor" {
		t.Errorf("Use = %v, want doctor", cmd.Use)
	}
}

func TestCheckConfiguration_DualConfig(t *testing.T) {
	// Test that doctor shows both project and global configs when available
	tempDir := t.TempDir()

	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create project config at Git root
	projectConfig := `
hooks:
  on_worktree_create:
    - run: "cargo fetch"
tmux:
  attach_on_create: false
`
	gitRoot := filepath.Join(tempDir, "myproject")
	os.MkdirAll(gitRoot, 0o755)

	// Initialize a real git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	projectPath := filepath.Join(gitRoot, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create global config
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

	// Change to git root directory to simulate being in a repo
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(gitRoot)

	// Run checkConfiguration
	var out bytes.Buffer
	result := checkConfiguration(&out)

	if !result {
		t.Error("checkConfiguration returned false, expected true")
	}

	output := out.String()

	// Verify both configs are shown
	if !strings.Contains(output, "✓ Project config:") {
		t.Error("Expected project config line in output")
	}
	if !strings.Contains(output, "✓ Global config:") {
		t.Error("Expected global config line in output")
	}
	if !strings.Contains(output, projectPath) {
		t.Errorf("Expected project path %q in output", projectPath)
	}
	if !strings.Contains(output, globalPath) {
		t.Errorf("Expected global path %q in output", globalPath)
	}
}

func TestCheckConfiguration_ProjectOnly(t *testing.T) {
	// Test when only project config exists
	tempDir := t.TempDir()

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create project config only
	gitRoot := filepath.Join(tempDir, "myproject")
	os.MkdirAll(gitRoot, 0o755)

	// Initialize a real git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	projectConfig := `
tmux:
  layout: "even-horizontal"
`
	projectPath := filepath.Join(gitRoot, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tempDir)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(gitRoot)

	var out bytes.Buffer
	result := checkConfiguration(&out)

	if !result {
		t.Error("checkConfiguration returned false, expected true")
	}

	output := out.String()

	// Verify only project config is shown
	if !strings.Contains(output, "✓ Project config:") {
		t.Error("Expected project config line in output")
	}
	if strings.Contains(output, "✓ Global config:") {
		t.Error("Did not expect global config line in output")
	}
}

func TestCheckConfiguration_GlobalOnly(t *testing.T) {
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

	// Create a git repo without project config
	gitRoot := filepath.Join(tempDir, "myproject")
	os.MkdirAll(gitRoot, 0o755)

	// Initialize a real git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	os.Setenv("HOME", tempDir)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(gitRoot)

	var out bytes.Buffer
	result := checkConfiguration(&out)

	if !result {
		t.Error("checkConfiguration returned false, expected true")
	}

	output := out.String()

	// Verify only global config is shown
	if strings.Contains(output, "✓ Project config:") {
		t.Error("Did not expect project config line in output")
	}
	if !strings.Contains(output, "✓ Global config:") {
		t.Error("Expected global config line in output")
	}
	if !strings.Contains(output, globalPath) {
		t.Errorf("Expected global path %q in output", globalPath)
	}
}

func TestCheckConfiguration_NoConfig(t *testing.T) {
	// Test when no configs exist
	tempDir := t.TempDir()

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create a git repo without any configs
	gitRoot := filepath.Join(tempDir, "myproject")
	os.MkdirAll(gitRoot, 0o755)

	// Initialize a real git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	os.Setenv("HOME", tempDir)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(gitRoot)

	var out bytes.Buffer
	result := checkConfiguration(&out)

	if result {
		t.Error("checkConfiguration returned true, expected false")
	}

	output := out.String()

	// Verify error message
	if !strings.Contains(output, "! No configuration file found") {
		t.Error("Expected 'No configuration file found' in output")
	}
}

func TestCheckConfiguration_InvalidYAML(t *testing.T) {
	// Test with invalid YAML in project config
	tempDir := t.TempDir()

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Create project config with invalid YAML
	gitRoot := filepath.Join(tempDir, "myproject")
	os.MkdirAll(gitRoot, 0o755)

	// Initialize a real git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRoot
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	projectConfig := `
tmux:
  layout: [invalid yaml
`
	projectPath := filepath.Join(gitRoot, ".wt.yaml")
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("HOME", tempDir)

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	os.Chdir(gitRoot)

	var out bytes.Buffer
	result := checkConfiguration(&out)

	if result {
		t.Error("checkConfiguration returned true, expected false for invalid YAML")
	}

	output := out.String()

	// Verify error message
	if !strings.Contains(output, "! Config is invalid") {
		t.Error("Expected 'Config is invalid' in output")
	}
}
