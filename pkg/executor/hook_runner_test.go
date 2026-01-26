package executor

import (
	"context"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestNewHookRunner(t *testing.T) {
	runner := NewHookRunner("/tmp")
	if runner == nil {
		t.Fatal("NewHookRunner() returned nil")
	}
	if runner.workingDir != "/tmp" {
		t.Errorf("workingDir = %v, want /tmp", runner.workingDir)
	}
}

func TestHookRunner_RunHooks(t *testing.T) {
	runner := NewHookRunner("/tmp")

	// Test with empty hooks
	err := runner.RunHooks(context.Background(), []config.Hook{})
	if err != nil {
		t.Errorf("RunHooks() with empty hooks error = %v", err)
	}
}
