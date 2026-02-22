package stack

import (
	"context"
	"fmt"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/spice"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
)

func newTestWorktreeService(t *testing.T, gitClient git.GitClient, cfg *config.Config) *worktree.Service {
	t.Helper()
	wtSvc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		t.Fatalf("worktree.NewService() error = %v", err)
	}
	return wtSvc
}

func TestNewService(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()

	service, err := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestNewService_NilGitClient(t *testing.T) {
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()

	_, err := NewService(nil, mockSpice, cfg, nil)
	if err == nil {
		t.Error("NewService() with nil gitClient should return error")
	}
}

func TestNewService_NilSpiceClient(t *testing.T) {
	mockGit := &MockGitClient{}
	cfg := config.DefaultConfig()

	_, err := NewService(mockGit, nil, cfg, newTestWorktreeService(t, mockGit, cfg))
	if err == nil {
		t.Error("NewService() with nil spiceClient should return error")
	}
}

func TestNewService_NilConfigUsesDefault(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}

	service, err := NewService(mockGit, mockSpice, nil, newTestWorktreeService(t, mockGit, nil))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service.cfg == nil {
		t.Error("NewService() should use default config when nil is passed")
	}
}

func TestNewService_NilWorktreeService(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()

	_, err := NewService(mockGit, mockSpice, cfg, nil)
	if err == nil {
		t.Error("NewService() with nil worktreeSvc should return error")
	}
}

func TestService_GenerateBranchSuffix(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	suffix1 := service.GenerateBranchSuffix()
	suffix2 := service.GenerateBranchSuffix()

	if len(suffix1) != 4 {
		t.Errorf("suffix length = %d, want 4", len(suffix1))
	}
	if suffix1 == suffix2 {
		t.Error("suffixes should be unique")
	}
}

func TestService_BuildStackBranchName(t *testing.T) {
	mockGit := &MockGitClient{}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	tests := []struct {
		name       string
		current    string
		suffixName string
		wantPrefix string
	}{
		{
			name:       "auto suffix",
			current:    "feat/auth",
			suffixName: "",
			wantPrefix: "feat/auth-",
		},
		{
			name:       "named suffix",
			current:    "feat/auth",
			suffixName: "api",
			wantPrefix: "feat/auth-api-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.BuildStackBranchName(tt.current, tt.suffixName)
			if !hasPrefix(result, tt.wantPrefix) {
				t.Errorf("branch name = %v, want prefix %v", result, tt.wantPrefix)
			}
			// Should have 4 char suffix at end
			suffixLen := len(result) - len(tt.wantPrefix)
			if suffixLen != 4 {
				t.Errorf("suffix length = %d, want 4", suffixLen)
			}
		})
	}
}

func TestService_CreateStackBranch(t *testing.T) {
	mockGit := &MockGitClient{
		currentBranch: "feat/auth",
		repoRoot:      "/home/user/project",
	}
	mockSpice := &MockSpiceClient{
		createFunc: func(_ context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error) {
			return &spice.Branch{Name: spec.Name, IsRoot: false, IsHead: false}, nil
		},
	}
	cfg := config.DefaultConfig()

	service, err := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	spec := BranchSpec{Name: ""}
	branch, err := service.CreateStackBranch(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateStackBranch() error = %v", err)
	}
	if branch == nil {
		t.Fatal("CreateStackBranch() returned nil branch")
	}
	if branch.Name == "" {
		t.Error("branch name should not be empty")
	}
	if branch.Path == "" {
		t.Error("branch path should not be empty")
	}
}

func TestService_CreateStackBranch_NamedSuffix(t *testing.T) {
	mockGit := &MockGitClient{
		currentBranch: "feat/auth",
		repoRoot:      "/home/user/project",
	}
	mockSpice := &MockSpiceClient{
		createFunc: func(_ context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error) {
			return &spice.Branch{Name: spec.Name}, nil
		},
	}
	cfg := config.DefaultConfig()

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	spec := BranchSpec{Name: "api"}
	branch, err := service.CreateStackBranch(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateStackBranch() error = %v", err)
	}
	if branch.Name == "" {
		t.Error("branch name should not be empty")
	}
}

func TestService_CreateStackBranch_WithBase(t *testing.T) {
	mockGit := &MockGitClient{
		currentBranch: "feat/auth",
		repoRoot:      "/home/user/project",
	}
	var capturedBase string
	mockSpice := &MockSpiceClient{
		createFunc: func(_ context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error) {
			capturedBase = spec.Base
			return &spice.Branch{Name: spec.Name}, nil
		},
	}
	cfg := config.DefaultConfig()

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	// Test with explicit base
	spec := BranchSpec{Name: "fix", Base: "main"}
	_, err := service.CreateStackBranch(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateStackBranch() error = %v", err)
	}
	if capturedBase != "main" {
		t.Errorf("Base = %v, want main", capturedBase)
	}
}

