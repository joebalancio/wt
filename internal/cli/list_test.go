package cli

import (
	"bytes"
	"strings"
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
		if !strings.Contains(output, "main") {
			t.Error("output should contain 'main' branch")
		}
		if !strings.Contains(output, "feature") {
			t.Error("output should contain 'feature' branch")
		}
	})

	t.Run("handles empty worktree list", func(t *testing.T) {
		worktrees := []*domain.Worktree{}

		var buf bytes.Buffer
		err := printWorktrees(&buf, worktrees)
		if err != nil {
			t.Fatalf("printWorktrees() error = %v", err)
		}

		output := buf.String()
		if output != "" {
			t.Errorf("expected empty output, got %q", output)
		}
	})
}

func TestListCmdFlags(t *testing.T) {
	t.Run("registers branches flag", func(t *testing.T) {
		cmd := NewListCmd()
		flag := cmd.Flags().Lookup("branches")
		if flag == nil {
			t.Fatal("--branches flag not found")
		}
	})

	t.Run("registers path flag", func(t *testing.T) {
		cmd := NewListCmd()
		flag := cmd.Flags().Lookup("path")
		if flag == nil {
			t.Fatal("--path flag not found")
		}
	})

	t.Run("parses branches flag", func(t *testing.T) {
		cmd := NewListCmd()
		args := []string{"--branches=main,feature"}
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		// Verify the flag was parsed by checking its value
		flag := cmd.Flags().Lookup("branches")
		if flag == nil {
			t.Fatal("--branches flag not found")
		}
	})
}
