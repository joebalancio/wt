package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/stack"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/spf13/cobra"
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

// createTmuxWindowForWorktree creates a tmux window for the worktree if conditions are met
func createTmuxWindowForWorktree(cmd *cobra.Command, branch, worktreePath string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	windowName := tmux.GenerateWindowName(branch)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
	}
}

// createStackTmuxWindow creates a tmux window for the stack branch
func createStackTmuxWindow(ctx context.Context, cmd *cobra.Command, stackService *stack.Service, branchName, worktreePath string) {
	if !shouldCreateTmuxWindow(NoTmux()) {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	// Get stack level for window naming
	stackLevel := getStackLevel(ctx, stackService, branchName)

	windowName := tmux.GenerateStackWindowName(branchName, stackLevel)
	if err := tmuxClient.CreateOrSelectWindow(windowName, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to create tmux window: %v\n", err)
	}
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
