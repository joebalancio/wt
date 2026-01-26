package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/wt/internal/config"
	"github.com/user/wt/internal/git"
	"github.com/user/wt/internal/worktree"
	"github.com/user/wt/pkg/domain"
)

// setupTestRepo creates a temporary git repository with an initial commit
// Returns the repo path and a cleanup function
func setupTestRepo(t testing.TB) (string, func()) {
	t.Helper()

	// Create temp directory
	tempDir := t.TempDir()

	// Initialize git repo
	runGitCommand(t, tempDir, "init", "-b", "main")
	runGitCommand(t, tempDir, "config", "user.name", "Test User")
	runGitCommand(t, tempDir, "config", "user.email", "test@example.com")

	// Create initial commit
	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repository\n"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	runGitCommand(t, tempDir, "add", "README.md")
	runGitCommand(t, tempDir, "commit", "-m", "Initial commit")

	return tempDir, func() {
		// t.TempDir() handles cleanup automatically
	}
}

// runGitCommand executes a git command in the specified directory
func runGitCommand(t testing.TB, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, output)
	}
}

// skipIfNoGit skips the test if git is not available
func skipIfNoGit(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
}

// TestIntegration_WorktreeLifecycle tests the complete worktree lifecycle:
// 1. Create a worktree for a new branch
// 2. List worktrees and verify the new one appears
// 3. Remove the worktree
// 4. Verify it's gone from the list
func TestIntegration_WorktreeLifecycle(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory for git client operations
	// The git client uses exec.Command which respects the current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	// Create git client
	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Step 1: List initial worktrees (should only have main)
	initialWorktrees, err := client.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("failed to list initial worktrees: %v", err)
	}

	if len(initialWorktrees) != 1 {
		t.Errorf("expected 1 initial worktree, got %d", len(initialWorktrees))
	}

	// Step 2: Add a new worktree for a feature branch
	featureBranch := "feature/test-branch"
	featurePath := filepath.Join(repoPath, "feature-test")

	spec := domain.WorktreeCreateSpec{
		Branch: featureBranch,
		Base:   "main", // Specify base branch to create new branch from
		Path:   featurePath,
	}

	addedWorktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Verify the returned worktree has correct info
	if addedWorktree.Branch != featureBranch {
		t.Errorf("expected branch %s, got %s", featureBranch, addedWorktree.Branch)
	}

	// Step 3: List worktrees again and verify the new one appears
	worktreesAfterAdd, err := client.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("failed to list worktrees after add: %v", err)
	}

	if len(worktreesAfterAdd) != 2 {
		t.Errorf("expected 2 worktrees after add, got %d", len(worktreesAfterAdd))
	}

	// Verify the new worktree is in the list
	var found bool
	for _, w := range worktreesAfterAdd {
		if w.Branch == featureBranch {
			found = true
			break
		}
	}
	if !found {
		t.Error("new worktree not found in list")
	}

	// Step 4: Remove the worktree (use force since worktree has .git directory)
	if err := client.RemoveWorktree(ctx, addedWorktree.Path, true); err != nil {
		t.Fatalf("failed to remove worktree: %v", err)
	}

	// Step 5: List worktrees again and verify it's gone
	worktreesAfterRemove, err := client.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("failed to list worktrees after remove: %v", err)
	}

	if len(worktreesAfterRemove) != 1 {
		t.Errorf("expected 1 worktree after remove, got %d", len(worktreesAfterRemove))
	}

	// Verify the feature branch is no longer in the list
	for _, w := range worktreesAfterRemove {
		if w.Branch == featureBranch {
			t.Error("removed worktree still found in list")
		}
	}
}

