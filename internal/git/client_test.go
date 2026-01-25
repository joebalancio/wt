package git

import (
	"context"
	"testing"
)

func TestGitClient_Interface(t *testing.T) {
	t.Run("Client satisfies GitClient interface", func(t *testing.T) {
		client, err := NewClient()
		if err != nil {
			t.Skipf("git not available: %v", err)
		}

		// This will fail if Client doesn't implement all interface methods
		var _ GitClient = client
	})
}

func TestGitClient_ListWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	client, err := NewClient()
	if err != nil {
		t.Skipf("git not available: %v", err)
	}

	worktrees, err := client.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}

	if len(worktrees) == 0 {
		t.Error("expected at least one worktree (the main one)")
	}
}

func TestGitClient_AddWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// This requires a valid git repo - skip for now
	t.Skip("requires test repo setup")
}
