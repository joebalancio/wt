package cli

import (
	"testing"
)

func TestNewAddCmd(t *testing.T) {
	t.Run("creates add command", func(t *testing.T) {
		cmd := NewAddCmd()
		if cmd == nil {
			t.Fatal("NewAddCmd() returned nil")
		}
		if cmd.Use != "add <branch>" {
			t.Errorf("got Use %q, want 'add <branch>'", cmd.Use)
		}

		// Check required args - Args should be cobra.ExactArgs(1)
		if cmd.Args == nil {
			t.Error("add command should require branch argument")
		}
	})

	t.Run("has expected flags", func(t *testing.T) {
		cmd := NewAddCmd()

		flag := cmd.Flag("base")
		if flag == nil {
			t.Error("missing --base flag")
		}

		flag = cmd.Flag("path")
		if flag == nil {
			t.Error("missing --path flag")
		}

		flag = cmd.Flag("force")
		if flag == nil {
			t.Error("missing --force flag")
		}
	})
}
