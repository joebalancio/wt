package cli

import (
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

// TestValidateGitSpiceConfig_NotConfigured tests validation when git-spice is not configured
func TestValidateGitSpiceConfig_NotConfigured(t *testing.T) {
	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: "", // Not configured
		},
	}

	err := validateGitSpiceConfig(cfg)
	if err == nil {
		t.Fatal("Expected error when git-spice not configured, got nil")
	}

	// Verify error message contains helpful instructions
	errMsg := err.Error()
	if !strings.Contains(errMsg, "git-spice not configured") {
		t.Errorf("Error message should mention 'git-spice not configured', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "wt init") {
		t.Errorf("Error message should mention 'wt init', got: %s", errMsg)
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
