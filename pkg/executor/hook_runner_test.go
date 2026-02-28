package executor

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/tmux"
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

func TestNewHookRunner_WithTmux(t *testing.T) {
	// Create a hook runner with tmux options
	runner := NewHookRunner("/tmp", WithTmux(nil, "test-window"))
	if runner == nil {
		t.Fatal("NewHookRunner() returned nil")
	}
	if runner.workingDir != "/tmp" {
		t.Errorf("workingDir = %v, want /tmp", runner.workingDir)
	}
	if runner.windowName != "test-window" {
		t.Errorf("windowName = %v, want test-window", runner.windowName)
	}
}

func TestNewHookRunner_WithTemplateVars(t *testing.T) {
	vars := map[string]string{"custom_var": "custom_value"}
	runner := NewHookRunner("/tmp", WithTemplateVars(vars))
	if runner == nil {
		t.Fatal("NewHookRunner() returned nil")
	}
	if runner.templateVars["custom_var"] != "custom_value" {
		t.Errorf("templateVars[custom_var] = %v, want custom_value", runner.templateVars["custom_var"])
	}
	// Should also have default worktree_path
	if runner.templateVars["worktree_path"] != "/tmp" {
		t.Errorf("templateVars[worktree_path] = %v, want /tmp", runner.templateVars["worktree_path"])
	}
}

func TestHookRunner_IsTmuxMode(t *testing.T) {
	// Without tmux
	runner := NewHookRunner("/tmp")
	if runner.isTmuxMode() {
		t.Error("isTmuxMode() should be false without tmux client")
	}

	// With nil tmux client, still not in tmux mode
	runner = NewHookRunner("/tmp", WithTmux(nil, "window"))
	if runner.isTmuxMode() {
		t.Error("isTmuxMode() should be false with nil tmux client")
	}

	// With real tmux client
	client, err := tmux.NewClient()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	runner = NewHookRunner("/tmp", WithTmux(client, "window"))
	if !runner.isTmuxMode() {
		t.Error("isTmuxMode() should be true with real tmux client")
	}
}

func TestWithFinalCommand(t *testing.T) {
	runner := NewHookRunner("/tmp", WithFinalCommand("claude"))
	if runner == nil {
		t.Fatal("NewHookRunner() returned nil")
	}
	if runner.finalCommand != "claude" {
		t.Errorf("finalCommand = %v, want claude", runner.finalCommand)
	}
}

func TestWithFinalCommand_Empty(t *testing.T) {
	runner := NewHookRunner("/tmp", WithFinalCommand(""))
	if runner.finalCommand != "" {
		t.Errorf("finalCommand = %v, want empty", runner.finalCommand)
	}
}

func TestBuildCompoundCommand(t *testing.T) {
	tests := []struct {
		name         string
		workingDir   string
		hooks        []config.Hook
		finalCommand string
		wantContains []string // substrings that should appear in output
	}{
		{
			name:         "no hooks no final command",
			workingDir:   "/tmp/worktree",
			hooks:        nil,
			finalCommand: "",
			wantContains: nil, // empty result
		},
		{
			name:         "no hooks with final command",
			workingDir:   "/tmp/worktree",
			hooks:        nil,
			finalCommand: "claude",
			wantContains: []string{"(cd /tmp/worktree && claude)"},
		},
		{
			name:         "one hook no final command",
			workingDir:   "/tmp/worktree",
			hooks: []config.Hook{
				{Run: "npm install"},
			},
			finalCommand: "",
			wantContains: []string{"(cd /tmp/worktree &&", "npm install)"},
		},
		{
			name:       "multiple hooks with final command",
			workingDir: "/tmp/worktree",
			hooks: []config.Hook{
				{Run: "npm install"},
				{Run: "direnv allow"},
			},
			finalCommand: "claude",
			wantContains: []string{
				"(cd /tmp/worktree &&",
				"npm install)",
				"&&",
				"direnv allow)",
				"claude)",
			},
		},
		{
			name:       "hook with custom cwd",
			workingDir: "/tmp/worktree",
			hooks: []config.Hook{
				{Run: "npm install", Cwd: "/tmp/worktree/frontend"},
			},
			finalCommand: "",
			wantContains: []string{"(cd /tmp/worktree/frontend &&"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewHookRunner(tt.workingDir, WithFinalCommand(tt.finalCommand))
			got := runner.buildCompoundCommand(tt.hooks)

			if tt.wantContains == nil {
				if got != "" {
					t.Errorf("buildCompoundCommand() = %q, want empty", got)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("buildCompoundCommand() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestHookRunner_RunHooks_LocalModeUnchanged(t *testing.T) {
	// Verify local mode behavior is unchanged (hooks block)
	runner := NewHookRunner("/tmp")

	hooks := []config.Hook{
		{Run: "echo hook1"},
		{Run: "echo hook2"},
	}

	err := runner.RunHooks(context.Background(), hooks)
	if err != nil {
		t.Errorf("RunHooks() in local mode error = %v", err)
	}
}

func TestHookRunner_RunHook_TmuxMode(t *testing.T) {
	if os.Getenv("WT_INTEGRATION_TEST") != "1" {
		t.Skip("set WT_INTEGRATION_TEST=1 to run integration tests")
	}

	// Create real tmux client for integration test
	client, err := tmux.NewClient()
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	// Create test window
	testWindow := "test-hook-runner"
	_ = client.KillWindow(testWindow)
	if err := client.CreateNewWindow(testWindow, "/tmp"); err != nil {
		t.Fatalf("CreateNewWindow() error = %v", err)
	}
	defer func() { _ = client.KillWindow(testWindow) }()

	runner := NewHookRunner("/tmp", WithTmux(client, testWindow))

	hook := config.Hook{
		Run:     "echo 'hook executed'",
		Timeout: "5s",
	}

	err = runner.RunHook(context.Background(), hook)
	if err != nil {
		t.Errorf("RunHook() in tmux mode error = %v", err)
	}
}
