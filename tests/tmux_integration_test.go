package tests

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestTmuxWindowCreation_AddCommand(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	if os.Getenv("TMUX") == "" {
		t.Skip("integration test must be run inside tmux")
	}

	// This test requires a real git repository
	// It verifies that wt add creates a tmux window

	// Create a test branch using wt
	cmd := exec.Command("go", "run", ".", "add", "test-tmux-integration")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wt add failed: %v\n%s", err, output)
	}

	// Check if tmux window was created
	listCmd := exec.Command("tmux", "list-windows", "-F", "#{window_name}")
	windowOutput, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tmux list-windows failed: %v", err)
	}

	if !strings.Contains(string(windowOutput), "test-tmux") {
		t.Error("tmux window was not created by wt add")
	}

	// Cleanup
	_ = exec.Command("go", "run", ".", "remove", "test-tmux-integration").Run()
	killCmd := exec.Command("tmux", "kill-window", "-t", "test-tmux")
	_ = killCmd.Run()
}

func TestTmuxWindowCreation_StackCommand(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	if os.Getenv("TMUX") == "" {
		t.Skip("integration test must be run inside tmux")
	}

	// Requires git-spice to be configured
	// This test verifies stack window naming

	// Create a stack
	cmd := exec.Command("go", "run", ".", "stack", "test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("git-spice not configured or stack failed: %v\n%s", err, output)
	}

	// Check for numbered window
	listCmd := exec.Command("tmux", "list-windows", "-F", "#{window_name}")
	windowOutput, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tmux list-windows failed: %v", err)
	}

	// Should have a window with /1 suffix
	if !strings.Contains(string(windowOutput), "/1") {
		t.Logf("Windows: %s", windowOutput)
		t.Error("stack window should have /1 suffix")
	}

	// Cleanup
	// (Would need to delete stack branches)
}

func TestTmuxNoTmuxFlag(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	if os.Getenv("TMUX") == "" {
		t.Skip("integration test must be run inside tmux")
	}

	// Test that --no-tmux flag prevents window creation

	// List windows before
	beforeCmd := exec.Command("tmux", "list-windows", "-F", "#{window_name}")
	beforeOutput, _ := beforeCmd.CombinedOutput()

	// Create branch with --no-tmux
	cmd := exec.Command("go", "run", ".", "add", "test-no-tmux", "--no-tmux")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wt add failed: %v\n%s", err, output)
	}

	// List windows after
	afterCmd := exec.Command("tmux", "list-windows", "-F", "#{window_name}")
	afterOutput, _ := afterCmd.CombinedOutput()

	// Should be the same windows
	if string(beforeOutput) != string(afterOutput) {
		t.Error("--no-tmux flag should not create windows")
	}

	// Cleanup
	_ = exec.Command("go", "run", ".", "remove", "test-no-tmux").Run()
}
