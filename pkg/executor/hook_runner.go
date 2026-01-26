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
	workingDir string
}

// NewHookRunner creates a new hook runner
func NewHookRunner(workingDir string) *HookRunner {
	return &HookRunner{workingDir: workingDir}
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
	if strings.Contains(cwd, "{worktree_path}") {
		cwd = strings.ReplaceAll(cwd, "{worktree_path}", h.workingDir)
	}
	if cwd == "" {
		cwd = h.workingDir
	}

	// Parse command
	parts := strings.Fields(hook.Run)
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