// TestIntegration_WorktreeService tests the worktree service layer
func TestIntegration_WorktreeService(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	// Create service
	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Test listing worktrees
	worktrees, err := service.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}

	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	// Test adding a worktree
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/service-test",
		Path:   filepath.Join(repoPath, "service-test"),
	}

	addedWorktree, err := service.Add(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree via service: %v", err)
	}

	if addedWorktree.Branch != "feature/service-test" {
		t.Errorf("expected branch feature/service-test, got %s", addedWorktree.Branch)
	}

	// Test filtering by branch
	filter := &domain.WorktreeFilter{
		Branches: []string{"feature/service-test"},
	}

	filtered, err := service.List(ctx, filter)
	if err != nil {
		t.Fatalf("failed to list filtered worktrees: %v", err)
	}

	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered worktree, got %d", len(filtered))
	}

	// Test removing a worktree (use force since worktree has .git directory)
	if err := service.Remove(ctx, addedWorktree.Path, true); err != nil {
		t.Fatalf("failed to remove worktree via service: %v", err)
	}

	// Verify it's gone
	finalWorktrees, err := service.List(ctx, nil)
	if err != nil {
		t.Fatalf("failed to list worktrees after remove: %v", err)
	}

	if len(finalWorktrees) != 1 {
		t.Errorf("expected 1 worktree after remove, got %d", len(finalWorktrees))
	}
}

// TestIntegration_WorktreeWithBase tests creating a worktree from a specific base
func TestIntegration_WorktreeWithBase(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Create a worktree with a base branch
	// Note: Since we only have main branch, we create from main (default)
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/from-main",
		Base:   "main", // Explicitly specify base
		Path:   filepath.Join(repoPath, "feature-from-main"),
	}

	worktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree with base: %v", err)
	}

	if worktree.Branch != "feature/from-main" {
		t.Errorf("expected branch feature/from-main, got %s", worktree.Branch)
	}

	// Cleanup
	if err := client.RemoveWorktree(ctx, worktree.Path, true); err != nil {
		t.Fatalf("failed to cleanup worktree: %v", err)
	}
}

// TestIntegration_WorktreeWithPath tests creating a worktree with an explicit path
func TestIntegration_WorktreeWithPath(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Create a worktree with explicit path
	// After unification, the git client requires Path to be provided.
	// Auto-path generation is now handled by the worktree service layer.
	branchName := "feature/auto-path"
	expectedPath := filepath.Join(repoPath, branchName)

	spec := domain.WorktreeCreateSpec{
		Branch: branchName,
		Path:   expectedPath,
	}

	worktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree with path: %v", err)
	}

	// Verify the path is set correctly
	if worktree.Path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, worktree.Path)
	}

	// Verify the directory actually exists
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree path directory does not exist: %s", expectedPath)
	}

	// Cleanup
	if err := client.RemoveWorktree(ctx, worktree.Path, true); err != nil {
		t.Fatalf("failed to cleanup worktree: %v", err)
	}
}

// TestIntegration_RemoveNonExistentWorktree tests error handling when removing a non-existent worktree
func TestIntegration_RemoveNonExistentWorktree(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Try to remove a worktree that doesn't exist
	nonexistentPath := filepath.Join(repoPath, "non-existent-worktree")
	err = client.RemoveWorktree(ctx, nonexistentPath, false)

	// This should fail
	if err == nil {
		t.Error("expected error when removing non-existent worktree, got nil")
	}
}

// TestIntegration_AddDuplicateWorktree tests error handling when adding a duplicate worktree
func TestIntegration_AddDuplicateWorktree(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Add a worktree
	branchName := "feature/duplicate"
	spec := domain.WorktreeCreateSpec{
		Branch: branchName,
		Path:   filepath.Join(repoPath, "duplicate-test"),
	}

	_, err = client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Try to add the same branch again (should fail)
	_, err = client.AddWorktree(ctx, spec)
	if err == nil {
		t.Error("expected error when adding duplicate worktree, got nil")
	}

	// Cleanup
	worktrees, _ := client.ListWorktrees(ctx)
	for _, w := range worktrees {
		if w.Branch == branchName {
			client.RemoveWorktree(ctx, w.Path, true)
			break
		}
	}
}

