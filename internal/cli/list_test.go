package cli

import (
	"bytes"
	"testing"

	"github.com/user/wt/pkg/domain"
)

func TestNewListCmd(t *testing.T) {
	t.Run("creates list command", func(t *testing.T) {
		cmd := NewListCmd()
		if cmd == nil {
			t.Fatal("NewListCmd() returned nil")
		}
		if cmd.Use != "list" {
			t.Errorf("got Use %q, want 'list'", cmd.Use)
		}
	})
}

func TestListWorktrees(t *testing.T) {
	t.Run("formats worktree output", func(t *testing.T) {
		worktrees := []*domain.Worktree{
			{Path: "/main", Branch: "main", Head: "abc123"},
			{Path: "/feature", Branch: "feature", Head: "def456"},
		}

		var buf bytes.Buffer
		err := printWorktrees(&buf, worktrees)
		if err != nil {
			t.Fatalf("printWorktrees() error = %v", err)
		}

		output := buf.String()
		if !contains(output, "main") {
			t.Error("output should contain 'main' branch")
		}
		if !contains(output, "feature") {
			t.Error("output should contain 'feature' branch")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
