package cli

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestNewStackCmd(t *testing.T) {
	cmd := NewStackCmd()
	if cmd == nil {
		t.Fatal("NewStackCmd() returned nil")
	}
	if cmd.Use != "stack [name]" {
		t.Errorf("Use = %v, want 'stack [name]'", cmd.Use)
	}
}

func TestNewStackListCmd(t *testing.T) {
	cmd := NewStackListCmd()
	if cmd == nil {
		t.Fatal("NewStackListCmd() returned nil")
	}
	if cmd.Use != "list" {
		t.Errorf("Use = %v, want 'list'", cmd.Use)
	}
}

// TestValidateGitSpiceConfig_NoBinaryNoPath tests validation when git-spice is not configured and not in PATH
func TestValidateGitSpiceConfig_NoBinaryNoPath(t *testing.T) {
	// Save original PATH and restore after test
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty to simulate no git-spice available
	os.Setenv("PATH", "")

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: "", // Not configured
		},
	}

	err := validateGitSpiceConfig(cfg)
	if err == nil {
		t.Fatal("Expected error when git-spice not configured and not in PATH, got nil")
	}

	// Verify error message contains helpful instructions
	errMsg := err.Error()
	if !strings.Contains(errMsg, "git-spice not found") {
		t.Errorf("Error message should mention 'git-spice not found', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "Install git-spice") {
		t.Errorf("Error message should contain install instructions, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "cargo install") && !strings.Contains(errMsg, "brew install") {
		t.Errorf("Error message should mention cargo or brew install, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "wt init") {
		t.Errorf("Error message should mention 'wt init', got: %s", errMsg)
	}
}

// TestValidateGitSpiceConfig_NoBinaryWithDetection tests auto-detection when git-spice is in PATH but not configured
func TestValidateGitSpiceConfig_NoBinaryWithDetection(t *testing.T) {
	// Check if git-spice is actually available in PATH
	if _, err := exec.LookPath("git-spice"); err != nil {
		if _, err := exec.LookPath("gs"); err != nil {
			t.Skip("Skipping test: git-spice not available in PATH")
		}
	}

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: "", // Not configured
		},
	}

	// Should not error - should auto-detect and populate cfg.Spice.BinaryPath
	err := validateGitSpiceConfig(cfg)
	if err != nil {
		t.Fatalf("Expected no error when git-spice is in PATH, got: %v", err)
	}

	// Verify that BinaryPath was auto-populated
	if cfg.Spice.BinaryPath == "" {
		t.Error("Expected BinaryPath to be auto-populated, but it's still empty")
	}

	// Verify the detected path is actually executable
	if _, err := exec.LookPath(cfg.Spice.BinaryPath); err != nil {
		t.Errorf("Auto-detected path is not executable: %s, error: %v", cfg.Spice.BinaryPath, err)
	}
}

// TestValidateGitSpiceConfig_AlreadyConfigured tests that validation passes when git-spice is already configured
func TestValidateGitSpiceConfig_AlreadyConfigured(t *testing.T) {
	// This test assumes git-spice is available in PATH
	gitSpicePath, err := exec.LookPath("git-spice")
	if err != nil {
		// Try gs
		gitSpicePath, err = exec.LookPath("gs")
		if err != nil {
			t.Skip("Skipping test: git-spice not available in PATH")
		}
	}

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: gitSpicePath, // Already configured
		},
	}

	// Should not error and should not modify BinaryPath
	err = validateGitSpiceConfig(cfg)
	if err != nil {
		t.Fatalf("Expected no error when git-spice is already configured, got: %v", err)
	}

	// Verify BinaryPath wasn't changed
	if cfg.Spice.BinaryPath != gitSpicePath {
		t.Errorf("Expected BinaryPath to remain %s, got: %s", gitSpicePath, cfg.Spice.BinaryPath)
	}
}
