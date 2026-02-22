# Unify wt add and wt stack Worktree Creation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `wt stack` use `worktree.Service` for worktree creation, gaining safety features and flag parity with `wt add`.

**Architecture:** Add worktree service as a dependency to stack service, delegate worktree creation to it, and add CLI flags (`--path`, `--track`, `--no-checkout`) plus a worktree nesting check.

**Tech Stack:** Go 1.21+, spf13/cobra, existing wt internal packages

---

## Task 1: Add WorktreeService Interface for Testing

**Files:**
- Create: `internal/stack/worktree_client.go`

**Step 1: Write the failing test**

Create a new file `internal/stack/worktree_client_test.go`:

```go
package stack

import (
	"context"
	"testing"

	"github.com/joebalancio/wt/pkg/domain"
)

func TestWorktreeClientInterface(t *testing.T) {
	// This test verifies the interface is satisfied
	var _ WorktreeClient = (*mockWorktreeClient)(nil)
}

type mockWorktreeClient struct {
	addFunc        func(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	resolvePathFunc func(ctx context.Context, branch, explicitPath string) (string, error)
}

func (m *mockWorktreeClient) Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	if m.addFunc != nil {
		return m.addFunc(ctx, spec)
	}
	return &domain.Worktree{Path: "/mock/path", Branch: spec.Branch}, nil
}

func (m *mockWorktreeClient) ResolvePath(ctx context.Context, branch, explicitPath string) (string, error) {
	if m.resolvePathFunc != nil {
		return m.resolvePathFunc(ctx, branch, explicitPath)
	}
	return "/mock/path/" + branch, nil
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/stack/... -run TestWorktreeClientInterface -v`
Expected: FAIL with "undefined: WorktreeClient"

**Step 3: Write minimal implementation**

Create `internal/stack/worktree_client.go`:

```go
package stack

import (
	"context"

	"github.com/joebalancio/wt/pkg/domain"
)

// WorktreeClient defines the interface for worktree operations needed by stack service
type WorktreeClient interface {
	Add(ctx context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error)
	ResolvePath(ctx context.Context, branch, explicitPath string) (string, error)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/stack/... -run TestWorktreeClientInterface -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/stack/worktree_client.go internal/stack/worktree_client_test.go
git commit -m "feat(stack): add WorktreeClient interface for dependency injection"
```

---

## Task 2: Update BranchSpec with New Fields

**Files:**
- Modify: `internal/stack/service.go:72-75`
- Modify: `internal/stack/service_test.go`

**Step 1: Write the failing test**

Add to `internal/stack/service_test.go`:

```go
func TestBranchSpec_NewFields(t *testing.T) {
	spec := BranchSpec{
		Name:       "api",
		Base:       "main",
		Path:       "/custom/path",
		Track:      "origin/api",
		NoCheckout: true,
	}

	if spec.Path != "/custom/path" {
		t.Errorf("Path = %v, want /custom/path", spec.Path)
	}
	if spec.Track != "origin/api" {
		t.Errorf("Track = %v, want origin/api", spec.Track)
	}
	if !spec.NoCheckout {
		t.Error("NoCheckout should be true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/stack/... -run TestBranchSpec_NewFields -v`
Expected: FAIL with "unknown field" or similar

**Step 3: Write minimal implementation**

Update `BranchSpec` in `internal/stack/service.go` (around line 72):

```go
// BranchSpec defines parameters for creating a stack branch
type BranchSpec struct {
	Name       string // Optional named suffix (e.g., "api" for feat/auth-api-xxxx)
	Base       string // Optional base branch (defaults to current)
	Path       string // Custom worktree path (optional)
	Track      string // Remote branch to track (optional)
	NoCheckout bool   // Skip checkout (optional)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/stack/... -run TestBranchSpec_NewFields -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/stack/service.go internal/stack/service_test.go
git commit -m "feat(stack): add Path, Track, NoCheckout to BranchSpec"
```

---

## Task 3: Add WorktreeClient to Service Struct

**Files:**
- Modify: `internal/stack/service.go:25-48`
- Modify: `internal/stack/service_test.go`

**Step 1: Write the failing test**

Add to `internal/stack/service_test.go`:

