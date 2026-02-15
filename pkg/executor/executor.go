// Package executor handles subprocess execution for hooks and shell commands.
// It provides timeout management, parallel execution support, and output capture
// for running user-defined hooks during worktree lifecycle events.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// HookResult represents the result of a hook execution
type HookResult struct {
	Hook    string
	Success bool
	Output  string
	Error   error
}

// Executor handles subprocess execution
type Executor struct {
	timeout time.Duration
	verbose int
}

// New creates a new Executor
func New() *Executor {
	return &Executor{
		timeout: 5 * time.Minute,
	}
}

// SetTimeout sets the execution timeout
func (e *Executor) SetTimeout(d time.Duration) {
	e.timeout = d
}

// SetVerboseLevel sets the verbosity level (0 = quiet, >0 = verbose)
func (e *Executor) SetVerboseLevel(level int) {
	e.verbose = level
}

// GetVerboseLevel returns the current verbosity level
func (e *Executor) GetVerboseLevel() int {
	return e.verbose
}

// Run executes a command with context and timeout
func (e *Executor) Run(ctx context.Context, workdir string, command string) *HookResult {
	startTime := time.Now()

	// Parse command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return &HookResult{
			Hook:    command,
			Success: false,
			Error:   fmt.Errorf("empty command"),
		}
	}

	// Add timeout context before creating the command
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)

	output := stdout.String()
	if stderr.String() != "" {
		output += "\n" + stderr.String()
	}

	result := &HookResult{
		Hook:    command,
		Success: err == nil,
		Output:  output,
		Error:   err,
	}

	// Add verbose output with timing information when verbosity is enabled
	if e.verbose > 0 {
		if err != nil {
			result.Output += fmt.Sprintf("\n[exited with error: %v after %v]", err, duration)
		} else {
			result.Output += fmt.Sprintf("\n[completed successfully after %v]", duration)
		}
	}

	return result
}

// RunParallel executes multiple hooks in parallel
func (e *Executor) RunParallel(ctx context.Context, hooks []HookDefinition) []HookResult {
	results := make([]HookResult, len(hooks))
	resultChan := make(chan *HookResult, len(hooks))

	for i, hook := range hooks {
		go func(_ int, h HookDefinition) {
			resultChan <- e.Run(ctx, h.Workdir, h.Command)
		}(i, hook)
	}

	for i := 0; i < len(hooks); i++ {
		result := <-resultChan
		results[i] = *result
	}

	return results
}

// HookDefinition represents a hook to execute
type HookDefinition struct {
	Command string
	Workdir string
}
