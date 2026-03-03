//go:build integration
// +build integration

package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/pkg/executor"
)

func TestHooksCompleteBeforeRun(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	client, err := tmux.NewClient()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	tmpDir := t.TempDir()
	worktreePath := filepath.Join(tmpDir, "worktree")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	windowName := "test-hooks-run-order"
	_ = client.KillWindow(windowName)
	if err := client.CreateNewWindow(windowName, tmpDir); err != nil {
		t.Fatalf("CreateNewWindow() error = %v", err)
	}
	defer func() { _ = client.KillWindow(windowName) }()

	markerFile := filepath.Join(tmpDir, "marker")
	hooks := []config.Hook{
		{
			Run:     "sleep 1 && touch " + markerFile,
			Timeout: "10s",
		},
	}

	finalCmd := "test -f " + markerFile + " && echo SUCCESS || echo FAIL"

	runner := executor.NewHookRunner(
		worktreePath,
		executor.WithTmux(client, windowName),
		executor.WithFinalCommand(finalCmd),
	)

	if err := runner.RunHooks(context.Background(), hooks); err != nil {
		t.Errorf("RunHooks() error = %v", err)
	}

	time.Sleep(2 * time.Second)

	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Error("Hook did not complete before final command - marker file not created")
	}
}
