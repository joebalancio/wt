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

func TestStackCommand_HasPathFlag(t *testing.T) {
	cmd := NewStackCmd()
	pathFlag := cmd.Flags().Lookup("path")
	if pathFlag == nil {
		t.Error("stack command should have --path flag")
	}
}

func TestStackCommand_HasTrackFlag(t *testing.T) {
	cmd := NewStackCmd()
	trackFlag := cmd.Flags().Lookup("track")
	if trackFlag == nil {
		t.Error("stack command should have --track flag")
	}
}

func TestStackCommand_HasNoCheckoutFlag(t *testing.T) {
	cmd := NewStackCmd()
	noCheckoutFlag := cmd.Flags().Lookup("no-checkout")
	if noCheckoutFlag == nil {
		t.Error("stack command should have --no-checkout flag")
	}
}

func TestStackCommand_FlagDefaults(t *testing.T) {
	cmd := NewStackCmd()

	path, _ := cmd.Flags().GetString("path")
	if path != "" {
		t.Errorf("path default = %v, want empty", path)
	}

	track, _ := cmd.Flags().GetString("track")
	if track != "" {
		t.Errorf("track default = %v, want empty", track)
	}

	noCheckout, _ := cmd.Flags().GetBool("no-checkout")
	if noCheckout {
		t.Errorf("no-checkout default = %v, want false", noCheckout)
	}
}

func TestStackCommand_NestingCheck(t *testing.T) {
	cmd := NewStackCmd()
	if cmd == nil {
		t.Fatal("NewStackCmd returned nil")
	}
	if cmd.Run == nil {
		t.Error("stack command should have Run function")
	}
}
