package cli

import (
	"testing"
)

func TestConfigUnsetCmdGlobalFlag(t *testing.T) {
	cmd := NewConfigUnsetCmd()
	if cmd.Flags().Lookup("global") == nil {
		t.Error("config unset command missing --global flag")
	}
}
