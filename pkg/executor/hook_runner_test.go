package executor

import (
	"context"
	"testing"
	"time"

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

func TestDetectTimeoutCommand(t *testing.T) {
	// This test verifies the timeout command detection logic
	// The actual command found depends on the system

	cmd := detectTimeoutCommand()
	// On Linux, should find "timeout"
	// On macOS with coreutils, should find "gtimeout"
	// We just verify it doesn't panic and returns a valid string or empty
	t.Logf("detected timeout command: %q", cmd)
}

func TestBuildTimedCommand(t *testing.T) {
	tests := []struct {
		name       string
		timeoutCmd string
		duration   time.Duration
		command    string
		want       string
	}{
		{
			name:       "with timeout command",
			timeoutCmd: "timeout",
			duration:   30 * time.Second,
			command:    "npm install",
			want:       "timeout 30s npm install",
		},
		{
			name:       "with gtimeout command",
			timeoutCmd: "gtimeout",
			duration:   2 * time.Minute,
			command:    "cargo build",
			want:       "gtimeout 120s cargo build",
		},
		{
			name:       "no timeout command available",
			timeoutCmd: "",
			duration:   30 * time.Second,
			command:    "echo test",
			want:       "echo test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTimedCommand(tt.timeoutCmd, tt.duration, tt.command)
			if got != tt.want {
				t.Errorf("buildTimedCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}
