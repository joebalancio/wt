package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConfigUnsetCmdGlobalFlag(t *testing.T) {
	cmd := NewConfigUnsetCmd()
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config unset command missing --global flag")
	}
}

func TestConfigUnsetDefaultBehavior(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Create a temp git repo
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	// Initialize a proper git repository
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte{}, 0o644); err != nil {
		t.Fatalf("creating .gitignore: %v", err)
	}
	// Use exec.Command to initialize git repo
	if _, err := exec.LookPath("git"); err == nil {
		// git is available, initialize a real repo
		execCmd := exec.Command("git", "init")
		execCmd.Dir = tmpDir
		if err := execCmd.Run(); err != nil {
			t.Fatalf("initializing git repo: %v", err)
		}
	} else {
		t.Skip("git not available, skipping test")
	}

	// Create command without --global flag
	cmd := NewConfigUnsetCmd()
	cmd.SetArgs([]string{"worktree.location"})

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Run command (this will create .wt.yaml)
	cmd.Execute()

	// Verify .wt.yaml was created in project dir
	if _, err := os.Stat(filepath.Join(tmpDir, ".wt.yaml")); os.IsNotExist(err) {
		t.Error("expected .wt.yaml to be created in project directory")
	}
}

func TestConfigUnsetGlobalFlagOutsideGit(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Change to a temp directory without .git
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	// Get global config path
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".config", "wt", "config.yaml")

	// Backup existing global config if it exists
	var backup []byte
	if data, err := os.ReadFile(globalPath); err == nil {
		backup = data
		defer func() {
			if backup != nil {
				os.WriteFile(globalPath, backup, 0o644)
			}
		}()
	}

	// Create command with --global flag
	cmd := NewConfigUnsetCmd()
	cmd.SetArgs([]string{"--global", "worktree.location"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Run command - should succeed outside git repo
	err := cmd.Execute()
	if err != nil {
		t.Errorf("config unset --global should work outside git repo, got error: %v", err)
	}
}
