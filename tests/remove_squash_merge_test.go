package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
)

func TestRemove_SquashMergedBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tempDir := t.TempDir()
	runGit(t, tempDir, "init", "-b", "main")
	runGit(t, tempDir, "config", "user.name", "Test")
	runGit(t, tempDir, "config", "user.email", "test@test.com")

	writeFile(t, filepath.Join(tempDir, "file.txt"), "initial")
	runGit(t, tempDir, "add", ".")
	runGit(t, tempDir, "commit", "-m", "initial")

	runGit(t, tempDir, "checkout", "-b", "feature/test")
	writeFile(t, filepath.Join(tempDir, "file.txt"), "initial\nfeature")
	runGit(t, tempDir, "add", ".")
	runGit(t, tempDir, "commit", "-m", "add feature")

	runGit(t, tempDir, "checkout", "main")
	runGit(t, tempDir, "merge", "--squash", "feature/test")
	runGit(t, tempDir, "commit", "-m", "squash merge")

	worktreePath := filepath.Join(tempDir, "wt-feature")
	runGit(t, tempDir, "worktree", "add", worktreePath, "feature/test")

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer os.Chdir(originalWd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir tempDir: %v", err)
	}

	gitClient, err := git.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	cfg := config.DefaultConfig()
	svc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	err = svc.RemoveEnhanced(context.Background(), worktreePath, domain.ForceNone)
	if err != nil {
		t.Errorf("RemoveEnhanced failed: %v\nExpected success for squash-merged branch", err)
	}

	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree path still exists: %s", worktreePath)
	}
}

func runGit(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func writeFile(t testing.TB, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
