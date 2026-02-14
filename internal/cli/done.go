package cli

import (
	"context"
	"fmt"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/spf13/cobra"
)

// NewDoneCmd creates the done command
func NewDoneCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "done <path> <branch>",
		Short: "Complete and remove a worktree",
		Long: `Complete and remove a worktree by merging it into the current branch.

This command will:
1. Squash merge the branch into the current branch
2. Create a commit with the merge
3. Remove the worktree
4. Delete the branch

By default, this will fail if the worktree has uncommitted changes.
Use --force to proceed anyway with a dirty worktree.`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			path := args[0]
			branch := args[1]

			ctx := context.Background()

			// Dry run mode
			if GetDryRun() {
				fmt.Fprintf(cmd.OutOrStdout(), "Would complete worktree: %s (branch: %s)\n", path, branch)
				fmt.Fprintf(cmd.OutOrStdout(), "  - Squash merge %s into current branch\n", branch)
				fmt.Fprintf(cmd.OutOrStdout(), "  - Create merge commit\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  - Remove worktree at %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "  - Delete branch %s\n", branch)
				if isInTmux() {
					fmt.Fprintf(cmd.OutOrStdout(), "  - Close tmux window if exists\n")
				}
				return
			}

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

			if err := svc.Done(ctx, path, branch, force); err != nil {
				Fatal("Failed to complete worktree: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Completed worktree: %s (branch: %s)\n", path, branch)

			// Close tmux window if in tmux and window matches
			if isInTmux() {
				tmuxClient, err := tmux.NewClient()
				if err == nil {
					windowName := tmux.GenerateWindowName(branch)
					// Kill the window if it exists
					_ = tmuxClient.KillWindow(windowName)
				}
			}
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force completion even with uncommitted changes")

	return cmd
}

func init() {
	RegisterCommand(NewDoneCmd())
}
