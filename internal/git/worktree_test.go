package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestIsInWorktree_MainRepo(t *testing.T) {
	skipIfNoGit(t)

	// Create a test repository
	tempDir := t.TempDir()
	runGitCommand(t, tempDir, "init", "-b", "main")
	runGitCommand(t, tempDir, "config", "user.name", "Test User")
	runGitCommand(t, tempDir, "config", "user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Initial commit")

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	inWorktree, mainRepoRoot, err := client.IsInWorktree(context.Background())
	if err != nil {
		t.Fatalf("IsInWorktree() error = %v", err)
	}

	if inWorktree {
		t.Error("IsInWorktree() returned true for main repo, want false")
	}

	// mainRepoRoot should be our temp directory
	if mainRepoRoot != tempDir {
		t.Errorf("mainRepoRoot = %q, want %q", mainRepoRoot, tempDir)
	}
}

func TestIsInWorktree_InWorktree(t *testing.T) {
	skipIfNoGit(t)

	// Create a test repository with a worktree
	tempDir := t.TempDir()
	runGitCommand(t, tempDir, "init", "-b", "main")
	runGitCommand(t, tempDir, "config", "user.name", "Test User")
	runGitCommand(t, tempDir, "config", "user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Initial commit")

	// Create a worktree
	worktreePath := filepath.Join(tempDir, "feature-test")
	runGitCommand(t, tempDir, "worktree", "add", "-b", "feature/test", worktreePath)

	// Change to worktree directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)
	if err := os.Chdir(worktreePath); err != nil {
		t.Fatal(err)
	}

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	inWorktree, mainRepoRoot, err := client.IsInWorktree(context.Background())
	if err != nil {
		t.Fatalf("IsInWorktree() error = %v", err)
	}

	if !inWorktree {
		t.Error("IsInWorktree() returned false for worktree, want true")
	}

	// mainRepoRoot should point to the main repo, not the worktree
	if mainRepoRoot != tempDir {
		t.Errorf("mainRepoRoot = %q, want %q", mainRepoRoot, tempDir)
	}
}

// Helper function for tests
func runGitCommand(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, output)
	}
}

// Helper function for tests
func skipIfNoGit(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
}
