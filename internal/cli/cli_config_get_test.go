package cli

import (
	"testing"
)

func TestConfigGetCmdFlags(t *testing.T) {
	cmd := NewConfigGetCmd()

	if cmd.Flags().Lookup("local") == nil {
		t.Error("config get command missing --local flag")
	}
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config get command missing --global flag")
	}
}

func TestConfigGetCmdConflictingFlags(t *testing.T) {
	cmd := NewConfigGetCmd()
	cmd.SetArgs([]string{"worktree.location", "--local", "--global"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when both --local and --global are specified")
	}
}
