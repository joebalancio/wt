package cli

import (
	"context"
	"fmt"
	"io"

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
			runDoneCommand(cmd, args[0], args[1], force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force completion even with uncommitted changes")

	return cmd
}

func runDoneCommand(cmd *cobra.Command, path, branch string, force bool) {
	ctx := context.Background()

	// Dry run mode
	if GetDryRun() {
		printDoneDryRun(cmd.OutOrStdout(), path, branch)
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

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Completed worktree: %s (branch: %s)\n", path, branch); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Close tmux window if in tmux and window matches
	closeTmuxWindowForBranch(branch)
}

func printDoneDryRun(out io.Writer, path, branch string) {
	if _, err := fmt.Fprintf(out, "Would complete worktree: %s (branch: %s)\n", path, branch); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if _, err := fmt.Fprintf(out, "  - Squash merge %s into current branch\n", branch); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if _, err := fmt.Fprintf(out, "  - Create merge commit\n"); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if _, err := fmt.Fprintf(out, "  - Remove worktree at %s\n", path); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if _, err := fmt.Fprintf(out, "  - Delete branch %s\n", branch); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if isInTmux() {
		if _, err := fmt.Fprintf(out, "  - Close tmux window if exists\n"); err != nil {
			Fatal("Failed to write output: %v", err)
		}
	}
}

func closeTmuxWindowForBranch(branch string) {
	if !isInTmux() {
		return
	}
	tmuxClient, err := tmux.NewClient()
	if err == nil {
		windowName := tmux.GenerateWindowName(branch)
		// Kill the window if it exists
		_ = tmuxClient.KillWindow(windowName)
	}
}

func init() {
	RegisterCommand(NewDoneCmd())
}
