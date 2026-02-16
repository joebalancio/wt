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
