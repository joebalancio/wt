package git

import (
	"testing"
)

func TestParseWorktreeOutput(t *testing.T) {
	input := `worktree /path/to/main
branch refs/heads/main
HEAD abc123
worktree /path/to/feature
branch refs/heads/feature
HEAD def456
`

	worktrees, err := parseWorktreeOutput(input)
	if err != nil {
		t.Fatalf("parseWorktreeOutput() error = %v", err)
	}

	if len(worktrees) != 2 {
		t.Errorf("got %d worktrees, want 2", len(worktrees))
	}

	if worktrees[0].Branch != "main" {
		t.Errorf("first branch = %v, want main", worktrees[0].Branch)
	}
}
