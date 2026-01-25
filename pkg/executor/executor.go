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

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	if workdir != "" {
		cmd.Dir = workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Add timeout context
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

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

	if Verbose := 0; Verbose > 0 && err != nil {
		result.Output += fmt.Sprintf("\n[exited with status %v after %v]", err, duration)
	}

	return result
}

// RunParallel executes multiple hooks in parallel
func (e *Executor) RunParallel(ctx context.Context, hooks []HookDefinition) []HookResult {
	results := make([]HookResult, len(hooks))
	resultChan := make(chan *HookResult, len(hooks))

	for i, hook := range hooks {
		go func(idx int, h HookDefinition) {
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