// TestIntegration_GetRepoInfo tests getting repository information
func TestIntegration_GetRepoInfo(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	repoInfo, err := client.GetRepoInfo(ctx)
	if err != nil {
		t.Fatalf("failed to get repo info: %v", err)
	}

	// Verify repo info
	if !repoInfo.IsValid() {
		t.Error("expected valid repo info")
	}

	if repoInfo.RootPath == "" {
		t.Error("expected non-empty root path")
	}

	// The root path should match our temp dir (or be a parent)
	if !strings.HasPrefix(repoInfo.RootPath, repoPath) && !strings.HasPrefix(repoPath, repoInfo.RootPath) {
		t.Errorf("repo root path %s not related to test repo path %s", repoInfo.RootPath, repoPath)
	}

	// Default branch should be "main" (we haven't configured any remote)
	if repoInfo.DefaultBranch == "" {
		t.Error("expected non-empty default branch")
	}
}

// TestIntegration_BranchExists tests checking if a branch exists
func TestIntegration_BranchExists(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Check if main branch exists (should be true)
	exists, err := client.BranchExists(ctx, "main")
	if err != nil {
		t.Fatalf("failed to check if main branch exists: %v", err)
	}

	if !exists {
		t.Error("expected main branch to exist")
	}

	// Check if non-existent branch exists (should be false)
	exists, err = client.BranchExists(ctx, "non-existent-branch")
	if err != nil {
		t.Fatalf("failed to check if non-existent branch exists: %v", err)
	}

	if exists {
		t.Error("expected non-existent branch to not exist")
	}

	// Create a new branch and check again
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/exists-test",
		Path:   filepath.Join(repoPath, "exists-test"),
	}

	worktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Now the branch should exist
	exists, err = client.BranchExists(ctx, "feature/exists-test")
	if err != nil {
		t.Fatalf("failed to check if new branch exists: %v", err)
	}

	if !exists {
		t.Error("expected new branch to exist")
	}

	// Cleanup
	client.RemoveWorktree(ctx, worktree.Path, true)
}

// TestIntegration_WorktreeFilter tests filtering worktrees
func TestIntegration_WorktreeFilter(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	cfg := config.DefaultConfig()
	service, err := worktree.NewService(client, cfg)
	if err != nil {
		t.Fatalf("failed to create worktree service: %v", err)
	}

	// Create multiple worktrees
	branches := []string{
		"feature/one",
		"feature/two",
		"bugfix/three",
	}
	paths := make([]string, 0, len(branches))
	for _, branch := range branches {
		spec := domain.WorktreeCreateSpec{
			Branch: branch,
			Path:   filepath.Join(repoPath, strings.ReplaceAll(branch, "/", "-")),
		}
		worktree, err := client.AddWorktree(ctx, spec)
		if err != nil {
			t.Fatalf("failed to add worktree for %s: %v", branch, err)
		}
		paths = append(paths, worktree.Path)
	}

	// Cleanup all worktrees at the end
	defer func() {
		for _, path := range paths {
			client.RemoveWorktree(ctx, path, true)
		}
	}()

	// Test filter by multiple branches
	filter := &domain.WorktreeFilter{
		Branches: []string{"feature/one", "feature/two"},
	}

	filtered, err := service.List(ctx, filter)
	if err != nil {
		t.Fatalf("failed to list filtered worktrees: %v", err)
	}

	// Should have only the two filtered branches
	expectedCount := 2 // feature/one + feature/two
	if len(filtered) != expectedCount {
		t.Errorf("expected %d filtered worktrees, got %d", expectedCount, len(filtered))
	}

	// Verify only the requested branches are present
	for _, w := range filtered {
		if w.Branch != "feature/one" && w.Branch != "feature/two" {
			t.Errorf("unexpected branch in filtered results: %s", w.Branch)
		}
	}

	// Test path prefix filter
	pathFilter := &domain.WorktreeFilter{
		PathPrefix: filepath.Join(repoPath, "feature-"),
	}

	pathFiltered, err := service.List(ctx, pathFilter)
	if err != nil {
		t.Fatalf("failed to list worktrees filtered by path: %v", err)
	}

	// Should only have worktrees with paths starting with the prefix
	for _, w := range pathFiltered {
		if !strings.HasPrefix(w.Path, filepath.Join(repoPath, "feature-")) && w.Branch != "main" {
			t.Errorf("worktree %s does not match path prefix", w.Path)
		}
	}
}

