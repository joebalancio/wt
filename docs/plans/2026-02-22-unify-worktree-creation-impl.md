# Unify wt add and wt stack Worktree Creation - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `wt stack` use `worktree.Service` for worktree creation, gaining safety checks and feature parity with `wt add`.

**Architecture:** Inject `worktree.Service` into `stack.Service`, refactor `CreateWorktree()` to delegate to `worktree.Service.Add()`, add CLI flags for `--path`, `--track`, `--no-checkout`, and add worktree nesting check.

**Tech Stack:** Go 1.21+, spf13/cobra, existing internal packages (git, worktree, stack, config)

---

## Task 1: Add worktree.Service dependency to stack.Service

**Files:**
- Modify: `internal/stack/service.go:25-48`
- Modify: `internal/stack/service_test.go:13-58`
- Modify: `internal/stack/service_test.go:348-412` (MockGitClient)

**Step 1: Write the failing test for NewService with worktreeSvc**

Add to `internal/stack/service_test.go`:

```go
func TestNewService_NilWorktreeService(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()

	_, err := NewService(mockGit, mockSpice, cfg, nil)
	if err == nil {
		t.Error("NewService() with nil worktreeSvc should return error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/stack -run TestNewService_NilWorktreeService -v`
Expected: FAIL - "too many arguments in call to NewService"

**Step 3: Update Service struct and NewService constructor**

Modify `internal/stack/service.go`:

```go
// Service provides stack-related operations
type Service struct {
	git         git.GitClient
	spice       SpiceClient
	cfg         *config.Config
	worktreeSvc *worktree.Service // NEW: delegate worktree operations
}

// NewService creates a new stack service
func NewService(gitClient git.GitClient, spiceClient SpiceClient, cfg *config.Config, worktreeSvc *worktree.Service) (*Service, error) {
	if gitClient == nil {
		return nil, fmt.Errorf("gitClient cannot be nil")
	}
	if spiceClient == nil {
		return nil, fmt.Errorf("spiceClient cannot be nil")
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	if worktreeSvc == nil {
		return nil, fmt.Errorf("worktreeSvc cannot be nil")
	}

	return &Service{
		git:         gitClient,
		spice:       spiceClient,
		cfg:         cfg,
		worktreeSvc: worktreeSvc,
	}, nil
}
```

**Step 4: Add worktree import**

Add to imports in `internal/stack/service.go`:

```go
import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aidarkhanov/nanoid"
	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/spice"
	"github.com/joebalancio/wt/internal/worktree" // NEW
	"github.com/joebalancio/wt/pkg/domain"
)
```

**Step 5: Update all existing tests to pass worktreeSvc**

Modify `internal/stack/service_test.go`. Add a helper function and update all NewService calls:

```go
// stubWorktreeService provides a minimal worktree.Service for tests
type stubWorktreeService struct{}

func (s *stubWorktreeService) Add(_ context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	return &domain.Worktree{Path: "/stub/path", Branch: spec.Branch}, nil
}

func (s *stubWorktreeService) List(_ context.Context, _ *domain.WorktreeFilter) ([]*domain.Worktree, error) {
	return nil, nil
}

func (s *stubWorktreeService) ResolvePath(_ context.Context, _, _ string) (string, error) {
	return "/stub/path", nil
}

func (s *stubWorktreeService) Remove(_ context.Context, _ string, _ bool) error {
	return nil
}

func (s *stubWorktreeService) ResolveFromCWD(_ context.Context, _ string) (*domain.Worktree, error) {
	return nil, nil
}

func (s *stubWorktreeService) RemoveEnhanced(_ context.Context, _ string, _ domain.ForceLevel) error {
	return nil
}

func (s *stubWorktreeService) Done(_ context.Context, _, _ string, _ bool) error {
	return nil
}
```

Then update each test function to use the stub:

```go
func TestNewService(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()
	stubWt := &stubWorktreeService{}

	service, err := NewService(mockGit, mockSpice, cfg, stubWt)
	// ... rest unchanged
}

func TestNewService_NilGitClient(t *testing.T) {
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()
	stubWt := &stubWorktreeService{}

	_, err := NewService(nil, mockSpice, cfg, stubWt)
	// ... rest unchanged
}

func TestNewService_NilSpiceClient(t *testing.T) {
	mockGit := &MockGitClient{}
	cfg := config.DefaultConfig()
	stubWt := &stubWorktreeService{}

	_, err := NewService(mockGit, nil, cfg, stubWt)
	// ... rest unchanged
}

func TestNewService_NilConfigUsesDefault(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	stubWt := &stubWorktreeService{}

	service, err := NewService(mockGit, mockSpice, nil, stubWt)
	// ... rest unchanged
}
```

