package cli

import "testing"

func TestStackCommand_HasPathFlag(t *testing.T) {
	cmd := NewStackCmd()
	flag := cmd.Flags().Lookup("path")
	if flag == nil {
		t.Error("stack command should have --path flag")
	}
}

func TestStackCommand_HasTrackFlag(t *testing.T) {
	cmd := NewStackCmd()
	flag := cmd.Flags().Lookup("track")
	if flag == nil {
		t.Error("stack command should have --track flag")
	}
}

func TestStackCommand_HasNoCheckoutFlag(t *testing.T) {
	cmd := NewStackCmd()
	flag := cmd.Flags().Lookup("no-checkout")
	if flag == nil {
		t.Error("stack command should have --no-checkout flag")
	}
}
