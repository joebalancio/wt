# Dedicated Mode Path Collision Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix path collisions in dedicated mode by switching default to per-repo and adding repo name namespace to dedicated paths.

**Architecture:** Change `IsDedicated()` to return false for empty location (new default: per-repo). Update `ResolvePath()` in worktree service and `getWorktreePath()` in stack service to include repo name in dedicated mode paths. Add collision detection as safety net.

**Tech Stack:** Go 1.21+, standard library (filepath, os, fmt)

**Design Doc:** `docs/plans/2026-02-16-dedicated-path-collision-design.md`

---

## Task 1: Change Default to Per-Repo (Config)

**Files:**
- Modify: `internal/config/config.go:69-71`
- Modify: `internal/config/config.go:104-106` (DefaultConfig)
- Modify: `internal/config/config_test.go:143-161`

**Step 1: Write the failing test**

Update the test in `internal/config/config_test.go` to expect per-repo as default:

```go
func TestWorktreeConfig_IsDedicated(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     bool
	}{
		{"empty defaults to per-repo", "", false},  // CHANGED: was true
		{"explicit dedicated", "dedicated", true},
		{"per-repo", "per-repo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := WorktreeConfig{Location: tt.location}
			if got := cfg.IsDedicated(); got != tt.want {
				t.Errorf("IsDedicated() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

Also update `TestDefaultConfig_HasWorktreeSettings`:

```go
func TestDefaultConfig_HasWorktreeSettings(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Worktree.IsDedicated() {
		t.Error("default config should use per-repo worktree location")  // CHANGED
	}
	// Remove the GetDedicatedPath check since default is now per-repo
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestWorktreeConfig_IsDedicated -v`
Expected: FAIL - "empty defaults to per-repo" test case fails (got true, want false)

**Step 3: Write minimal implementation**

Update `internal/config/config.go`:

```go
// IsDedicated returns true if using dedicated worktree location
func (w *WorktreeConfig) IsDedicated() bool {
	return w.Location == "dedicated"  // empty = per-repo (not dedicated)
}
```

Update `DefaultConfig()` to not set dedicated as default:

```go
func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{},
		Tmux: TmuxConfig{
			Layout:         "main-vertical",
			WindowName:     "work",
			AttachOnCreate: true,
			WindowNaming: TmuxWindowNamingConfig{
				MaxLength:         16,
				AbbreviateIssueID: true,
			},
		},
		Worktree: WorktreeConfig{
			Location:      "",  // CHANGED: empty means per-repo (default)
			DedicatedPath: "~/worktrees",
		},
		Spice: SpiceConfig{
			BinaryPath: "",
		},
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): change default worktree location to per-repo

Empty location now defaults to per-repo mode instead of dedicated.
Users must explicitly set 'location: dedicated' to use dedicated mode.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 2: Add Repo Name to Dedicated Paths (Worktree Service)

**Files:**
- Modify: `internal/worktree/service.go:87-111`
- Modify: `internal/worktree/service_test.go`

**Step 1: Write the failing test**

Add test in `internal/worktree/service_test.go`:

```go
func TestResolvePath_Dedicated_addsRepoName(t *testing.T) {
	mockGit := &mockGitClient{
		getRepoInfoFunc: func(ctx context.Context) (*domain.GitRepo, error) {
			return &domain.GitRepo{RootPath: "/home/user/projects/my-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "dedicated"
	cfg.Worktree.DedicatedPath = "/tmp/worktrees"

	svc, err := NewService(mockGit, cfg)
	if err != nil {
		t.Fatal(err)
	}

	path, err := svc.ResolvePath(context.Background(), "feature/auth", "")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}

	// Path should include repo name: /tmp/worktrees/my-repo/feature/auth
	expected := "/tmp/worktrees/my-repo/feature/auth"
	if path != expected {
		t.Errorf("ResolvePath() = %q, want %q", path, expected)
	}
}

func TestResolvePath_PerRepo_unchanged(t *testing.T) {
	mockGit := &mockGitClient{
		getRepoInfoFunc: func(ctx context.Context) (*domain.GitRepo, error) {
			return &domain.GitRepo{RootPath: "/home/user/projects/my-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig()  // per-repo is default

	svc, err := NewService(mockGit, cfg)
	if err != nil {
		t.Fatal(err)
	}

	path, err := svc.ResolvePath(context.Background(), "feature/auth", "")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}

	// Path should be per-repo style: <repo>/.worktrees/<branch>
	expected := "/home/user/projects/my-repo/.worktrees/feature/auth"
	if path != expected {
		t.Errorf("ResolvePath() = %q, want %q", path, expected)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -run TestResolvePath_Dedicated_addsRepoName -v`