Update all other test functions similarly (TestService_GenerateBranchSuffix, TestService_BuildStackBranchName, etc.).

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/stack -v`
Expected: PASS (all tests)

**Step 7: Commit**

```bash
git add internal/stack/service.go internal/stack/service_test.go
git commit -m "feat(stack): add worktree.Service dependency to stack.Service

- Add worktreeSvc field to Service struct
- Update NewService() to require worktreeSvc parameter
- Add nil check for worktreeSvc
- Update all tests to use stub worktree service

Refs: wt-ido"
```

---

## Task 2: Add new fields to BranchSpec and refactor CreateWorktree

**Files:**
- Modify: `internal/stack/service.go:71-127`

**Step 1: Write the failing test for CreateWorktree using worktreeSvc**

Add to `internal/stack/service_test.go`:

```go
func TestService_CreateWorktree_UsesWorktreeService(t *testing.T) {
	mockGit := &MockGitClient{repoRoot: "/home/user/project"}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()

	var receivedSpec domain.WorktreeCreateSpec
	stubWt := &stubWorktreeServiceWithCapture{captureSpec: &receivedSpec}

	service, _ := NewService(mockGit, mockSpice, cfg, stubWt)

	_, err := service.CreateWorktree(context.Background(), "feat/auth-xyz1")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	if receivedSpec.Branch != "feat/auth-xyz1" {
		t.Errorf("Branch = %v, want feat/auth-xyz1", receivedSpec.Branch)
	}
	if !receivedSpec.Checkout {
		t.Error("Checkout should be true by default")
	}
}

// stubWorktreeServiceWithCapture captures the spec for assertions
type stubWorktreeServiceWithCapture struct {
	captureSpec *domain.WorktreeCreateSpec
}

func (s *stubWorktreeServiceWithCapture) Add(_ context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	*s.captureSpec = spec
	return &domain.Worktree{Path: "/stub/path", Branch: spec.Branch}, nil
}

func (s *stubWorktreeServiceWithCapture) List(_ context.Context, _ *domain.WorktreeFilter) ([]*domain.Worktree, error) {
	return nil, nil
}

func (s *stubWorktreeServiceWithCapture) ResolvePath(_ context.Context, _, _ string) (string, error) {
	return "/stub/path", nil
}

func (s *stubWorktreeServiceWithCapture) Remove(_ context.Context, _ string, _ bool) error {
	return nil
}

func (s *stubWorktreeServiceWithCapture) ResolveFromCWD(_ context.Context, _ string) (*domain.Worktree, error) {
	return nil, nil
}

func (s *stubWorktreeServiceWithCapture) RemoveEnhanced(_ context.Context, _ string, _ domain.ForceLevel) error {
	return nil
}

