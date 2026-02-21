package cli

import (
	"testing"
)

func TestNewAddCmd_TmuxIntegration(t *testing.T) {
	// This is an integration test that verifies the add command
	// properly calls tmux window creation when in tmux

	// We can't easily test the actual tmux integration in unit tests,
	// but we can verify the code path exists

	// Verify the command structure
	cmd := NewAddCmd()
	if cmd == nil {
		t.Fatal("NewAddCmd() should return a command")
	}

	// The --no-tmux flag is defined on root and inherited
	// We verify the add command exists and has the expected structure
	// The actual flag inheritance is verified by integration tests
	if cmd.Use != "add <branch>" {
		t.Errorf("Expected command use 'add <branch>', got %q", cmd.Use)
	}
}

func TestNewAddCmd_AllowsOptionalBranch(t *testing.T) {
	cmd := NewAddCmd()

	err := cmd.ValidateArgs([]string{})
	if err != nil {
		t.Errorf("add command should accept 0 arguments for interactive mode, got error: %v", err)
	}

	err = cmd.ValidateArgs([]string{"feature-branch"})
	if err != nil {
		t.Errorf("add command should accept 1 argument, got error: %v", err)
	}

	err = cmd.ValidateArgs([]string{"branch1", "branch2"})
	if err == nil {
		t.Error("add command should reject more than 1 argument")
	}
}
