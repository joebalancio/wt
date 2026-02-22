package cli

import (
	"testing"
)

func TestNewStackCmd_TmuxIntegration(t *testing.T) {
	// Verify the stack command exists and has the expected structure
	cmd := NewStackCmd()
	if cmd == nil {
		t.Fatal("NewStackCmd() should return a command")
	}
	// The --no-tmux flag is inherited from root
	if cmd.Use != "stack [name]" {
		t.Errorf("Expected command use 'stack [name]', got %q", cmd.Use)
	}
}

func TestNewStackCmd_HasRunFlag(t *testing.T) {
	cmd := NewStackCmd()
	flag := cmd.Flags().Lookup("run")
	if flag == nil {
		t.Fatal("expected --run flag to be defined")
	}
	if flag.DefValue != "" {
		t.Errorf("expected --run default to be empty, got %q", flag.DefValue)
	}
}
