package cli

import (
	"testing"
)

func TestNewInitCmd(t *testing.T) {
	t.Run("creates init command", func(t *testing.T) {
		cmd := NewInitCmd()
		if cmd == nil {
			t.Fatal("NewInitCmd() returned nil")
		}
		if cmd.Use != "init" {
			t.Errorf("got Use %q, want 'init'", cmd.Use)
		}
	})

	t.Run("requires no arguments", func(t *testing.T) {
		cmd := NewInitCmd()
		if cmd.Args == nil {
			t.Error("init command should have Args validation")
		}
	})
}