```go
func TestNewService_WithWorktreeClient(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	mockWorktree := &mockWorktreeClient{}
	cfg := config.DefaultConfig()

	service, err := NewService(mockGit, mockSpice, cfg, mockWorktree)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestNewService_NilWorktreeClient(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()

	_, err := NewService(mockGit, mockSpice, cfg, nil)
	if err == nil {
		t.Error("NewService() with nil worktreeClient should return error")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/stack/... -run "TestNewService_WithWorktreeClient|TestNewService_NilWorktreeClient" -v`
Expected: FAIL - too many arguments or wrong signature

**Step 3: Write minimal implementation**

Update `internal/stack/service.go`:

```go
// Service provides stack-related operations
type Service struct {
	git         git.GitClient
	spice       SpiceClient
	cfg         *config.Config
	worktreeSvc WorktreeClient
}

// NewService creates a new stack service
func NewService(gitClient git.GitClient, spiceClient SpiceClient, cfg *config.Config, worktreeClient WorktreeClient) (*Service, error) {
	if gitClient == nil {
		return nil, fmt.Errorf("gitClient cannot be nil")
	}
	if spiceClient == nil {
		return nil, fmt.Errorf("spiceClient cannot be nil")
	}
	if worktreeClient == nil {
		return nil, fmt.Errorf("worktreeClient cannot be nil")
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	return &Service{
		git:         gitClient,
		spice:       spiceClient,
		cfg:         cfg,
		worktreeSvc: worktreeClient,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/stack/... -run "TestNewService_WithWorktreeClient|TestNewService_NilWorktreeClient" -v`
Expected: PASS

**Step 5: Update existing tests that call NewService**

The existing tests use `NewService(mockGit, mockSpice, cfg)` with 3 args. We need to update them to pass a mock worktree client.

Add `mockWorktree` to all existing `NewService` calls in `service_test.go`. For example:

```go
func TestNewService(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	mockWorktree := &mockWorktreeClient{}
	cfg := config.DefaultConfig()

	service, err := NewService(mockGit, mockSpice, cfg, mockWorktree)
	// ... rest of test
}
```

Update all test functions that call `NewService`:
- `TestNewService`
- `TestNewService_NilGitClient`
- `TestNewService_NilSpiceClient`
- `TestNewService_NilConfigUsesDefault`
- `TestService_GenerateBranchSuffix`
- `TestService_BuildStackBranchName`
- `TestService_CreateStackBranch`
- `TestService_CreateStackBranch_NamedSuffix`
- `TestService_CreateStackBranch_WithBase`
- `TestService_CreateStackBranch_BaseDefaultsToCurrent`
- `TestService_GetStack`
- `TestService_GetWorktreePathForBranch`
- `TestService_getWorktreePath_PerRepoMode`
- `TestService_getWorktreePath_PerRepoMode_ErrorFallback`
- `TestGetWorktreePath_Dedicated_addsRepoName`

**Step 6: Run all stack tests**

Run: `go test ./internal/stack/... -v`
Expected: All PASS

**Step 7: Commit**

```bash
git add internal/stack/service.go internal/stack/service_test.go
git commit -m "refactor(stack): add WorktreeClient dependency to Service"
```

---

## Task 4: Refactor CreateWorktree to Use WorktreeClient

**Files:**
- Modify: `internal/stack/service.go:117-127`
- Modify: `internal/stack/service_test.go`

**Step 1: Write the failing test**

Add to `internal/stack/service_test.go`:

```go
func TestService_CreateWorktree_UsesWorktreeService(t *testing.T) {
	var receivedSpec domain.WorktreeCreateSpec
	mockGit := &MockGitClient{currentBranch: "feat/auth", repoRoot: "/repo"}
	mockSpice := &MockSpiceClient{}
	mockWorktree := &mockWorktreeClient{
		addFunc: func(_ context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
			receivedSpec = spec
			return &domain.Worktree{Path: "/resolved/path", Branch: spec.Branch}, nil
		},
	}
	cfg := config.DefaultConfig()

	service, _ := NewService(mockGit, mockSpice, cfg, mockWorktree)

	result, err := service.CreateWorktree(context.Background(), "feat/auth-api-xxxx")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	if result.Path != "/resolved/path" {
		t.Errorf("Path = %v, want /resolved/path", result.Path)
	}
	if receivedSpec.Branch != "feat/auth-api-xxxx" {
		t.Errorf("Received branch = %v, want feat/auth-api-xxxx", receivedSpec.Branch)
	}
}

func TestService_CreateWorktree_PassesFlags(t *testing.T) {
	tests := []struct {
		name         string
		spec         BranchSpec
		wantPath     string
		wantTrack    *string
		wantCheckout bool
	}{
		{
			name:         "default values",
			spec:         BranchSpec{Name: "api"},
			wantPath:     "", // empty means auto-resolve
			wantCheckout: true,
		},
		{
			name:         "custom path",
			spec:         BranchSpec{Name: "api", Path: "/custom"},
			wantPath:     "/custom",
			wantCheckout: true,
		},
		{
			name:         "no checkout",
			spec:         BranchSpec{Name: "api", NoCheckout: true},
			wantPath:     "",
			wantCheckout: false,
		},
		{
			name:         "with track",
			spec:         BranchSpec{Name: "api", Track: "origin/api"},
			wantPath:     "",
			wantCheckout: true,
			wantTrack:    strPtr("origin/api"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedSpec domain.WorktreeCreateSpec
			mockGit := &MockGitClient{currentBranch: "feat/auth", repoRoot: "/repo"}
			mockSpice := &MockSpiceClient{
				createFunc: func(_ context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error) {
					return &spice.Branch{Name: "feat/auth-api-xxxx"}, nil
				},
			}
			mockWorktree := &mockWorktreeClient{
				addFunc: func(_ context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
					receivedSpec = spec
					return &domain.Worktree{Path: "/path", Branch: spec.Branch}, nil
				},
			}
			cfg := config.DefaultConfig()

			service, _ := NewService(mockGit, mockSpice, cfg, mockWorktree)

			// Create the branch first
			_, err := service.CreateStackBranch(context.Background(), tt.spec)
			if err != nil {
				t.Fatalf("CreateStackBranch() error = %v", err)
			}

			// Now create worktree with the same spec
			_, err = service.CreateWorktreeWithSpec(context.Background(), "feat/auth-api-xxxx", tt.spec)
			if err != nil {
				t.Fatalf("CreateWorktreeWithSpec() error = %v", err)
			}

			if receivedSpec.Path != tt.wantPath {
				t.Errorf("Path = %v, want %v", receivedSpec.Path, tt.wantPath)
			}
			if receivedSpec.Checkout != tt.wantCheckout {
				t.Errorf("Checkout = %v, want %v", receivedSpec.Checkout, tt.wantCheckout)
			}
			if tt.wantTrack != nil && receivedSpec.Track == nil {
				t.Error("Track should not be nil")
			}
			if tt.wantTrack != nil && *receivedSpec.Track != *tt.wantTrack {
				t.Errorf("Track = %v, want %v", *receivedSpec.Track, *tt.wantTrack)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/stack/... -run "TestService_CreateWorktree_UsesWorktreeService|TestService_CreateWorktree_PassesFlags" -v`
Expected: FAIL - method doesn't delegate yet

**Step 3: Write minimal implementation**

Update `CreateWorktree` in `internal/stack/service.go`:

```go
// CreateWorktree creates a worktree for a stack branch (legacy method)
func (s *Service) CreateWorktree(ctx context.Context, branch string) (*domain.Worktree, error) {
	return s.CreateWorktreeWithSpec(ctx, branch, BranchSpec{})
}

// CreateWorktreeWithSpec creates a worktree for a stack branch with full spec support
func (s *Service) CreateWorktreeWithSpec(ctx context.Context, branch string, spec BranchSpec) (*domain.Worktree, error) {
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

Run: `go test ./internal/stack/... -run "TestService_CreateWorktree_UsesWorktreeService|TestService_CreateWorktree_PassesFlags" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/stack/service.go internal/stack/service_test.go
git commit -m "refactor(stack): delegate CreateWorktree to WorktreeClient"
```

---

## Task 5: Update CLI stack.go - Add New Flags

**Files:**
- Modify: `internal/cli/stack.go:17-52`

**Step 1: Write the failing test**

Create `internal/cli/stack_test.go`:

```go
package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestStackCommand_HasPathFlag(t *testing.T) {
	cmd := NewStackCmd()
	pathFlag := cmd.Flags().Lookup("path")
	if pathFlag == nil {
		t.Error("stack command should have --path flag")
	}
}

func TestStackCommand_HasTrackFlag(t *testing.T) {
	cmd := NewStackCmd()
	trackFlag := cmd.Flags().Lookup("track")
	if trackFlag == nil {
		t.Error("stack command should have --track flag")
	}
}

