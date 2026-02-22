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

func TestParseCherryOutput(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		allMerged     bool
		unmergedCount int
	}{
		{
			name: "all commits merged (all minus signs)",
			input: `- abc123 Commit one
- def456 Commit two
- ghi789 Commit three`,
			allMerged:     true,
			unmergedCount: 0,
		},
		{
			name: "some commits not merged (plus signs)",
			input: `- abc123 Merged commit
+ def456 Unmerged commit
- ghi789 Another merged`,
			allMerged:     false,
			unmergedCount: 1,
		},
		{
			name:          "no commits (empty output)",
			input:         ``,
			allMerged:     true,
			unmergedCount: 0,
		},
		{
			name: "all unmerged (all plus signs)",
			input: `+ abc123 First commit
+ def456 Second commit`,
			allMerged:     false,
			unmergedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allMerged, unmergedCount := parseCherryOutput(tt.input)
			if allMerged != tt.allMerged {
				t.Errorf("allMerged = %v, want %v", allMerged, tt.allMerged)
			}
			if unmergedCount != tt.unmergedCount {
				t.Errorf("unmergedCount = %d, want %d", unmergedCount, tt.unmergedCount)
			}
		})
	}
}

func TestClient_IsBranchCherryMerged_Integration(t *testing.T) {
	skipIfNoGit(t)

	tempDir := t.TempDir()
	runGitCommand(t, tempDir, "init", "-b", "main")
	runGitCommand(t, tempDir, "config", "user.name", "Test User")
	runGitCommand(t, tempDir, "config", "user.email", "test@example.com")

	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Initial commit")

	runGitCommand(t, tempDir, "checkout", "-b", "feature/test")
	if err := os.WriteFile(testFile, []byte("# Test\n\nFeature content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Add feature")

	runGitCommand(t, tempDir, "checkout", "main")
	runGitCommand(t, tempDir, "cherry-pick", "feature/test")

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

	merged, err := client.IsBranchCherryMerged(context.Background(), "feature/test")
	if err != nil {
		t.Fatalf("IsBranchCherryMerged() error = %v", err)
	}

	if !merged {
		t.Error("IsBranchCherryMerged() = false, want true (cherry-picked commit should be detected)")
	}
}

func TestClient_IsBranchMerged_TwoTierDetection(t *testing.T) {
	skipIfNoGit(t)

	tempDir := t.TempDir()
	runGitCommand(t, tempDir, "init", "-b", "main")
	runGitCommand(t, tempDir, "config", "user.name", "Test User")
	runGitCommand(t, tempDir, "config", "user.email", "test@example.com")

	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Initial commit")

	runGitCommand(t, tempDir, "checkout", "-b", "feature/test")
	if err := os.WriteFile(testFile, []byte("# Test\n\nFeature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Add feature")

	runGitCommand(t, tempDir, "checkout", "main")
	runGitCommand(t, tempDir, "merge", "--squash", "feature/test")
	runGitCommand(t, tempDir, "commit", "-m", "Squash merge feature/test")

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

	ghClient, _ := NewGhClient()

	merged, err := client.IsBranchMergedWithDetection(context.Background(), "feature/test", ghClient)
	if err != nil {
		t.Fatalf("IsBranchMergedWithDetection() error = %v", err)
	}

	if !merged {
		t.Error("IsBranchMergedWithDetection() = false, want true (squash-merged branch should be detected)")
	}
}