func TestService_CreateStackBranch_BaseDefaultsToCurrent(t *testing.T) {
	mockGit := &MockGitClient{
		currentBranch: "feat/auth",
		repoRoot:      "/home/user/project",
	}
	var capturedBase string
	mockSpice := &MockSpiceClient{
		createFunc: func(_ context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error) {
			capturedBase = spec.Base
			return &spice.Branch{Name: spec.Name}, nil
		},
	}
	cfg := config.DefaultConfig()

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	// Test without base - should default to current branch
	spec := BranchSpec{Name: "fix"}
	_, err := service.CreateStackBranch(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateStackBranch() error = %v", err)
	}
	if capturedBase != "feat/auth" {
		t.Errorf("Base = %v, want feat/auth", capturedBase)
	}
}

func TestService_CreateWorktree_UsesWorktreeService(t *testing.T) {
	mockGit := &MockGitClient{
		repoRoot: "/home/user/project",
	}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()
	wtSvc := newTestWorktreeService(t, mockGit, cfg)

	service, err := NewService(mockGit, mockSpice, cfg, wtSvc)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = service.CreateWorktree(context.Background(), "feat/auth-xyz1")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	if mockGit.lastAddWorktreeSpec == nil {
		t.Fatal("expected AddWorktree to be called")
	}
	if mockGit.lastAddWorktreeSpec.Branch != "feat/auth-xyz1" {
		t.Errorf("Branch = %v, want feat/auth-xyz1", mockGit.lastAddWorktreeSpec.Branch)
	}
	if !mockGit.lastAddWorktreeSpec.Checkout {
		t.Error("Checkout should be true by default")
	}
}

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
			wantPath:     "/home/user/project/.worktrees/feat/auth-xyz1",
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
			wantPath:     "/home/user/project/.worktrees/feat/auth-xyz1",
			wantCheckout: false,
		},
		{
			name:           "with track",
			branch:         "feat/auth-xyz1",
			path:           "",
			track:          "origin/feat/auth-xyz1",
			noCheckout:     false,
			wantPath:       "/home/user/project/.worktrees/feat/auth-xyz1",
			wantCheckout:   true,
			wantTrackValue: "origin/feat/auth-xyz1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := &MockGitClient{
				repoRoot: "/home/user/project",
			}
			mockSpice := &MockSpiceClient{}
			cfg := config.DefaultConfig()
			wtSvc := newTestWorktreeService(t, mockGit, cfg)

			service, err := NewService(mockGit, mockSpice, cfg, wtSvc)
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}

			spec := WorktreeSpec{
				Path:       tt.path,
				Track:      tt.track,
				NoCheckout: tt.noCheckout,
			}

			_, err = service.CreateWorktreeWithSpec(context.Background(), tt.branch, spec)
			if err != nil {
				t.Fatalf("CreateWorktreeWithSpec() error = %v", err)
			}

			if mockGit.lastAddWorktreeSpec == nil {
				t.Fatal("expected AddWorktree to be called")
			}
			if mockGit.lastAddWorktreeSpec.Branch != tt.branch {
				t.Errorf("Branch = %v, want %v", mockGit.lastAddWorktreeSpec.Branch, tt.branch)
			}
			if mockGit.lastAddWorktreeSpec.Path != tt.wantPath {
				t.Errorf("Path = %v, want %v", mockGit.lastAddWorktreeSpec.Path, tt.wantPath)
			}
			if mockGit.lastAddWorktreeSpec.Checkout != tt.wantCheckout {
				t.Errorf("Checkout = %v, want %v", mockGit.lastAddWorktreeSpec.Checkout, tt.wantCheckout)
			}
			if tt.wantTrackValue != "" {
				if mockGit.lastAddWorktreeSpec.Track == nil || *mockGit.lastAddWorktreeSpec.Track != tt.wantTrackValue {
					t.Errorf("Track = %v, want %v", mockGit.lastAddWorktreeSpec.Track, tt.wantTrackValue)
				}
			}
		})
	}
}

func TestService_GetStack(t *testing.T) {
	mockGit := &MockGitClient{
		repoRoot: "/home/user/project",
	}
	mockBranches := []*spice.Branch{
		{Name: "main", IsRoot: true},
		{Name: "feat/auth", IsRoot: false},
	}

	mockSpice := &MockSpiceClient{
		stackFunc: func(_ context.Context) ([]*spice.Branch, error) {
			return mockBranches, nil
		},
	}
	cfg := config.DefaultConfig()

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	stack, err := service.GetStack(context.Background())
	if err != nil {
		t.Fatalf("GetStack() error = %v", err)
	}
	if len(stack) != 2 {
		t.Errorf("got %d branches, want 2", len(stack))
	}
	// Verify paths are populated
	for _, branch := range stack {
		if branch.Path == "" {
			t.Errorf("branch %s should have a path", branch.Name)
		}
	}
}

