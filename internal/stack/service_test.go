package stack

import (
	"context"
	"testing"

	"github.com/user/wt/internal/spice"
)

func TestNewService(t *testing.T) {
	service, err := NewService(nil)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_GenerateBranchSuffix(t *testing.T) {
	service, _ := NewService(nil)

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
	service, _ := NewService(nil)

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
	// Create a mock spice client
	mockClient := &MockSpiceClient{
		createFunc: func(_ context.Context, spec spice.BranchCreateSpec) (*spice.Branch, error) {
			return &spice.Branch{Name: spec.Name}, nil
		},
	}

	service, err := NewService(mockClient)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	branch, err := service.CreateStackBranch(context.Background(), "feat/auth", "")
	if err != nil {
		t.Fatalf("CreateStackBranch() error = %v", err)
	}
	if branch == nil {
		t.Fatal("CreateStackBranch() returned nil branch")
	}
	if branch.Name == "" {
		t.Error("branch name should not be empty")
	}
}

func TestService_GetStack(t *testing.T) {
	mockBranches := []*spice.Branch{
		{Name: "main", IsRoot: true},
		{Name: "feat/auth", IsRoot: false},
	}

	mockClient := &MockSpiceClient{
		stackFunc: func(_ context.Context) ([]*spice.Branch, error) {
			return mockBranches, nil
		},
	}

	service, err := NewService(mockClient)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	stack, err := service.GetStack(context.Background())
	if err != nil {
		t.Fatalf("GetStack() error = %v", err)
	}
	if len(stack) != 2 {
		t.Errorf("got %d branches, want 2", len(stack))
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
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
