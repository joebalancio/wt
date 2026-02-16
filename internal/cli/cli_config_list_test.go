package cli

import (
	"testing"
)

func TestConfigListCmdFlags(t *testing.T) {
	cmd := NewConfigListCmd()

	if cmd.Flags().Lookup("local") == nil {
		t.Error("config list command missing --local flag")
	}
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config list command missing --global flag")
	}
}

func TestConfigListCmdConflictingFlags(t *testing.T) {
	cmd := NewConfigListCmd()
	cmd.SetArgs([]string{"--local", "--global"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when both --local and --global are specified")
	}
}
