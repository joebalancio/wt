package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/tmux"
)

var (
	timeoutCmdOnce sync.Once
	timeoutCmd     string
)

// detectTimeoutCommand checks for available timeout command
// Returns "timeout", "gtimeout" (macOS coreutils), or "" if not found
func detectTimeoutCommand() string {
	timeoutCmdOnce.Do(func() {
		if _, err := exec.LookPath("timeout"); err == nil {
			timeoutCmd = "timeout"
		} else if _, err := exec.LookPath("gtimeout"); err == nil {
			timeoutCmd = "gtimeout"
		}
	})
	return timeoutCmd
}

// buildTimedCommand wraps a command with timeout if available
func buildTimedCommand(timeoutBin string, d time.Duration, command string) string {
	if timeoutBin == "" {
		return command
	}
	return fmt.Sprintf("%s %ds %s", timeoutBin, int(d.Seconds()), command)
}

// HookRunner executes post-create hooks
type HookRunner struct {
	workingDir   string
	templateVars map[string]string
	tmuxClient   *tmux.Client // nil = run locally
	windowName   string       // used if tmuxClient is set
}

// HookRunnerOption configures a HookRunner
type HookRunnerOption func(*HookRunner)

// WithTmux sets tmux execution mode
func WithTmux(client *tmux.Client, windowName string) HookRunnerOption {
	return func(hr *HookRunner) {
		hr.tmuxClient = client
		hr.windowName = windowName
	}
}

// WithTemplateVars sets additional template variables
func WithTemplateVars(vars map[string]string) HookRunnerOption {
	return func(hr *HookRunner) {
		for k, v := range vars {
			hr.templateVars[k] = v
		}
	}
}

// NewHookRunner creates a new hook runner
// Optional tmuxClient and windowName for running hooks in tmux window
func NewHookRunner(workingDir string, opts ...HookRunnerOption) *HookRunner {
	hr := &HookRunner{
		workingDir:   workingDir,
		templateVars: make(map[string]string),
	}

	// Set default template variables
	hr.templateVars["worktree_path"] = workingDir

	// Apply options
	for _, opt := range opts {
		opt(hr)
	}

	return hr
}

// isTmuxMode returns true if hooks should run in tmux window
func (h *HookRunner) isTmuxMode() bool {
	return h.tmuxClient != nil
}

// substituteTemplates replaces template variables in a string
func (h *HookRunner) substituteTemplates(cmd string) string {
	result := cmd
	for key, value := range h.templateVars {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// RunHooks executes all hooks in sequence
func (h *HookRunner) RunHooks(ctx context.Context, hooks []config.Hook) error {
	for i, hook := range hooks {
		if err := h.RunHook(ctx, hook); err != nil {
			return fmt.Errorf("hook %d failed: %w", i, err)
		}
	}
	return nil
}

// RunHook executes a single hook
func (h *HookRunner) RunHook(ctx context.Context, hook config.Hook) error {
	// Expand templates
	cwd := hook.Cwd
	cwd = h.substituteTemplates(cwd)
	if cwd == "" {
		cwd = h.workingDir
	}

	runCommand := h.substituteTemplates(hook.Run)
	if runCommand == "" {
		return fmt.Errorf("empty hook command")
	}

	// Parse timeout
	timeout, err := hook.ParseTimeout()
	if err != nil {
		return fmt.Errorf("parsing timeout: %w", err)
	}

	// Execute in tmux window or locally
	if h.isTmuxMode() {
		return h.runHookInTmux(hook, runCommand, cwd, timeout)
	}
	return h.runHookLocally(ctx, hook, runCommand, cwd, timeout)
}

// runHookLocally executes hook in current terminal
func (h *HookRunner) runHookLocally(ctx context.Context, hook config.Hook, command, cwd string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty hook command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %q: %w", hook.Run, err)
	}
	return nil
}

// runHookInTmux executes hook in tmux window
func (h *HookRunner) runHookInTmux(hook config.Hook, command, cwd string, timeout time.Duration) error {
	// Build the command with cd prefix and timeout wrapper
	timeoutBin := detectTimeoutCommand()
	timedCmd := buildTimedCommand(timeoutBin, timeout, command)

	// Build full command: cd <cwd> && <timedCmd>
	fullCmd := fmt.Sprintf("cd %s && %s", cwd, timedCmd)

	// Run in tmux window
	if err := h.tmuxClient.RunInWindow(h.windowName, fullCmd); err != nil {
		return fmt.Errorf("running %q in tmux: %w", hook.Run, err)
	}
	return nil
}
