//go:build integration
// +build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/spice"
)

// skipIfNoGitSpice skips the test if git-spice is not available
// It checks for both 'git-spice' and 'gs' commands
func skipIfNoGitSpice(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("git-spice"); err == nil {
		return // git-spice found
	}
	if _, err := exec.LookPath("gs"); err == nil {
		// Verify gs is actually git-spice
		cmd := exec.Command("gs", "--version")
		output, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(output), "git-spice") {
			return // gs is git-spice
		}
	}
	t.Skip("git-spice not available in PATH")
}

// findGitSpiceBinary attempts to find the git-spice binary in PATH
// It checks for both 'git-spice' and 'gs' commands
func findGitSpiceBinary(t testing.TB) string {
	t.Helper()
	// Try git-spice first
	path, err := exec.LookPath("git-spice")
	if err == nil {
		return path
	}

	// Try gs command
	path, err = exec.LookPath("gs")
	if err == nil {
		// Verify it's actually git-spice
		cmd := exec.Command("gs", "--version")
		output, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(output), "git-spice") {
			return path
		}
	}

	return ""
}

// TestSpiceConfig_E2E tests the full end-to-end flow of git-spice configuration
func TestSpiceConfig_E2E(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Find the real git-spice binary
	gsPath := findGitSpiceBinary(t)
	if gsPath == "" {
		t.Skip("git-spice binary not found")
	}

	// Create a test config with Spice.BinaryPath set
	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: gsPath,
		},
	}

	// Create the spice client - should succeed
	client, err := spice.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create spice client with valid config: %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Verify the client works by calling GetVersion
	version, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("failed to get git-spice version: %v", err)
	}

	// Verify version contains "git-spice"
	if !strings.Contains(version, "git-spice") {
		t.Errorf("expected version to contain 'git-spice', got: %s", version)
	}

	// Verify version is not empty
	if strings.TrimSpace(version) == "" {
		t.Error("expected non-empty version string")
	}
}

// TestSpiceConfig_NotConfigured tests the error flow when git-spice is not configured
func TestSpiceConfig_NotConfigured(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a test config with empty Spice.BinaryPath
	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: "", // Empty means not configured
		},
	}

	// Try to create the spice client - should fail
	client, err := spice.NewClient(cfg)
	if err == nil {
		t.Error("expected error when creating spice client with empty BinaryPath, got nil")
	}

	if client != nil {
		t.Error("expected nil client when BinaryPath is empty")
	}

	// Verify error message mentions "wt init"
	if err != nil && !strings.Contains(err.Error(), "wt init") {
		t.Errorf("expected error message to mention 'wt init', got: %v", err)
	}
}

// TestSpiceConfig_InvalidPath tests the error flow when BinaryPath points to an invalid path
func TestSpiceConfig_InvalidPath(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a test config with an invalid BinaryPath
	invalidPath := "/nonexistent/path/to/git-spice"
	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: invalidPath,
		},
	}

	// Try to create the spice client - should fail
	client, err := spice.NewClient(cfg)
	if err == nil {
		t.Error("expected error when creating spice client with invalid BinaryPath, got nil")
	}

	if client != nil {
		t.Error("expected nil client when BinaryPath is invalid")
	}

	// Verify error message mentions path not found or binary not found
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error message to mention 'not found', got: %v", err)
	}
}

// TestSpiceConfig_NilConfig tests the error flow when config is nil
func TestSpiceConfig_NilConfig(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Try to create the spice client with nil config
	client, err := spice.NewClient(nil)
	if err == nil {
		t.Error("expected error when creating spice client with nil config, got nil")
	}

	if client != nil {
		t.Error("expected nil client when config is nil")
	}

	// Verify error message mentions config
	if err != nil && !strings.Contains(err.Error(), "config") {
		t.Errorf("expected error message to mention 'config', got: %v", err)
	}
}

// TestSpiceConfig_WrongBinary tests the error flow when BinaryPath points to a non-git-spice binary
func TestSpiceConfig_WrongBinary(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Find a binary that is definitely not git-spice
	// We'll use 'sh' which should exist on all systems but is not git-spice
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh binary not found for testing wrong binary scenario")
	}

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: shPath,
		},
	}

	// Try to create the spice client - should fail
	client, err := spice.NewClient(cfg)
	if err == nil {
		t.Error("expected error when creating spice client with wrong binary, got nil")
	}

	if client != nil {
		t.Error("expected nil client when BinaryPath points to wrong binary")
	}

	// Verify error message mentions it's not git-spice
	if err != nil && !strings.Contains(err.Error(), "not git-spice") {
		t.Errorf("expected error message to mention 'not git-spice', got: %v", err)
	}
}