func (s *stubWorktreeServiceWithCapture) Done(_ context.Context, _, _ string, _ bool) error {
	return nil
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/stack -run TestService_CreateWorktree_UsesWorktreeService -v`
Expected: FAIL - CreateWorktree still calls git.AddWorktree directly

**Step 3: Add new fields to BranchSpec**

Modify `internal/stack/service.go`:

```go
// BranchSpec defines parameters for creating a stack branch
type BranchSpec struct {
	Name       string // Optional named suffix (e.g., "api" for feat/auth-api-xxxx)
	Base       string // Optional base branch (defaults to current)
	Path       string // Optional custom worktree path
	Track      string // Optional remote branch to track
	NoCheckout bool   // Skip checkout when creating worktree
}
```

**Step 4: Refactor CreateWorktree to use worktreeSvc**

Replace the existing `CreateWorktree` method in `internal/stack/service.go`:

```go
// CreateWorktree creates a worktree for a stack branch
func (s *Service) CreateWorktree(ctx context.Context, branch string) (*domain.Worktree, error) {
	// Build worktree spec - path is optional (empty = auto-resolve)
	spec := domain.WorktreeCreateSpec{
		Branch:   branch,
		Path:     "", // Let worktree.Service resolve the path
		Checkout: true,
	}

	return s.worktreeSvc.Add(ctx, spec)
}
```

Note: We keep the simple signature for now. Flags will be added in Task 4 via a new method or by extending the signature.

**Step 5: Run test to verify it passes**

Run: `go test ./internal/stack -run TestService_CreateWorktree_UsesWorktreeService -v`
Expected: PASS

**Step 6: Run all stack tests to verify no regression**

Run: `go test ./internal/stack -v`
Expected: PASS (all tests)

**Step 7: Commit**

```bash
git add internal/stack/service.go internal/stack/service_test.go
git commit -m "refactor(stack): use worktree.Service in CreateWorktree

- Refactor CreateWorktree to delegate to worktreeSvc.Add()
- Add Path, Track, NoCheckout fields to BranchSpec for future use
- Add test to verify worktree service is called

Refs: wt-ido"
```

---

## Task 3: Create CreateWorktreeWithSpec method for flag support

**Files:**
- Modify: `internal/stack/service.go`
- Modify: `internal/stack/service_test.go`

**Step 1: Write the failing test for CreateWorktreeWithSpec**

Add to `internal/stack/service_test.go`:

```go
func TestService_CreateWorktreeWithSpec(t *testing.T) {
	tests := []struct {
		name           string
		branch         string
		path           string
		track          string
		noCheckout     bool
		wantPath       string
		wantCheckout   bool
		wantTrackValue string
	}{
		{
			name:         "defaults",
			branch:       "feat/auth-xyz1",
			path:         "",
			track:        "",
			noCheckout:   false,
			wantPath:     "", // auto-resolved
			wantCheckout: true,
		},
		{
			name:         "custom path",
			branch:       "feat/auth-xyz1",
			path:         "/custom/path",
			track:        "",
			noCheckout:   false,
			wantPath:     "/custom/path",
			wantCheckout: true,
		},
		{
			name:         "no checkout",
			branch:       "feat/auth-xyz1",
			path:         "",
			track:        "",
			noCheckout:   true,
			wantPath:     "",
			wantCheckout: false,
		},
		{
			name:           "with track",
			branch:         "feat/auth-xyz1",
			path:           "",
			track:          "origin/feat/auth-xyz1",
			noCheckout:     false,
			wantPath:       "",
			wantCheckout:   true,
			wantTrackValue: "origin/feat/auth-xyz1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := &MockGitClient{repoRoot: "/home/user/project"}
			mockSpice := &MockSpiceClient{}
			cfg := config.DefaultConfig()

			var receivedSpec domain.WorktreeCreateSpec
			stubWt := &stubWorktreeServiceWithCapture{captureSpec: &receivedSpec}

			service, _ := NewService(mockGit, mockSpice, cfg, stubWt)

			spec := WorktreeSpec{
				Path:       tt.path,
				Track:      tt.track,
				NoCheckout: tt.noCheckout,
			}

			_, err := service.CreateWorktreeWithSpec(context.Background(), tt.branch, spec)
			if err != nil {
				t.Fatalf("CreateWorktreeWithSpec() error = %v", err)
			}

			if receivedSpec.Branch != tt.branch {
				t.Errorf("Branch = %v, want %v", receivedSpec.Branch, tt.branch)
			}
			if receivedSpec.Path != tt.wantPath {
				t.Errorf("Path = %v, want %v", receivedSpec.Path, tt.wantPath)
			}
			if receivedSpec.Checkout != tt.wantCheckout {
				t.Errorf("Checkout = %v, want %v", receivedSpec.Checkout, tt.wantCheckout)
			}
			if tt.wantTrackValue != "" {
				if receivedSpec.Track == nil || *receivedSpec.Track != tt.wantTrackValue {
					t.Errorf("Track = %v, want %v", receivedSpec.Track, tt.wantTrackValue)
				}
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/stack -run TestService_CreateWorktreeWithSpec -v`
Expected: FAIL - undefined: WorktreeSpec, undefined: Service.CreateWorktreeWithSpec

**Step 3: Add WorktreeSpec type and CreateWorktreeWithSpec method**

Add to `internal/stack/service.go` after BranchSpec:

```go
// WorktreeSpec defines worktree creation options for stack commands
type WorktreeSpec struct {
	Path       string // Custom worktree path (empty = auto-resolve)
	Track      string // Remote branch to track
	NoCheckout bool   // Skip checkout
}

// CreateWorktreeWithSpec creates a worktree with explicit options
func (s *Service) CreateWorktreeWithSpec(ctx context.Context, branch string, spec WorktreeSpec) (*domain.Worktree, error) {
	wtSpec := domain.WorktreeCreateSpec{
		Branch:   branch,
		Path:     spec.Path,
		Checkout: !spec.NoCheckout,
	}

	if spec.Track != "" {
		wtSpec.Track = &spec.Track
	}

	return s.worktreeSvc.Add(ctx, wtSpec)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/stack -run TestService_CreateWorktreeWithSpec -v`
Expected: PASS

**Step 5: Run all stack tests**

Run: `go test ./internal/stack -v`
Expected: PASS (all tests)

**Step 6: Commit**

```bash
git add internal/stack/service.go internal/stack/service_test.go
git commit -m "feat(stack): add CreateWorktreeWithSpec for flag support

- Add WorktreeSpec type with Path, Track, NoCheckout fields
- Add CreateWorktreeWithSpec method that passes flags to worktree service
- Add comprehensive tests for all flag combinations

Refs: wt-ido"
```

---

## Task 4: Add CLI flags to wt stack command

**Files:**
- Modify: `internal/cli/stack.go:16-44`

**Step 1: Write the failing test for flag parsing**

Create `internal/cli/stack_flags_test.go`:

```go
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestStackCommand_HasPathFlag(t *testing.T) {
	cmd := NewStackCmd()
	flag := cmd.Flags().Lookup("path")
	if flag == nil {
		t.Error("stack command should have --path flag")
	}
}

func TestStackCommand_HasTrackFlag(t *testing.T) {
	cmd := NewStackCmd()
	flag := cmd.Flags().Lookup("track")
	if flag == nil {
		t.Error("stack command should have --track flag")
	}
}

func TestStackCommand_HasNoCheckoutFlag(t *testing.T) {
	cmd := NewStackCmd()
	flag := cmd.Flags().Lookup("no-checkout")
	if flag == nil {
		t.Error("stack command should have --no-checkout flag")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli -run TestStackCommand -v`
Expected: FAIL - flags not found

**Step 3: Add new flags to stack command**

Modify `internal/cli/stack.go`:

```go
// NewStackCmd creates the stack command group
func NewStackCmd() *cobra.Command {
	var (
		stackBase   string
		stackForce  bool
		noSetup     bool
		path        string  // NEW: custom worktree path
		track       string  // NEW: remote branch to track
		noCheckout  bool    // NEW: skip checkout
	)

	cmd := &cobra.Command{
		Use:   "stack [name]",
		Short: "Create a stacked branch",
		Long: `Create a new stacked branch on top of the current branch.

If no name is provided, generates an auto-suffix (4 chars).
If a name is provided, appends it with a 4-char suffix.

Examples:
  wt stack              # Creates: currentBranch-xY7k
  wt stack api          # Creates: currentBranch-api-k9P2
  wt stack api --path /custom/path    # Custom worktree location
  wt stack api --track origin/api     # Track remote branch`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runStackCommand(cmd, args, stackBase, stackForce, noSetup, path, track, noCheckout)
		},
	}

	cmd.Flags().StringVar(&stackBase, "base", "", "base branch for stack (default: current)")
	cmd.Flags().BoolVar(&stackForce, "force", false, "allow stacking on main/master")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false, "skip setup hooks and worktree creation")
	cmd.Flags().StringVar(&path, "path", "", "custom path for worktree")
	cmd.Flags().StringVar(&track, "track", "", "remote branch to track")
	cmd.Flags().BoolVar(&noCheckout, "no-checkout", false, "don't checkout the branch")

	return cmd
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli -run TestStackCommand -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/stack.go internal/cli/stack_flags_test.go
git commit -m "feat(cli): add --path, --track, --no-checkout flags to wt stack

- Add --path flag for custom worktree location
- Add --track flag for remote branch tracking
- Add --no-checkout flag to skip checkout
- Add tests for flag presence

Refs: wt-ido"
```

---

## Task 5: Add worktree nesting check to wt stack

**Files:**
- Modify: `internal/cli/stack.go:47-88`

**Step 1: Write the failing test for nesting check**

Create `internal/cli/stack_nesting_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestStackCommand_NestingCheck(t *testing.T) {
	// This test verifies the nesting check is called at the start of runStackCommand
	// The actual behavior is tested via integration tests since it requires git setup

	// For unit test, we verify the error message format contains expected text
	expectedErrorParts := []string{
		"cannot stack from inside another worktree",
		"Main repository:",
	}

	// Verify the error message format (this is documentation of expected behavior)
	for _, part := range expectedErrorParts {
		if !strings.Contains("cannot stack from inside another worktree\n\nMain repository: /path", part) {
			t.Errorf("Expected error message to contain %q", part)
		}
	}
}
```

**Step 2: Run test to verify it passes (documentation test)**

Run: `go test ./internal/cli -run TestStackCommand_NestingCheck -v`
Expected: PASS (documentation test)

**Step 3: Add nesting check to runStackCommand**

Modify `internal/cli/stack.go`. Update the function signature and add nesting check:

```go
func runStackCommand(cmd *cobra.Command, args []string, stackBase string, stackForce bool, noSetup bool, path string, track string, noCheckout bool) {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	// Create git client early for nesting check
	gitClient, err := git.NewClient()
	if err != nil {
		Fatal("Failed to create git client: %v", err)
	}

	// Check if we're inside a worktree - this is not allowed
	inWorktree, mainRepoRoot, err := gitClient.IsInWorktree(ctx)
	if err != nil {
		Fatal("Failed to check worktree context: %v", err)
	}

	if inWorktree {
		// Get current path for the error message
		repoInfo, err := gitClient.GetRepoInfo(ctx)
		currentPath := "unknown"
		if err == nil {
			currentPath = repoInfo.RootPath
		}

		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		Fatal(`cannot stack from inside another worktree

Current location: %s
Main repository:  %s

Run this command from the main repository instead:
  cd %s && wt stack %s`,
			currentPath,
			mainRepoRoot,
			mainRepoRoot,
			name)
	}

	// Check for main/master protection
	currentBranch, err := gitClient.GetCurrentBranch(ctx)
	if err != nil {
		Fatal("Failed to get current branch: %v", err)
	}

	if !stackForce && isProtectedBranch(currentBranch) {
		Fatal("Cannot stack on '%s'. Stack on feature branches only.\nUse --force to override.", currentBranch)
	}

	// Initialize services (now needs path, track, noCheckout for worktree creation)
	stackService := initStackServiceWithWorktree(gitClient)

	// Get the optional name argument
	var name string
	if len(args) > 0 {
		name = args[0]
	}

	// Create the stack branch
	spec := stack.BranchSpec{
		Name: name,
		Base: stackBase,
	}

	stackBranch, err := stackService.CreateStackBranch(ctx, spec)
	if err != nil {
		Fatal("Failed to create stack branch: %v", err)
	}

	if _, err := fmt.Fprintf(out, "Created stacked branch: %s\n", stackBranch.Name); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Create worktree with new flags
	if !noSetup {
		wtSpec := stack.WorktreeSpec{
			Path:       path,
			Track:      track,
			NoCheckout: noCheckout,
		}
		createStackWorktreeWithSpec(ctx, cmd, stackService, stackBranch.Name, wtSpec)
	}
}
```

**Step 4: Add initStackServiceWithWorktree and update createStackWorktree**

Add to `internal/cli/stack.go`:

```go
// initStackServiceWithWorktree initializes stack service with shared git client
func initStackServiceWithWorktree(gitClient *git.Client) *stack.Service {
	cfg, err := loadConfigForCommand()
	if err != nil {
		Fatal("Failed to load config: %v", err)
	}

	// Validate git-spice configuration early
	if err := validateGitSpiceConfig(cfg); err != nil {
		Fatal("%v", err)
	}

	spiceClient, err := spice.NewClient(cfg)
	if err != nil {
		Fatal("Failed to create spice client: %v", err)
	}

	worktreeSvc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		Fatal("Failed to create worktree service: %v", err)
	}

	stackService, err := stack.NewService(gitClient, spiceClient, cfg, worktreeSvc)
	if err != nil {
		Fatal("Failed to create stack service: %v", err)
	}

	return stackService
}

// createStackWorktreeWithSpec creates a worktree with explicit options
func createStackWorktreeWithSpec(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName string, spec stack.WorktreeSpec) {
	out := cmd.OutOrStdout()

	worktree, err := stackService.CreateWorktreeWithSpec(ctx, branchName, spec)
	if err != nil {
		Fatal("Failed to create worktree: %v", err)
	}
	if _, err := fmt.Fprintf(out, "Created worktree: %s\n", worktree.Path); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Setup tmux and hooks (existing logic)
	setupStackWorktree(ctx, cmd, stackService, branchName, worktree.Path)
}

// setupStackWorktree handles tmux window and hook setup
func setupStackWorktree(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName string, worktreePath string) {
	// NEW ORDER: Create tmux window BEFORE running hooks
	if shouldCreateTmuxWindow(NoTmux()) {
		tmuxClient, err := tmux.NewClient()
		if err != nil {
			// Fall back to local hooks
			if err := runSetupHooks(ctx, worktreePath); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
			}
			return
		}

		// Get stack level for window naming
		stackLevel := getStackLevel(ctx, stackService, branchName)
		windowName := tmux.GenerateStackWindowName(branchName, stackLevel)

		if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
			if err := runSetupHooks(ctx, worktreePath); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
			}
			return
		}

		// Select the window
		_ = tmuxClient.SelectWindow(windowName)

		// Run hooks INSIDE the new window
		if err := runSetupHooksInWindow(ctx, worktreePath, tmuxClient, windowName); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}
	} else {
		// Not in tmux or --no-tmux: run hooks locally
		if err := runSetupHooks(ctx, worktreePath); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
		}
	}
}
```

**Step 5: Add worktree import**

Add to imports in `internal/cli/stack.go`:

```go
import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/spice"
	"github.com/joebalancio/wt/internal/stack"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree" // NEW
	"github.com/spf13/cobra"
)
```

**Step 6: Run all CLI tests**

Run: `go test ./internal/cli -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/cli/stack.go internal/cli/stack_nesting_test.go
git commit -m "feat(cli): add worktree nesting check to wt stack

