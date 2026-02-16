package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestConfigGetCmdFlags(t *testing.T) {
	cmd := NewConfigGetCmd()

	if cmd.Flags().Lookup("local") == nil {
		t.Error("config get command missing --local flag")
	}
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config get command missing --global flag")
	}
}

func TestConfigGetCmdConflictingFlags(t *testing.T) {
	cmd := NewConfigGetCmd()
	cmd.SetArgs([]string{"worktree.location", "--local", "--global"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when both --local and --global are specified")
	}
}

func TestConfigGetMergedOutsideGitWarning(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Change to a temp directory without .git
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	cmd := NewConfigGetCmd()
	cmd.SetArgs([]string{"worktree.location"})

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Redirect os.Stderr to capture warnings from Warn() function
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// This should succeed but show warning
	err := cmd.Execute()

	// Close and restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured stderr
	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(r)
	capturedStderr := stderrBuf.String()

	if err != nil {
		t.Errorf("merged read outside git should succeed, got error: %v", err)
	}

	// Check warning was printed to stderr
	if !strings.Contains(capturedStderr, "Warning:") || !strings.Contains(capturedStderr, "not in a git repository") {
		t.Errorf("expected warning in stderr, got: %s", capturedStderr)
	}
}
