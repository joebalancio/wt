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
	if cmd.Use != "remove <path>" {
		t.Errorf("Expected command use 'remove <path>', got %q", cmd.Use)
	}
}
