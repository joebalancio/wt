package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/joebalancio/wt/internal/config"
)

// HookRunner executes post-create hooks
type HookRunner struct {
	workingDir   string
	templateVars map[string]string
}

// NewHookRunner creates a new hook runner
func NewHookRunner(workingDir string, templateVars ...map[string]string) *HookRunner {
	hr := &HookRunner{
		workingDir:   workingDir,
		templateVars: make(map[string]string),
	}
	// Set default template variables
	hr.templateVars["worktree_path"] = workingDir
	// Override with provided template variables
	if len(templateVars) > 0 && templateVars[0] != nil {
		for k, v := range templateVars[0] {
			hr.templateVars[k] = v
		}
	}
	return hr
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
	// Expand {worktree_path} template
	cwd := hook.Cwd
	cwd = h.substituteTemplates(cwd)
	if cwd == "" {
		cwd = h.workingDir
	}

	// Parse command
	runCommand := h.substituteTemplates(hook.Run)
	parts := strings.Fields(runCommand)
	if len(parts) == 0 {
		return fmt.Errorf("empty hook command")
	}

	// Add timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %q: %w", hook.Run, err)
	}

	return nil
}
