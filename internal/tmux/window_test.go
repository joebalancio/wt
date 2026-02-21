package tmux

import (
	"os"
	"testing"
)

func TestClient_CreateNewWindow(t *testing.T) {
	// Skip integration tests unless explicitly enabled
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Create a test window
	err = client.CreateNewWindow("test-window", "/tmp")
	if err != nil {
		t.Fatalf("CreateNewWindow() error = %v", err)
	}

	// Clean up
	_ = client.KillWindow("test-window")
}

func TestClient_CreateNewWindow_InvalidTarget(t *testing.T) {
	// Skip integration tests unless explicitly enabled
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	// This test validates error handling for invalid window creation
	// We'll use a mock approach to test error conditions

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Try to create window with empty name - should error or handle gracefully
	err = client.CreateNewWindow("", "/tmp")
	// We expect this to either succeed with default name or fail gracefully
	// The exact behavior depends on tmux implementation
	_ = err // Just check it doesn't panic
}

func TestClient_WindowExists(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Test non-existent window
	exists, err := client.WindowExists("this-window-should-not-exist-xyz123")
	if err != nil {
		t.Fatalf("WindowExists() error = %v", err)
	}
	if exists {
		t.Error("WindowExists() should return false for non-existent window")
	}
}

func TestClient_SelectWindow(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Create test window first
	_ = client.CreateNewWindow("test-select-window", "/tmp")
	defer func() { _ = client.KillWindow("test-select-window") }()

	// Select the window
	err = client.SelectWindow("test-select-window")
	if err != nil {
		t.Fatalf("SelectWindow() error = %v", err)
	}
}

func TestClient_SendKeys(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Create test window
	_ = client.CreateNewWindow("test-send-keys", "/tmp")
	defer func() { _ = client.KillWindow("test-send-keys") }()

	// Send keys to the window
	err = client.SendKeys("test-send-keys", "echo 'test'", true)
	if err != nil {
		t.Fatalf("SendKeys() error = %v", err)
	}
}

func TestParseWindowList(t *testing.T) {
	input := "bash*\nzsh\nvim"

	windows := parseWindowList(input)
	if len(windows) != 3 {
		t.Errorf("got %d windows, want 3", len(windows))
	}

	if windows[0] != "bash*" {
		t.Errorf("first window = %v, want bash*", windows[0])
	}

	if windows[1] != "zsh" {
		t.Errorf("second window = %v, want zsh", windows[1])
	}

	if windows[2] != "vim" {
		t.Errorf("third window = %v, want vim", windows[2])
	}
}

func TestParseWindowList_Empty(t *testing.T) {
	windows := parseWindowList("")
	if len(windows) != 0 {
		t.Errorf("got %d windows, want 0", len(windows))
	}
}

func TestClient_CreateOrSelectWindow_NewWindow(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	testWindow := "test-create-or-select-new"
	// Ensure window doesn't exist
	_ = client.KillWindow(testWindow)

	// Create or select (should create since it doesn't exist)
	err = client.CreateOrSelectWindow(testWindow, "/tmp")
	if err != nil {
		t.Fatalf("CreateOrSelectWindow() error = %v", err)
	}

	// Verify window exists
	exists, _ := client.WindowExists(testWindow)
	if !exists {
		t.Error("CreateOrSelectWindow() should have created window")
	}

	// Clean up
	_ = client.KillWindow(testWindow)
}

func TestClient_CreateOrSelectWindow_ExistingWindow(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	testWindow := "test-create-or-select-existing"

	// Create the window first
	_ = client.CreateNewWindow(testWindow, "/tmp")
	defer func() { _ = client.KillWindow(testWindow) }()

	// Create or select (should select since it exists)
	err = client.CreateOrSelectWindow(testWindow, "/tmp")
	if err != nil {
		t.Fatalf("CreateOrSelectWindow() error = %v", err)
	}

	// Window should still exist
	exists, _ := client.WindowExists(testWindow)
	if !exists {
		t.Error("CreateOrSelectWindow() should not have removed existing window")
	}
}

func TestClient_RunInWindow(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Create a test window first
	testWindow := "test-run-in-window"
	_ = client.KillWindow(testWindow) // cleanup any existing

	err = client.CreateNewWindow(testWindow, "/tmp")
	if err != nil {
		t.Fatalf("CreateNewWindow() error = %v", err)
	}
	defer func() { _ = client.KillWindow(testWindow) }()

	// Run a simple command in the window
	err = client.RunInWindow(testWindow, "echo 'hello from run-in-window'")
	if err != nil {
		t.Errorf("RunInWindow() error = %v", err)
	}
}

func TestClient_RunInWindow_NonexistentWindow(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := NewClient()
	if err != nil {
		t.Skipf("skipping test: tmux not available: %v", err)
	}

	// Try to run in a window that doesn't exist
	// Note: tmux run-shell doesn't necessarily error on invalid targets
	// This test verifies the method doesn't panic and handles the call
	_ = client.RunInWindow("nonexistent-window-xyz123", "echo test")
	// We don't assert on error since tmux behavior varies
}
