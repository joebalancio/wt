package cli

import (
	"strings"
	"testing"
)

func TestStackCommand_NestingCheck(t *testing.T) {
	expectedErrorParts := []string{
		"cannot stack from inside another worktree",
		"Main repository:",
	}

	for _, part := range expectedErrorParts {
		if !strings.Contains("cannot stack from inside another worktree\n\nMain repository: /path", part) {
			t.Errorf("Expected error message to contain %q", part)
		}
	}
}
