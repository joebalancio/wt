package cli

import (
	"testing"
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
