package spice

import (
	"context"
	"os/exec"
	"path/filepath"
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

func TestVerifyGitSpice_ValidBinary(t *testing.T) {
	// Skip if git-spice not available
	path, err := findGitSpice()
	if err != nil {
		t.Skip("git-spice not available")
	}

	if err := verifyGitSpice(path); err != nil {
		t.Errorf("verifyGitSpice failed: %v", err)
	}
}

func TestVerifyGitSpice_InvalidBinary(t *testing.T) {
	err := verifyGitSpice("/bin/ls")
	if err == nil {
		t.Error("expected error for /bin/ls, got nil")
	}
}

func TestDetectGitSpice_PrefersGitSpiceCommand(t *testing.T) {
	// This test verifies the precedence: git-spice > gs
	path, err := detectGitSpice()
	if err != nil {
		t.Skip("git-spice not available")
	}

	// Should not be just "gs" if "git-spice" exists
	if filepath.Base(path) == "gs" {
		// Check if git-spice also exists
		if _, err := exec.LookPath("git-spice"); err == nil {
			t.Error("prefer 'git-spice' over 'gs', but got 'gs'")
		}
	}
}
