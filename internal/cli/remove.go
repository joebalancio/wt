package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/picker"
	"github.com/joebalancio/wt/internal/tmux"
	"github.com/joebalancio/wt/internal/worktree"
	"github.com/joebalancio/wt/pkg/domain"
	"github.com/spf13/cobra"
)

// NewRemoveCmd creates the remove command
func NewRemoveCmd() *cobra.Command {
	var forceStr string

	cmd := &cobra.Command{
		Use:   "remove [path]",
		Short: "Remove a worktree and its branch",
		Long: `Remove a worktree and its associated branch from the repository.

If no path is provided, resolves the worktree from the current working directory.

By default, this will fail if:
- The worktree has uncommitted changes
- The branch is not merged

Use --force to remove anyway (forces local deletion only).
Use --force=remote to also delete the remote branch.

Force levels:
  --force         Force local worktree and branch deletion
  --force=remote  Force local deletion + delete remote branch
  --force=all     Same as --force=remote`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			force, err := domain.ParseForceLevel(forceStr)
			if err != nil {
				Fatal("%v", err)
			}

			path := ""
			if len(args) > 0 {
				path = args[0]
			}
			runRemoveCommand(cmd, path, force)
		},
	}

	cmd.Flags().StringVar(&forceStr, "force", "", "force removal (true, remote, or all)")
	if flag := cmd.Flags().Lookup("force"); flag != nil {
		flag.NoOptDefVal = "true"
	}

	return cmd
}

func runRemoveCommand(cmd *cobra.Command, path string, force domain.ForceLevel) {
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

	resolvedPath := path
	if resolvedPath == "" {
		if picker.IsTerminal() {
			inWorktree, _, err := gitClient.IsInWorktree(ctx)
			if err != nil {
				Fatal("Failed to check worktree context: %v", err)
			}
			if !inWorktree {
				p := picker.NewPicker(gitClient)
				selected, err := p.SelectWorktree(ctx)
				if err != nil {
					Fatal("Error: %v. Provide a path: wt remove <path>", err)
				}
				resolvedPath = selected
			}
		}

		if resolvedPath == "" {
			cwd, err := os.Getwd()
			if err != nil {
				Fatal("Failed to get current directory: %v", err)
			}
			wt, err := svc.ResolveFromCWD(ctx, cwd)
			if err != nil {
				Fatal("Error: %v. Provide a path: wt remove <path>", err)
			}
			resolvedPath = wt.Path
		}
	}

	branchName := findBranchByPath(ctx, gitClient, resolvedPath)
	if err := svc.RemoveEnhanced(ctx, resolvedPath, force); err != nil {
		Fatal("Failed to remove worktree: %v", err)
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", resolvedPath); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if branchName != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch: %s\n", branchName); err != nil {
			Fatal("Failed to write output: %v", err)
		}
	}

	// Close tmux window if in tmux and window matches
	if branchName != "" {
		closeRemoveTmuxWindowForBranch(branchName)
	}
}

// closeRemoveTmuxWindowForBranch closes the tmux window for the given branch.
func closeRemoveTmuxWindowForBranch(branch string) {
	if !isInTmux() {
		return
	}

	tmuxClient, err := tmux.NewClient()
	if err != nil {
		return
	}
	windowName := tmux.GenerateWindowName(branch)
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