// TestSpiceConfig_AbsolutePath tests using an absolute path for git-spice
func TestSpiceConfig_AbsolutePath(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Find the real git-spice binary (may be git-spice or gs)
	gsPath := findGitSpiceBinary(t)
	if gsPath == "" {
		t.Skip("git-spice binary not found")
	}

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: gsPath,
		},
	}

	// Create the spice client - should succeed with absolute path
	client, err := spice.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create spice client with absolute path: %v", err)
	}

	// Verify it works
	version, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("failed to get git-spice version: %v", err)
	}

	if !strings.Contains(version, "git-spice") {
		t.Errorf("expected version to contain 'git-spice', got: %s", version)
	}
}

// TestSpiceConfig_CommandInPath tests that git-spice command can be used directly
// It checks for both 'git-spice' and 'gs' commands
func TestSpiceConfig_CommandInPath(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Determine which command is available
	gsPath := findGitSpiceBinary(t)
	if gsPath == "" {
		t.Skip("git-spice binary not found")
	}

	// Test that the command is in PATH and executable
	// Use the basename of the found path (either 'git-spice' or 'gs')
	cmdName := filepath.Base(gsPath)
	cmd := exec.Command(cmdName, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version failed: %v\nOutput: %s", cmdName, err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "git-spice") {
		t.Errorf("expected git-spice version output to contain 'git-spice', got: %s", outputStr)
	}
}

// TestSpiceConfig_MultipleClients tests creating multiple clients with the same config
func TestSpiceConfig_MultipleClients(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	gsPath := findGitSpiceBinary(t)
	if gsPath == "" {
		t.Skip("git-spice binary not found")
	}

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: gsPath,
		},
	}

	// Create multiple clients
	client1, err := spice.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create first spice client: %v", err)
	}

	client2, err := spice.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create second spice client: %v", err)
	}

	// Both clients should work
	version1, err := client1.GetVersion(ctx)
	if err != nil {
		t.Fatalf("failed to get version from client1: %v", err)
	}

	version2, err := client2.GetVersion(ctx)
	if err != nil {
		t.Fatalf("failed to get version from client2: %v", err)
	}

	// Versions should match
	if version1 != version2 {
		t.Errorf("expected versions to match, got: %s vs %s", version1, version2)
	}
}

// TestSpiceConfig_DefaultConfig tests using DefaultConfig with spice configured
func TestSpiceConfig_DefaultConfig(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	gsPath := findGitSpiceBinary(t)
	if gsPath == "" {
		t.Skip("git-spice binary not found")
	}

	// Get default config and set spice path
	cfg := config.DefaultConfig()
	cfg.Spice.BinaryPath = gsPath

	// Create client with modified default config
	client, err := spice.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create spice client with default config: %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Verify it works
	ctx := context.Background()
	version, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("failed to get git-spice version: %v", err)
	}

	if !strings.Contains(version, "git-spice") {
		t.Errorf("expected version to contain 'git-spice', got: %s", version)
	}
}

// TestSpiceConfig_EnvironmentVariable tests using git-spice path from environment
func TestSpiceConfig_EnvironmentVariable(t *testing.T) {
	skipIfNoGitSpice(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if GIT_SPICE_PATH environment variable is set
	gsPath := os.Getenv("GIT_SPICE_PATH")
	if gsPath == "" {
		t.Skip("GIT_SPICE_PATH environment variable not set")
	}

	// Verify the path exists
	if _, err := os.Stat(gsPath); os.IsNotExist(err) {
		t.Skipf("GIT_SPICE_PATH points to non-existent file: %s", gsPath)
	}

	cfg := &config.Config{
		Spice: config.SpiceConfig{
			BinaryPath: gsPath,
		},
	}

	client, err := spice.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create spice client with env path: %v", err)
	}

	ctx := context.Background()
	version, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("failed to get git-spice version: %v", err)
	}

	if !strings.Contains(version, "git-spice") {
		t.Errorf("expected version to contain 'git-spice', got: %s", version)
	}
}
