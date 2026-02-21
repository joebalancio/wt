package cli

import (
	"testing"
)

func TestNewRemoveCmd_TmuxIntegration(t *testing.T) {
	// Verify the remove command exists and has the expected structure
	cmd := NewRemoveCmd()
	if cmd == nil {
		t.Fatal("NewRemoveCmd() should return a command")
	}
	if cmd.Use != "remove [path]" {
		t.Errorf("Expected command use 'remove [path]', got %q", cmd.Use)
	}
}

func TestNewRemoveCmd_AllowsOptionalPath(t *testing.T) {
	cmd := NewRemoveCmd()

	err := cmd.ValidateArgs([]string{})
	if err != nil {
		t.Errorf("remove command should accept 0 arguments for interactive mode, got error: %v", err)
	}

	err = cmd.ValidateArgs([]string{"/path/to/worktree"})
	if err != nil {
		t.Errorf("remove command should accept 1 argument, got error: %v", err)
	}
}
