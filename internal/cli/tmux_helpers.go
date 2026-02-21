package cli

import (
	"context"

	"github.com/joebalancio/wt/internal/stack"
	"github.com/joebalancio/wt/internal/tmux"
)

// isInTmux checks if currently running in tmux
func isInTmux() bool {
	return tmux.IsInTmux()
}

// shouldCreateTmuxWindow determines if tmux window should be created
func shouldCreateTmuxWindow(noTmuxFlag bool) bool {
	if !isInTmux() {
		return false
	}
	if noTmuxFlag {
		return false
	}
	return true
}

// getStackLevel returns the stack level for a given branch name
func getStackLevel(ctx context.Context, stackService *stack.Service, branchName string) int {
	stackBranches, _ := stackService.GetStack(ctx)
	for i, sb := range stackBranches {
		if sb.Name == branchName {
			return i
		}
	}
	return 0
}
