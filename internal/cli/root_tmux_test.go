package cli

import (
	"os"
	"testing"
)

func TestIsInTmux(t *testing.T) {
	// Save original value
	original := os.Getenv("TMUX")
	defer os.Setenv("TMUX", original)

	// Test when not in tmux
	os.Unsetenv("TMUX")
	if isInTmux() {
		t.Error("isInTmux() should return false when TMUX is not set")
	}

	// Test when in tmux
	os.Setenv("TMUX", "/tmp/tmux-1000/default,1234,5678")
	if !isInTmux() {
		t.Error("isInTmux() should return true when TMUX is set")
	}

	// Test empty string
	os.Setenv("TMUX", "")
	if isInTmux() {
		t.Error("isInTmux() should return false when TMUX is empty")
	}
}

func TestShouldCreateTmuxWindow_Default(t *testing.T) {
	// Save original value
	original := os.Getenv("TMUX")
	defer os.Setenv("TMUX", original)

	os.Setenv("TMUX", "/tmp/tmux-1000/default,1234,5678")

	// Test default behavior (should create when in tmux)
	if !shouldCreateTmuxWindow(false) {
		t.Error("shouldCreateTmuxWindow() should return true by default when in tmux")
	}

	// Test with --no-tmux flag
	if shouldCreateTmuxWindow(true) {
		t.Error("shouldCreateTmuxWindow() should return false when --no-tmux is set")
	}
}

func TestShouldCreateTmuxWindow_NotInTmux(t *testing.T) {
	// Save original value
	original := os.Getenv("TMUX")
	defer os.Setenv("TMUX", original)

	os.Unsetenv("TMUX")

	// Should not create window when not in tmux
	if shouldCreateTmuxWindow(false) {
		t.Error("shouldCreateTmuxWindow() should return false when not in tmux")
	}
}
