package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/spf13/cobra"
)

// NewRemoveCmd creates the remove command
func NewRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove a worktree",
		Long: `Remove a worktree from the repository.

By default, this will fail if the worktree has uncommitted changes.
Use --force to remove it anyway.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runRemoveCommand(cmd, args[0], force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force removal even with uncommitted changes")

	return cmd
}

func runRemoveCommand(cmd *cobra.Command, path string, force bool) {
	ctx := context.Background()

	gitClient, err := git.NewClient()
	if err != nil {
		Fatal("Failed to create git client: %v", err)
	}

	cfg, err := loadConfigForCommand()
	if err != nil {
		Fatal("Failed to load config: %v", err)
	}

	svc, err := worktree.NewService(gitClient, cfg)
	if err != nil {
		Fatal("Failed to create service: %v", err)
	}

	if err := svc.Remove(ctx, path, force); err != nil {
		Fatal("Failed to remove worktree: %v", err)
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", path); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Close tmux window if in tmux and window matches
	closeTmuxWindowForPath(ctx, gitClient, path)
}

// closeTmuxWindowForPath closes the tmux window for the given worktree path
func closeTmuxWindowForPath(ctx context.Context, gitClient *git.Client, path string) {
	if !isInTmux() {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}

	// Try to determine branch name from path
	branchName := findBranchByPath(ctx, gitClient, path)
	if branchName == "" {
		return
	}

	windowName := tmux.GenerateWindowName(branchName)
	// Kill the window if it exists
	_ = tmuxClient.KillWindow(windowName)
}

// findBranchByPath finds the branch name for a given worktree path
func findBranchByPath(ctx context.Context, gitClient *git.Client, path string) string {
	worktrees, _ := gitClient.ListWorktrees(ctx)
	for _, wt := range worktrees {
		if wt.Path == path {
			return wt.Branch
		}
	}
	return ""
}

func init() {
	RegisterCommand(NewRemoveCmd())
}