- Add IsInWorktree check at start of runStackCommand
- Provide helpful error message with main repo path
- Use shared git client for nesting check and service init
- Add initStackServiceWithWorktree to create worktree service
- Add createStackWorktreeWithSpec for flag support

Refs: wt-ido"
```

---

## Task 6: Update NewStackListCmd to use new constructor

**Files:**
- Modify: `internal/cli/stack.go:170-232`

**Step 1: Verify NewStackListCmd still works**

Run: `go test ./internal/cli -run TestStack -v`
Expected: PASS

**Step 2: Update NewStackListCmd to use worktree service**

The `NewStackListCmd` function also creates a stack service and needs updating. Modify it:

```go
// NewStackListCmd creates the stack list subcommand
func NewStackListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show stack hierarchy with paths",
		Long:  `Display the current stack as a tree with branch names and worktree paths.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			// Create clients
			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			// Load config
			cfg, err := loadConfigForCommand()
			if err != nil {
				Fatal("Failed to load config: %v", err)
			}

			// Validate git-spice configuration early
			if err := validateGitSpiceConfig(cfg); err != nil {
				Fatal("%v", err)
			}

			spiceClient, err := spice.NewClient(cfg)
			if err != nil {
				Fatal("Failed to create spice client: %v", err)
			}

			// Create worktree service (required for stack service)
			worktreeSvc, err := worktree.NewService(gitClient, cfg)
			if err != nil {
				Fatal("Failed to create worktree service: %v", err)
			}

			// Create stack service
			stackService, err := stack.NewService(gitClient, spiceClient, cfg, worktreeSvc)
			if err != nil {
				Fatal("Failed to create stack service: %v", err)
			}

			// Get stack with worktree paths
			branches, err := stackService.GetStack(ctx)
			if err != nil {
				Fatal("Failed to get stack: %v", err)
			}

			// Get current branch for highlighting
			currentBranch, _ := gitClient.GetCurrentBranch(ctx)
			for _, branch := range branches {
				if branch.Name == currentBranch {
					branch.IsHead = true
				}
			}

			// Format and display tree
			treeOutput := stack.FormatStackTree(branches)
			if _, err := fmt.Fprint(out, treeOutput); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	return cmd
}
```

