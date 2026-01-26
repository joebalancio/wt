package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/joebalancio/wt/internal/git"
)

// NewSetupCmd creates the setup command
func NewSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Re-run setup hooks in current worktree",
		Long: `Re-run all post-create hooks for the current worktree.

This is useful after manually fixing setup issues or when you want
to refresh your development environment.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			// Get current directory
			wd, err := os.Getwd()
			if err != nil {
				Fatal("Failed to get working directory: %v", err)
			}

			// Verify we're in a git worktree
			gitClient, err := git.NewClient()
			if err != nil {
				Fatal("Failed to create git client: %v", err)
			}

			repoInfo, err := gitClient.GetRepoInfo(ctx)
			if err != nil {
				Fatal("Not in a git repository: %v", err)
			}

			// Check if we're in a worktree (not the main repo)
			if isMainWorktree(wd, repoInfo.RootPath) {
				Fatal("Setup hooks should be run from a worktree, not the main repository")
			}

			fmt.Fprintf(out, "Running setup hooks for: %s\n", filepath.Base(wd))

			// Run hooks
			if err := runSetupHooks(ctx, wd); err != nil {
				Fatal("Setup hooks failed: %v", err)
			}

			fmt.Fprintln(out, "✓ Setup complete")
		},
	}

	return cmd
}

func isMainWorktree(wd, repoRoot string) bool {
	// Normalize paths
	wd = filepath.Clean(wd)
	repoRoot = filepath.Clean(repoRoot)
	return wd == repoRoot
}

func init() {
	RegisterCommand(NewSetupCmd())
}
