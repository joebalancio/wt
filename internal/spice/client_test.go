package spice

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

func TestClient_GetVersion(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skipf("git-spice not available: %v", err)
	}

	ctx := context.Background()
	version, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("GetVersion() error = %v", err)
	}
	if version == "" {
		t.Error("GetVersion() returned empty version")
	}
	t.Logf("git-spice version: %s", version)
}

func TestClient_CreateBranch(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Skipf("git-spice not available: %v", err)
	}

	// TODO: Add integration test for branch creation
	// This requires a git repo with git-spice initialized
	_ = client
}