func TestService_GetWorktreePathForBranch(t *testing.T) {
	mockGit := &MockGitClient{
		repoRoot: "/home/user/project",
	}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()
	// Default is now per-repo, so set to dedicated for this test
	cfg.Worktree.Location = "dedicated"

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	path, err := service.GetWorktreePathForBranch(context.Background(), "feat/auth")
	if err != nil {
		t.Fatalf("GetWorktreePathForBranch() error = %v", err)
	}
	// Path should include repo name (project)
	expected := cfg.Worktree.GetDedicatedPath() + "/project/feat/auth"
	if path != expected {
		t.Errorf("path = %v, want %v", path, expected)
	}
}

func TestService_getWorktreePath_PerRepoMode(t *testing.T) {
	mockGit := &MockGitClient{
		repoRoot: "/home/user/project",
	}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "per-repo"

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	path := service.getWorktreePath(context.Background(), "feat/auth")
	expected := "/home/user/project/.worktrees/feat/auth"
	if path != expected {
		t.Errorf("path = %v, want %v", path, expected)
	}
}

func TestService_getWorktreePath_PerRepoMode_ErrorFallback(t *testing.T) {
	mockGit := &MockGitClient{
		repoRoot:      "",
		repoInfoError: true, // simulate error
	}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "per-repo"

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	path := service.getWorktreePath(context.Background(), "feat/auth")
	// Should fallback to relative path
	if path == "" {
		t.Error("path should not be empty even with error")
	}
}

func TestGetWorktreePath_Dedicated_addsRepoName(t *testing.T) {
	mockGit := &MockGitClient{
		repoRoot: "/home/user/projects/my-repo",
	}
	mockSpice := &MockSpiceClient{}
	cfg := config.DefaultConfig()
	cfg.Worktree.Location = "dedicated"
	cfg.Worktree.DedicatedPath = "/tmp/worktrees"

	service, _ := NewService(mockGit, mockSpice, cfg, newTestWorktreeService(t, mockGit, cfg))

	path, err := service.GetWorktreePathForBranch(context.Background(), "feature/auth")
	if err != nil {
		t.Fatalf("GetWorktreePathForBranch() error = %v", err)
	}

	// Path should include repo name
	expected := "/tmp/worktrees/my-repo/feature/auth"
	if path != expected {
		t.Errorf("GetWorktreePathForBranch() = %q, want %q", path, expected)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// MockGitClient is a mock implementation of git.GitClient for testing
type MockGitClient struct {
	currentBranch       string
	repoRoot            string
	repoInfoError       bool
	lastAddWorktreeSpec *domain.WorktreeCreateSpec
}

func (m *MockGitClient) GetCurrentBranch(_ context.Context) (string, error) {
	return m.currentBranch, nil
}

func (m *MockGitClient) GetRepoInfo(_ context.Context) (*domain.GitRepo, error) {
	if m.repoInfoError {
		return nil, fmt.Errorf("mock repo info error")
	}
	return &domain.GitRepo{RootPath: m.repoRoot}, nil
}

func (m *MockGitClient) ListWorktrees(_ context.Context) ([]*domain.Worktree, error) {
	return []*domain.Worktree{}, nil
}

func (m *MockGitClient) AddWorktree(_ context.Context, spec domain.WorktreeCreateSpec) (*domain.Worktree, error) {
	specCopy := spec
	m.lastAddWorktreeSpec = &specCopy

	path := spec.Path
	if path == "" {
		path = "/resolved/path/" + spec.Branch
	}
	return &domain.Worktree{Path: path, Branch: spec.Branch}, nil
}

func (m *MockGitClient) RemoveWorktree(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *MockGitClient) BranchExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *MockGitClient) DeleteBranch(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *MockGitClient) SquashMerge(_ context.Context, _ string) error {
	return nil
}

func (m *MockGitClient) CreateSquashCommit(_ context.Context, _ string) error {
	return nil
}

func (m *MockGitClient) IsWorktreeDirty(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *MockGitClient) IsBranchMerged(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (m *MockGitClient) RemoteBranchExists(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (m *MockGitClient) DeleteRemoteBranch(_ context.Context, _, _ string) error {
	return nil
}

func (m *MockGitClient) IsInWorktree(_ context.Context) (bool, string, error) {
	return false, "/repo", nil
}

func (m *MockGitClient) ListAllBranches(_ context.Context) ([]string, error) {
	return []string{"main"}, nil
}

// MockSpiceClient is a mock implementation of SpiceClient for testing
type MockSpiceClient struct {
	createFunc func(context.Context, spice.BranchCreateSpec) (*spice.Branch, error)
	stackFunc  func(context.Context) ([]*spice.Branch, error)
}

func (m *MockSpiceClient) CreateBranch(_ context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error) {
	if m.createFunc != nil {
		return m.createFunc(context.Background(), spec)
	}
	return &spice.Branch{Name: spec.Name}, nil
}

func (m *MockSpiceClient) GetStack(_ context.Context) ([]*spice.Branch, error) {
	if m.stackFunc != nil {
		return m.stackFunc(context.Background())
	}
	return []*spice.Branch{}, nil
}