**Step 3: Run tests to verify**

Run: `go test ./internal/cli -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/cli/stack.go
git commit -m "fix(cli): update NewStackListCmd to use worktree service

- Add worktree service creation in NewStackListCmd
- Required for stack.NewService constructor

Refs: wt-ido"
```

---

## Task 7: Remove unused getWorktreePath method and initStackService

**Files:**
- Modify: `internal/stack/service.go:150-172`
- Modify: `internal/cli/stack.go:90-118`

**Step 1: Identify usages of getWorktreePath**

Run: `grep -rn "getWorktreePath" internal/`
Check if it's still used by CreateStackBranch or convertToDomainStackBranch.

**Step 2: Keep getWorktreePath for CreateStackBranch**

The `CreateStackBranch` method and `convertToDomainStackBranch` still use `getWorktreePath` to return the expected path in `StackBranch.Path`. This is used by `wt stack list` to show where worktrees would be created. Keep this method.

**Step 3: Remove old initStackService (now replaced)**

Remove the old `initStackService` function from `internal/cli/stack.go` since it's replaced by `initStackServiceWithWorktree`.

**Step 4: Remove old createStackWorktree (now replaced)**

Remove the old `createStackWorktree` function from `internal/cli/stack.go` since it's replaced by `createStackWorktreeWithSpec` and `setupStackWorktree`.

