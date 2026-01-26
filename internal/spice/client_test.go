package spice

import (
	"context"
	"os/exec"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestNewClient_RequiresConfig(t *testing.T) {
	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: "",
		},
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Error("expected error when BinaryPath is empty")
	}
}

func TestNewClient_ValidatesPath(t *testing.T) {
	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: "/nonexistent/path",
		},
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestNewClient_NilConfig(t *testing.T) {
	_, err := NewClient(nil)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestClient_GetVersion(t *testing.T) {
	// Skip if git-spice not available
	path, err := exec.LookPath("git-spice")
	if err != nil {
		t.Skipf("git-spice not available: %v", err)
	}

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: path,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
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
	// Skip if git-spice not available
	path, err := exec.LookPath("git-spice")
	if err != nil {
		t.Skipf("git-spice not available: %v", err)
	}

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: path,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// TODO: Add integration test for branch creation
	// This requires a git repo with git-spice initialized
	_ = client
}

func TestVerifyGitSpice_ValidBinary(t *testing.T) {
	// Skip if git-spice not available
	path, err := exec.LookPath("git-spice")
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
