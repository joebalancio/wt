//go:build integration
// +build integration

package tests

import (
	"context"
	"os"
	"testing"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/pkg/executor"
)

func TestHooksRunInTmuxWindow(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	// Create tmux client
	client, err := tmux.NewClient()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	// Create test window
	testWindow := "test-hooks-in-window"
	_ = client.KillWindow(testWindow)

	tempDir := t.TempDir()
	if err := client.CreateNewWindow(testWindow, tempDir); err != nil {
		t.Fatalf("CreateNewWindow() error = %v", err)
	}
	defer func() { _ = client.KillWindow(testWindow) }()

	// Create hook runner with tmux mode
	runner := executor.NewHookRunner(tempDir, executor.WithTmux(client, testWindow))

	// Run a hook
	hooks := []config.Hook{
		{Run: "echo 'Hook running in tmux window'", Timeout: "10s"},
		{Run: "pwd", Timeout: "5s"},
	}

	err = runner.RunHooks(context.Background(), hooks)
	if err != nil {
		t.Errorf("RunHooks() error = %v", err)
	}
}

func TestHooksWithTimeoutInTmux(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := tmux.NewClient()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	testWindow := "test-hooks-timeout"
	_ = client.KillWindow(testWindow)

	tempDir := t.TempDir()
	if err := client.CreateNewWindow(testWindow, tempDir); err != nil {
		t.Fatalf("CreateNewWindow() error = %v", err)
	}
	defer func() { _ = client.KillWindow(testWindow) }()

	runner := executor.NewHookRunner(tempDir, executor.WithTmux(client, testWindow))

	// Hook with short timeout - should complete
	hooks := []config.Hook{
		{Run: "sleep 1 && echo 'done'", Timeout: "5s"},
	}

	err = runner.RunHooks(context.Background(), hooks)
	if err != nil {
		t.Errorf("RunHooks() error = %v", err)
	}
}

func TestHookTimeoutField(t *testing.T) {
	// Unit test for the Timeout field parsing
	tests := []struct {
		name        string
		timeout     string
		wantDefault bool
		wantErr     bool
	}{
		{"empty uses default", "", true, false},
		{"valid seconds", "30s", false, false},
		{"valid minutes", "2m", false, false},
		{"valid hours", "1h", false, false},
		{"invalid bare number", "30", false, true},
		{"invalid format", "invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := config.Hook{Run: "echo test", Timeout: tt.timeout}
			dur, err := hook.ParseTimeout()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.wantDefault {
				if dur != config.DefaultHookTimeout {
					t.Errorf("ParseTimeout() = %v, want default %v", dur, config.DefaultHookTimeout)
				}
			}
		})
	}
}