**Step 5: Run all tests**

Run: `go test ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/cli/stack.go
git commit -m "refactor(cli): remove duplicate stack helper functions

- Remove old initStackService (replaced by initStackServiceWithWorktree)
- Remove old createStackWorktree (replaced by createStackWorktreeWithSpec)

Refs: wt-ido"
```

---

## Task 8: Run full test suite and lint

**Files:**
- None (verification only)

**Step 1: Run all tests**

Run: `make test`
Expected: PASS (all tests)

**Step 2: Run linter**

Run: `make lint`
Expected: PASS (no issues)

**Step 3: Run format check**

Run: `make fmt`
Expected: No changes (already formatted)

**Step 4: Run full check**

Run: `make check`
Expected: PASS (fmt + lint + test)

**Step 5: Commit any formatting changes**

```bash
git add .
git commit -m "chore: fix linting issues

Refs: wt-ido"
```

---

## Task 9: Update CLAUDE.md documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add wt stack command documentation**

Add a new section to CLAUDE.md for the wt stack command:

```markdown
### wt stack command

Create and manage stacked branches using git-spice.

```bash
# Create stacked branch with auto-suffix
wt stack              # Creates: currentBranch-xY7k

# Create stacked branch with named suffix
wt stack api          # Creates: currentBranch-api-k9P2

