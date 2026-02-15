package cli

import (
	"testing"
)

func TestRemoveCmd_Enhanced(t *testing.T) {
	t.Run("accepts optional path argument", func(t *testing.T) {
		cmd := NewRemoveCmd()
		if cmd.Args == nil {
			t.Error("expected Args to be set")
		}

		if err := cmd.Args(cmd, []string{}); err != nil {
			t.Errorf("expected no error for 0 args, got: %v", err)
		}
		if err := cmd.Args(cmd, []string{"/path/to/worktree"}); err != nil {
			t.Errorf("expected no error for 1 arg, got: %v", err)
		}
		if err := cmd.Args(cmd, []string{"/path/one", "/path/two"}); err == nil {
			t.Error("expected error for 2 args")
		}
	})

	t.Run("parses --force flag variants", func(t *testing.T) {
		tests := []struct {
			name string
			args []string
		}{
			{name: "no flag", args: []string{}},
			{name: "--force", args: []string{"--force"}},
			{name: "--force=local", args: []string{"--force=local"}},
			{name: "--force=remote", args: []string{"--force=remote"}},
			{name: "--force=all", args: []string{"--force=all"}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cmd := NewRemoveCmd()
				if err := cmd.ParseFlags(tt.args); err != nil {
					t.Fatalf("ParseFlags() error = %v", err)
				}
			})
		}
	})
}
