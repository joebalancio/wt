package cli

import (
	"testing"
)

func TestNewRemoveCmd(t *testing.T) {
	t.Run("creates remove command", func(t *testing.T) {
		cmd := NewRemoveCmd()
		if cmd == nil {
			t.Fatal("NewRemoveCmd() returned nil")
		}
		if cmd.Use != "remove <path>" {
			t.Errorf("got Use %q, want 'remove <path>'", cmd.Use)
		}

		// Check required args - Args should be cobra.ExactArgs(1)
		if cmd.Args == nil {
			t.Error("remove command should require path argument")
		}
	})

	t.Run("has expected flags", func(t *testing.T) {
		cmd := NewRemoveCmd()

		flag := cmd.Flag("force")
		if flag == nil {
			t.Error("missing --force flag")
		}
	})
}