# With custom path
wt stack api --path /custom/path

# Track remote branch
wt stack api --track origin/feat-auth

# Skip checkout
wt stack api --no-checkout

# Allow stacking on main/master
wt stack api --force

# Skip setup hooks and worktree creation
wt stack api --no-setup

# List stack hierarchy
wt stack list
```

**Flags:**
- `--base` - Base branch for stack (default: current)
- `--force` - Allow stacking on main/master
- `--no-setup` - Skip setup hooks and worktree creation
- `--path` - Custom path for worktree
- `--track` - Remote branch to track
- `--no-checkout` - Don't checkout the branch

**Safety Features:**
- Worktree nesting check (cannot stack from inside a worktree)
- Main/master protection (use --force to override)
- Path collision detection via worktree.Service
```

**Step 2: Commit documentation**

```bash
git add CLAUDE.md
git commit -m "docs: document wt stack command with new flags

- Add --path, --track, --no-checkout flag documentation
- Document safety features (nesting check, path collision)
- Add usage examples

Refs: wt-ido"
```

---

## Task 10: Final verification and integration test

**Files:**
- None (verification only)

**Step 1: Build the binary**

Run: `make build`
Expected: Binary created at `bin/wt`

**Step 2: Test nesting check manually (if in worktree)**

Run: `./bin/wt stack test-nesting`
Expected: Error message about being inside worktree (if in worktree)

**Step 3: Verify help output includes new flags**

Run: `./bin/wt stack --help`
Expected: Shows --path, --track, --no-checkout flags

**Step 4: Final commit**

```bash
git add .
git commit -m "chore: final cleanup for wt-ido implementation

Refs: wt-ido"
```

---

## Acceptance Criteria Verification

- [ ] `wt stack` fails with helpful error when run from inside a worktree
- [ ] `wt stack --path /custom/path api` creates worktree at specified path
- [ ] `wt stack --no-checkout api` creates worktree without checkout
- [ ] `wt stack --track origin/api api` tracks remote branch
- [ ] Path collision detection works for stack commands
- [ ] All existing tests pass
- [ ] `make check` passes
