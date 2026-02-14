package cli

import (
	"testing"
)

func TestNewDoneCmd_BasicStructure(t *testing.T) {
	// Verify the done command exists and has the expected structure
	cmd := NewDoneCmd()
	if cmd == nil {
		t.Fatal("NewDoneCmd() should return a command")
	}
	if cmd.Use != "done <path> <branch>" {
		t.Errorf("Expected command use 'done <path> <branch>', got %q", cmd.Use)
	}

	// Verify --force flag exists
	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Error("Expected --force flag to be defined")
	}

	// Note: --dry-run is a persistent flag on root command, inherited by subcommands
	// It cannot be tested directly on this detached command
}
