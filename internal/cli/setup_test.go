package cli

import (
	"testing"
)

func TestNewSetupCmd(t *testing.T) {
	cmd := NewSetupCmd()
	if cmd == nil {
		t.Fatal("NewSetupCmd() returned nil")
	}
	if cmd.Use != "setup" {
		t.Errorf("Use = %v, want setup", cmd.Use)
	}
}