// TestIntegration_WorktreeForce tests the force flag when removing worktrees
func TestIntegration_WorktreeForce(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Create a worktree
	spec := domain.WorktreeCreateSpec{
		Branch: "feature/force-test",
		Path:   filepath.Join(repoPath, "force-test"),
	}

	worktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}

	// Add an untracked file to the worktree
	untrackedFile := filepath.Join(worktree.Path, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked"), 0o644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	// Try to remove without force (should fail)
	err = client.RemoveWorktree(ctx, worktree.Path, false)
	if err == nil {
		t.Error("expected error when removing worktree with untracked files without force")
	}

	// Now remove with force (should succeed)
	if err := client.RemoveWorktree(ctx, worktree.Path, true); err != nil {
		t.Fatalf("failed to remove worktree with force: %v", err)
	}

	// Verify it's gone
	worktrees, _ := client.ListWorktrees(ctx)
	for _, w := range worktrees {
		if w.Branch == "feature/force-test" {
			t.Error("removed worktree still found in list")
		}
	}
}

// TestIntegration_WorktreeNoCheckout tests creating a worktree without checking out files
func TestIntegration_WorktreeNoCheckout(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Create a worktree without checkout
	spec := domain.WorktreeCreateSpec{
		Branch:   "feature/no-checkout",
		Path:     filepath.Join(repoPath, "no-checkout"),
		Checkout: false,
	}

	worktree, err := client.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("failed to create worktree without checkout: %v", err)
	}

	// Verify the worktree was created
	if worktree.Branch != "feature/no-checkout" {
		t.Errorf("expected branch feature/no-checkout, got %s", worktree.Branch)
	}

	// Verify .git directory exists but working directory might be empty
	gitDir := filepath.Join(worktree.Path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error(".git directory should exist in worktree")
	}

	// Cleanup
	client.RemoveWorktree(ctx, worktree.Path, true)
}

// TestIntegration_InvalidSpec tests validation of worktree create specs
func TestIntegration_InvalidSpec(t *testing.T) {
	skipIfNoGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		t.Fatalf("failed to create git client: %v", err)
	}

	// Test with empty branch name (should fail validation)
	spec := domain.WorktreeCreateSpec{
		Branch: "",
		Path:   filepath.Join(repoPath, "invalid"),
	}

	_, err = client.AddWorktree(ctx, spec)
	if err == nil {
		t.Error("expected error for invalid spec with empty branch")
	}
}

// Benchmark_AddWorktree benchmarks the performance of adding a worktree
func Benchmark_AddWorktree(b *testing.B) {
	skipIfNoGit(b)

	repoPath, _ := setupTestRepo(b)

	ctx := context.Background()

	// Change to repo directory
	originalWd, err := os.Getwd()
	if err != nil {
		b.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(repoPath); err != nil {
		b.Fatalf("failed to change to repo directory: %v", err)
	}

	client, err := git.NewClient()
	if err != nil {
		b.Fatalf("failed to create git client: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		branchName := fmt.Sprintf("feature/bench-%d", i)
		spec := domain.WorktreeCreateSpec{
			Branch: branchName,
			Path:   filepath.Join(repoPath, fmt.Sprintf("bench-%d", i)),
		}

		worktree, err := client.AddWorktree(ctx, spec)
		if err != nil {
			b.Fatalf("failed to add worktree: %v", err)
		}

		// Cleanup immediately to avoid running out of inodes
		client.RemoveWorktree(ctx, worktree.Path, true)
	}
}