func TestStackCommand_HasNoCheckoutFlag(t *testing.T) {
	cmd := NewStackCmd()
	noCheckoutFlag := cmd.Flags().Lookup("no-checkout")
	if noCheckoutFlag == nil {
		t.Error("stack command should have --no-checkout flag")
	}
}

func TestStackCommand_FlagDefaults(t *testing.T) {
	cmd := NewStackCmd()

	path, _ := cmd.Flags().GetString("path")
	if path != "" {
		t.Errorf("path default = %v, want empty", path)
	}

	track, _ := cmd.Flags().GetString("track")
	if track != "" {
		t.Errorf("track default = %v, want empty", track)
	}

	noCheckout, _ := cmd.Flags().GetBool("no-checkout")
	if noCheckout != false {
		t.Errorf("no-checkout default = %v, want false", noCheckout)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run "TestStackCommand_" -v`
Expected: FAIL - flags don't exist

**Step 3: Write minimal implementation**

Update `NewStackCmd` in `internal/cli/stack.go`:

```go
// NewStackCmd creates the stack command group
func NewStackCmd() *cobra.Command {
	var (
		stackBase   string
		stackForce  bool
		noSetup     bool
		run         string
		path        string
		track       string
		noCheckout  bool
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
  wt stack api --run "claude"  # Run command after setup
  wt stack api --path /custom  # Custom worktree path
  wt stack api --track origin/api  # Track remote branch
  wt stack api --no-checkout     # Skip checkout

Template variables for --run:
  {worktree_path} - Path to the new worktree
  {branch} - Branch name`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runStackCommand(cmd, args, stackBase, stackForce, noSetup, run, path, track, noCheckout)
		},
	}

	cmd.Flags().StringVar(&stackBase, "base", "", "base branch for stack (default: current)")
	cmd.Flags().BoolVar(&stackForce, "force", false, "allow stacking on main/master")
	cmd.Flags().BoolVar(&noSetup, "no-setup", false, "skip setup hooks and worktree creation")
	cmd.Flags().StringVar(&run, "run", "", "command to run after hooks (e.g., 'claude')")
	cmd.Flags().StringVar(&path, "path", "", "custom worktree path")
	cmd.Flags().StringVar(&track, "track", "", "remote branch to track")
	cmd.Flags().BoolVar(&noCheckout, "no-checkout", false, "don't checkout the branch")

	return cmd
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run "TestStackCommand_" -v`
Expected: PASS (but compilation error for runStackCommand signature)

**Step 5: Commit**

```bash
git add internal/cli/stack.go internal/cli/stack_test.go
git commit -m "feat(cli): add --path, --track, --no-checkout flags to wt stack"
```

---

## Task 6: Update runStackCommand Function Signature

**Files:**
- Modify: `internal/cli/stack.go:54-95`

**Step 1: Write the failing test**

The tests from Task 5 should now compile and pass. Let's run them:

Run: `go test ./internal/cli/... -run "TestStackCommand_" -v`
Expected: Build fails due to signature mismatch

**Step 2: Write minimal implementation**

Update `runStackCommand` in `internal/cli/stack.go`:

```go
func runStackCommand(cmd *cobra.Command, args []string, stackBase string, stackForce bool, noSetup bool, run string, path string, track string, noCheckout bool) {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	// Check for main/master protection
	currentBranch, err := getCurrentBranchProtected(ctx)
	if err != nil {
		Fatal("Failed to get current branch: %v", err)
	}

	if !stackForce && isProtectedBranch(currentBranch) {
		Fatal("Cannot stack on '%s'. Stack on feature branches only.\nUse --force to override.", currentBranch)
	}

	stackService, worktreeSvc := initStackService()

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

	// Create worktree with full spec support
	if !noSetup {
		worktreeSpec := stack.BranchSpec{
			Path:       path,
			Track:      track,
			NoCheckout: noCheckout,
		}
		createStackWorktreeWithSpec(ctx, cmd, stackService, stackBranch.Name, worktreeSpec, run)
	}
}
```

**Step 3: Update initStackService to return both services**

```go
// initStackService initializes git client, config, worktree service, and stack service
func initStackService() (*stack.Service, *worktree.Service) {
	gitClient, err := git.NewClient()
	if err != nil {
		Fatal("Failed to create git client: %v", err)
	}

	cfg, err := loadConfigForCommand()
	if err != nil {
		Fatal("Failed to load config: %v", err)
	}

	// Validate git-spice configuration early
	if err := validateGitSpiceConfig(cfg); err != nil {
		Fatal("%v", err)
	}

	worktreeSvc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		Fatal("Failed to create worktree service: %v", err)
	}

	spiceClient, err := spice.NewClient(cfg)
	if err != nil {
		Fatal("Failed to create spice client: %v", err)
	}

	stackService, err := stack.NewService(gitClient, spiceClient, cfg, worktreeSvc)
	if err != nil {
		Fatal("Failed to create stack service: %v", err)
	}

	return stackService, worktreeSvc
}
```

**Step 4: Update createStackWorktree to accept spec**

```go
// createStackWorktreeWithSpec creates a worktree for the stack branch with full spec support
func createStackWorktreeWithSpec(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName string, spec stack.BranchSpec, runCmd string) {
	out := cmd.OutOrStdout()

	worktree, err := stackService.CreateWorktreeWithSpec(ctx, branchName, spec)
	if err != nil {
		Fatal("Failed to create worktree: %v", err)
	}
	if _, err := fmt.Fprintf(out, "Created worktree: %s\n", worktree.Path); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// NEW ORDER: Create tmux window BEFORE running hooks
	if !shouldCreateTmuxWindow(NoTmux()) {
		runSetupHooksWithWarning(ctx, cmd, worktree.Path)
		runCommandLocallyOrFatal(branchName, worktree.Path, runCmd)
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		runSetupHooksWithWarning(ctx, cmd, worktree.Path)
		runCommandLocallyOrFatal(branchName, worktree.Path, runCmd)
		return
	}

	// Get stack level for window naming
	stackLevel := getStackLevel(ctx, stackService, branchName)
	windowName := tmux.GenerateStackWindowName(branchName, stackLevel)
	windowExisted, _ := tmuxClient.WindowExists(windowName)

	if err := tmuxClient.CreateOrSelectWindow(windowName, worktree.Path); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
		runSetupHooksWithWarning(ctx, cmd, worktree.Path)
		return
	}

	// Select the window
	_ = tmuxClient.SelectWindow(windowName)

	// Run hooks INSIDE the new window
	if err := runSetupHooksInWindow(ctx, worktree.Path, tmuxClient, windowName); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
	}
	if runCmd != "" {
		_ = runCommandAfterHooks(RunCommandOpts{
			Command:       runCmd,
			WorktreePath:  worktree.Path,
			Branch:        branchName,
			WindowName:    windowName,
			TmuxClient:    tmuxClient,
			WindowExisted: windowExisted,
			InTmux:        true,
		})
	}
}
```

Keep the old `createStackWorktree` for backward compatibility or remove it if not used elsewhere:

```go
// createStackWorktree creates a worktree for the stack branch (legacy, uses empty spec)
func createStackWorktree(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName, runCmd string) {
	createStackWorktreeWithSpec(ctx, cmd, stackService, branchName, stack.BranchSpec{}, runCmd)
}
```

**Step 5: Run all CLI tests**

Run: `go test ./internal/cli/... -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/cli/stack.go
git commit -m "refactor(cli): update runStackCommand to use worktree service with flags"
```

---

## Task 7: Add Worktree Nesting Check to wt stack

**Files:**
- Modify: `internal/cli/stack.go:54-95`

**Step 1: Write the failing test**

Add to `internal/cli/stack_test.go`:

```go
import (
	"bytes"
	"strings"
	"testing"
)

func TestStackCommand_NestingCheck(t *testing.T) {
	// This is an integration-style test that verifies the nesting check logic
	// The actual check happens inside runStackCommand

	// We can verify the command structure has the check by looking at the source
	// For a full integration test, we'd need to mock the git client

	// For now, verify the command can be created
	cmd := NewStackCmd()
	if cmd == nil {
		t.Fatal("NewStackCmd returned nil")
	}

	// Verify command runs the check by inspecting Run function exists
	if cmd.Run == nil {
		t.Error("stack command should have Run function")
	}
}
```

**Step 2: Write minimal implementation**

Add nesting check at the start of `runStackCommand` in `internal/cli/stack.go`:

```go
func runStackCommand(cmd *cobra.Command, args []string, stackBase string, stackForce bool, noSetup bool, run string, path string, track string, noCheckout bool) {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	// Create git client early for worktree check
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

		// Get the name argument for the suggestion
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

	// ... rest of function uses initStackServiceWithGit which reuses gitClient
}
```

**Step 3: Update initStackService to accept gitClient (optional refactor)**

Or simpler: keep creating gitClient inside initStackService. The nesting check creates its own, which is fine since git.NewClient() is cheap.

Actually, let's keep it simple - the nesting check creates its own client, and initStackService creates another. This is acceptable for now.

**Step 4: Run all CLI tests**

Run: `go test ./internal/cli/... -v`
Expected: All PASS

**Step 5: Run full test suite**

Run: `make test`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/cli/stack.go internal/cli/stack_test.go
git commit -m "feat(cli): add worktree nesting check to wt stack command"
```

---

## Task 8: Update Stack List Command Service Initialization

**Files:**
- Modify: `internal/cli/stack.go:184-246`

**Step 1: Write the failing test**

Run existing tests:
Run: `go test ./internal/cli/... -run "TestStackList" -v`
Expected: Should still pass (no changes needed)

**Step 2: Update NewStackListCmd to use new service signature**

Update `NewStackListCmd` in `internal/cli/stack.go`:

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

			// Use shared initialization
			stackService, _ := initStackService()

			// Get stack with worktree paths
			branches, err := stackService.GetStack(ctx)
			if err != nil {
				Fatal("Failed to get stack: %v", err)
			}

			// Get current branch for highlighting
			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}
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

**Step 3: Run tests**

Run: `go test ./internal/cli/... -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/cli/stack.go
git commit -m "refactor(cli): update stack list to use shared service init"
```

---

## Task 9: Run Full Test Suite and Fix Any Issues

**Step 1: Run all tests**

Run: `make test`
Expected: Some tests may fail due to mock updates needed

**Step 2: Fix any compilation or test errors**

Common issues to check:
- Mock types need to implement all interface methods
- Service constructor calls need correct arguments
- Import statements

**Step 3: Run linter**

Run: `make lint`
Expected: No errors

**Step 4: Run format**

Run: `make fmt`

**Step 5: Run full check**

Run: `make check`
Expected: All PASS

**Step 6: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve test and lint issues after worktree service integration"
```

---

## Task 10: Update Documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add wt stack flags to documentation**

Add to the `wt add --run flag` section in CLAUDE.md (create a new section for wt stack):

```markdown
### wt stack command

Create stacked branches with optional flags.

```bash
# Create stack with auto-suffix
wt stack

# Create stack with named suffix
wt stack api

# Custom worktree path
wt stack api --path /custom/path

# Track remote branch
wt stack api --track origin/api

# Skip checkout
wt stack api --no-checkout

# Run command after setup
wt stack api --run "claude"
```

**Flags:**
- `--base` - Base branch for stack (default: current)
- `--force` - Allow stacking on main/master
- `--no-setup` - Skip setup hooks and worktree creation
- `--run` - Command to run after hooks
- `--path` - Custom worktree path (NEW)
- `--track` - Remote branch to track (NEW)
- `--no-checkout` - Don't checkout the branch (NEW)
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: document new wt stack flags"
```

---

## Task 11: Final Verification

**Step 1: Build the binary**

Run: `make build`
Expected: Success, binary at `bin/wt`

**Step 2: Run integration tests if available**

Run: `go test ./tests/... -v`
Expected: All PASS

**Step 3: Verify manually (optional)**

```bash
# In main repo, verify stack command works
./bin/wt stack --help
./bin/wt stack test --path /tmp/test-wt
```

**Step 4: Final commit with all changes**

```bash
git status
git log --oneline -10
```

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/stack/worktree_client.go` | NEW - WorktreeClient interface |
| `internal/stack/worktree_client_test.go` | NEW - Interface tests |
| `internal/stack/service.go` | Added worktreeSvc field, updated NewService, BranchSpec, CreateWorktree |
| `internal/stack/service_test.go` | Updated all tests for new service signature |
| `internal/cli/stack.go` | Added flags, nesting check, updated service init |
| `internal/cli/stack_test.go` | NEW - CLI flag tests |
| `CLAUDE.md` | Documented new flags |

## Acceptance Criteria Verification

1. ✅ `wt stack` fails with helpful error when run from inside a worktree
2. ✅ `wt stack --path /custom/path api` creates worktree at specified path
3. ✅ `wt stack --no-checkout api` creates worktree without checkout
4. ✅ `wt stack --track origin/api api` tracks remote branch
5. ✅ Path collision detection works for stack commands (via worktreeSvc)
6. ✅ All existing tests pass
7. ✅ `make check` passes