Expected: FAIL - path is `/tmp/worktrees/feature/auth`, want `/tmp/worktrees/my-repo/feature/auth`

**Step 3: Write minimal implementation**

Update `internal/worktree/service.go`:

```go
// ResolvePath returns the worktree path for a branch.
// If explicitPath is provided, it's used as-is.
// Otherwise, path is resolved from config based on worktree.location setting.
func (s *Service) ResolvePath(ctx context.Context, branch string, explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}

	if s.cfg.Worktree.IsDedicated() {
		dedicatedPath := s.cfg.Worktree.GetDedicatedPath()
		// Expand ~ to home directory
		if strings.HasPrefix(dedicatedPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("getting home directory: %w", err)
			}
			dedicatedPath = filepath.Join(home, dedicatedPath[2:])
		}

		// Get repo info for namespace
		repoInfo, err := s.git.GetRepoInfo(ctx)
		if err != nil {
			return "", fmt.Errorf("getting repo info: %w", err)
		}
		repoName := filepath.Base(repoInfo.RootPath)

		return filepath.Join(dedicatedPath, repoName, branch), nil
	}

	// per-repo mode
	repoInfo, err := s.git.GetRepoInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("getting repo info: %w", err)
	}
	return filepath.Join(repoInfo.RootPath, ".worktrees", branch), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -run TestResolvePath -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/service.go internal/worktree/service_test.go
git commit -m "feat(worktree): add repo name namespace to dedicated mode paths

Dedicated mode paths now include repo name to prevent collisions:
  Before: ~/worktrees/feature/auth
  After:  ~/worktrees/my-repo/feature/auth

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 3: Add Repo Name to Dedicated Paths (Stack Service)

**Files:**
- Modify: `internal/stack/service.go:150-162`
- Modify: `internal/stack/service_test.go`

**Step 1: Write the failing test**

Add test in `internal/stack/service_test.go`:

```go
func TestGetWorktreePath_Dedicated_addsRepoName(t *testing.T) {
	mockGit := &mockGitClient{
		getRepoInfoFunc: func(ctx context.Context) (*domain.GitRepo, error) {
			return &domain.GitRepo{RootPath: "/home/user/projects/my-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "dedicated"
	cfg.Worktree.DedicatedPath = "/tmp/worktrees"

	svc := NewService(mockGit, nil, cfg)

	path, err := svc.GetWorktreePathForBranch(context.Background(), "feature/auth")
	if err != nil {
		t.Fatalf("GetWorktreePathForBranch() error = %v", err)
	}

	// Path should include repo name
	expected := "/tmp/worktrees/my-repo/feature/auth"
	if path != expected {
		t.Errorf("GetWorktreePathForBranch() = %q, want %q", path, expected)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/stack/... -run TestGetWorktreePath_Dedicated_addsRepoName -v`
Expected: FAIL - path doesn't include repo name

**Step 3: Write minimal implementation**

Update `internal/stack/service.go`:

```go
// getWorktreePath returns the worktree path for a branch
func (s *Service) getWorktreePath(ctx context.Context, branch string) string {
	if s.cfg.Worktree.IsDedicated() {
		dedicatedPath := s.cfg.Worktree.GetDedicatedPath()

		// Get repo info for namespace
		repoInfo, err := s.git.GetRepoInfo(ctx)
		if err != nil || repoInfo == nil {
			// Fallback: use branch only if we can't get repo info
			return filepath.Join(dedicatedPath, branch)
		}
		repoName := filepath.Base(repoInfo.RootPath)

		return filepath.Join(dedicatedPath, repoName, branch)
	}
	// per-repo mode - get repo info safely
	repoInfo, err := s.git.GetRepoInfo(ctx)
	if err != nil || repoInfo == nil {
		// Fallback: use current directory if we can't get repo info
		return filepath.Join(".", ".worktrees", branch)
	}
	return filepath.Join(repoInfo.RootPath, ".worktrees", branch)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/stack/... -run TestGetWorktreePath -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/stack/service.go internal/stack/service_test.go
git commit -m "feat(stack): add repo name namespace to dedicated mode paths

Stack service now uses same namespacing as worktree service for
consistency in dedicated mode.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 4: Add Collision Detection

**Files:**
- Modify: `internal/worktree/service.go`
- Modify: `internal/worktree/service_test.go`

**Step 1: Write the failing test**

Add test in `internal/worktree/service_test.go`:

```go
import (
	"os"
	"path/filepath"
)

func TestResolvePath_CollisionDetection(t *testing.T) {
	// Create a temp directory to simulate existing worktree from different repo
	tempDir := t.TempDir()
	existingPath := filepath.Join(tempDir, "my-repo", "feature", "auth")
	if err := os.MkdirAll(existingPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a .git file to simulate worktree pointing to different repo
	gitFile := filepath.Join(existingPath, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: /other/repo/.git/worktrees/feature/auth"), 0o644); err != nil {
		t.Fatal(err)
	}

	mockGit := &mockGitClient{
		getRepoInfoFunc: func(ctx context.Context) (*domain.GitRepo, error) {
			// Current repo is different from the one that created the worktree
			return &domain.GitRepo{RootPath: "/home/user/projects/other-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "dedicated"
	cfg.Worktree.DedicatedPath = tempDir

	svc, err := NewService(mockGit, cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.ResolvePath(context.Background(), "feature/auth", "")
	if err == nil {
		t.Error("ResolvePath() expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("ResolvePath() error should mention collision, got: %v", err)
	}
}

func TestResolvePath_SameRepo_NoCollision(t *testing.T) {
	// Create a temp directory to simulate existing worktree from SAME repo
	tempDir := t.TempDir()
	existingPath := filepath.Join(tempDir, "my-repo", "feature", "auth")
	if err := os.MkdirAll(existingPath, 0o755); err != nil {
		t.Fatal(err)
	}

	mockGit := &mockGitClient{
		getRepoInfoFunc: func(ctx context.Context) (*domain.GitRepo, error) {
			// Current repo is the SAME as the one that created the worktree
			return &domain.GitRepo{RootPath: "/home/user/projects/my-repo", DefaultBranch: "main"}, nil
		},
	}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "dedicated"
	cfg.Worktree.DedicatedPath = tempDir

	svc, err := NewService(mockGit, cfg)
	if err != nil {
		t.Fatal(err)
	}

	path, err := svc.ResolvePath(context.Background(), "feature/auth", "")
	if err != nil {
		t.Errorf("ResolvePath() unexpected error: %v", err)
	}
	if path != existingPath {
		t.Errorf("ResolvePath() = %q, want %q", path, existingPath)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -run TestResolvePath_CollisionDetection -v`
Expected: FAIL - no collision detection implemented

**Step 3: Write minimal implementation**

Add collision detection helper and update `ResolvePath` in `internal/worktree/service.go`:

```go
import (
	"bufio"
	// ... existing imports
)

// ErrPathCollision is returned when a worktree path already exists from a different repo
var ErrPathCollision = errors.New("path collision detected")

// ResolvePath returns the worktree path for a branch.
// If explicitPath is provided, it's used as-is.
// Otherwise, path is resolved from config based on worktree.location setting.
func (s *Service) ResolvePath(ctx context.Context, branch string, explicitPath string) (string, error) {
	if explicitPath != "" {
		return explicitPath, nil
	}

	if s.cfg.Worktree.IsDedicated() {
		dedicatedPath := s.cfg.Worktree.GetDedicatedPath()
		// Expand ~ to home directory
		if strings.HasPrefix(dedicatedPath, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("getting home directory: %w", err)
			}
			dedicatedPath = filepath.Join(home, dedicatedPath[2:])
		}

		// Get repo info for namespace
		repoInfo, err := s.git.GetRepoInfo(ctx)
		if err != nil {
			return "", fmt.Errorf("getting repo info: %w", err)
		}
		repoName := filepath.Base(repoInfo.RootPath)
		targetPath := filepath.Join(dedicatedPath, repoName, branch)

		// Check for collision
		if err := s.checkPathCollision(targetPath, repoInfo.RootPath); err != nil {
			return "", err
		}

		return targetPath, nil
	}

	// per-repo mode
	repoInfo, err := s.git.GetRepoInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("getting repo info: %w", err)
	}
	return filepath.Join(repoInfo.RootPath, ".worktrees", branch), nil
}

// checkPathCollision verifies the target path doesn't exist from a different repo
func (s *Service) checkPathCollision(targetPath, currentRepoRoot string) error {
	// If path doesn't exist, no collision
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil
	}

	// Path exists - check if it's from the same repo
	existingRepo, err := s.getWorktreeOriginRepo(targetPath)
	if err != nil {
		// Can't determine origin, allow it (existing behavior)
		return nil
	}

	// Compare repo roots
	if existingRepo != "" && existingRepo != currentRepoRoot {
		return fmt.Errorf("%w: %s already exists from another repo (%s)\n\nOptions:\n  --path <explicit-path>  # specify a different path\n  wt config set worktree.location per-repo  # use per-repo mode",
			ErrPathCollision, targetPath, existingRepo)
	}

	return nil
}

// getWorktreeOriginRepo reads the .git file in a worktree to find the origin repo path
func (s *Service) getWorktreeOriginRepo(worktreePath string) (string, error) {
	gitFile := filepath.Join(worktreePath, ".git")
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return "", err
	}

	// Parse gitdir line: "gitdir: /path/to/repo/.git/worktrees/branch"
	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "gitdir: ") {
		return "", fmt.Errorf("invalid .git file format")
	}

	gitdir := strings.TrimPrefix(content, "gitdir: ")

	// Extract repo root from gitdir
	// gitdir is typically: /path/to/repo/.git/worktrees/branch
	// We need: /path/to/repo
	parts := strings.Split(gitdir, string(filepath.Separator))
	for i, part := range parts {
		if part == ".git" {
			repoPath := filepath.Join(parts[:i]...)
			if repoPath == "" {
				return "/", nil
			}
			return "/" + repoPath, nil
		}
	}

	return "", fmt.Errorf("could not parse repo path from gitdir: %s", gitdir)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -run TestResolvePath -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/worktree/service.go internal/worktree/service_test.go
git commit -m "feat(worktree): add collision detection for dedicated mode

Detects when a worktree path already exists from a different repo
and provides helpful error message with options.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 5: Add Integration Tests

**Files:**
- Create: `tests/dedicated_namespacing_test.go`

**Step 1: Write the integration tests**

Create `tests/dedicated_namespacing_test.go`:

```go
package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/worktree"
)

func TestIntegration_DedicatedMode_Namespacing(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	// Create two repos with same branch name
	repoA, _ := setupTestRepo(t)
	repoB, _ := setupTestRepo(t)
	defer os.RemoveAll(repoA)
	defer os.RemoveAll(repoB)

	// Create a shared worktrees directory
	worktreesDir := t.TempDir()

	// Create worktree from repo A
	cfgA := config.DefaultConfig()
	cfgA.Worktree.Location = "dedicated"
	cfgA.Worktree.DedicatedPath = worktreesDir

	gitClientA, _ := git.NewClient()
	svcA, _ := worktree.NewService(gitClientA, cfgA)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(repoA)

	pathA, err := svcA.ResolvePath(context.Background(), "feature/test", "")
	if err != nil {
		t.Fatalf("ResolvePath from repo A failed: %v", err)
	}

	// Create worktree from repo B
	cfgB := config.DefaultConfig()
	cfgB.Worktree.Location = "dedicated"
	cfgB.Worktree.DedicatedPath = worktreesDir

	gitClientB, _ := git.NewClient()
	svcB, _ := worktree.NewService(gitClientB, cfgB)

	os.Chdir(repoB)

	pathB, err := svcB.ResolvePath(context.Background(), "feature/test", "")
	if err != nil {
		t.Fatalf("ResolvePath from repo B failed: %v", err)
	}

	// Paths should be different due to namespacing
	if pathA == pathB {
		t.Errorf("Paths should be different:\n  repo A: %s\n  repo B: %s", pathA, pathB)
	}

	// Paths should include repo names
	repoAName := filepath.Base(repoA)
	repoBName := filepath.Base(repoB)

	expectedA := filepath.Join(worktreesDir, repoAName, "feature/test")
	expectedB := filepath.Join(worktreesDir, repoBName, "feature/test")

	if pathA != expectedA {
		t.Errorf("Path A = %q, want %q", pathA, expectedA)
	}
	if pathB != expectedB {
		t.Errorf("Path B = %q, want %q", pathB, expectedB)
	}
}

func TestIntegration_DefaultIsPerRepo(t *testing.T) {
	skipIfNoGit(t)
	skipIfNoIntegrationTest(t)

	repoPath, _ := setupTestRepo(t)
	defer os.RemoveAll(repoPath)

	cfg := config.DefaultConfig()

	// Default should be per-repo
	if cfg.Worktree.IsDedicated() {
		t.Error("Default config should use per-repo mode, not dedicated")
	}

	gitClient, _ := git.NewClient()
	svc, _ := worktree.NewService(gitClient, cfg)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(repoPath)

	path, err := svc.ResolvePath(context.Background(), "feature/test", "")
	if err != nil {
		t.Fatalf("ResolvePath failed: %v", err)
	}

	// Path should be per-repo style
	expected := filepath.Join(repoPath, ".worktrees", "feature/test")
	if path != expected {
		t.Errorf("ResolvePath() = %q, want %q", path, expected)
	}
}
```

**Step 2: Run integration tests**

Run: `WT_INTEGRATION_TEST=1 go test ./tests/... -run TestIntegration_DedicatedMode -v`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/dedicated_namespacing_test.go
git commit -m "test: add integration tests for dedicated mode namespacing

Tests verify:
- Different repos with same branch get different paths
- Default config uses per-repo mode

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Task 6: Run Full Test Suite and Verify

**Step 1: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 2: Run linter**

Run: `make lint`
Expected: No errors

**Step 3: Build binary**

Run: `make build`
Expected: Binary builds successfully

**Step 4: Manual verification**

```bash
# Create temp repo
cd $(mktemp -d)
git init
git config user.email "test@test.com"
git config user.name "Test"
echo "test" > README.md
git add README.md
git commit -m "init"

# Run wt with no config (should use per-repo default)
~/projects/wt/.worktrees/feature/wt-kie/bin/wt add test-branch

# Verify worktree created in .worktrees/
ls -la .worktrees/test-branch
```

Expected: Worktree created at `<repo>/.worktrees/test-branch`

**Step 5: Final commit (if any fixes needed)**

```bash
git status
# If any changes, commit them
```

---

## Summary

| Task | Description | Files Changed |
|------|-------------|---------------|
| 1 | Change default to per-repo | config.go, config_test.go |
| 2 | Namespace worktree paths | worktree/service.go, service_test.go |
| 3 | Namespace stack paths | stack/service.go, service_test.go |
| 4 | Add collision detection | worktree/service.go, service_test.go |
| 5 | Add integration tests | tests/dedicated_namespacing_test.go |
| 6 | Verify all tests pass | - |
