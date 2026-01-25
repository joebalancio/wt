package cli

import (
	"github.com/spf13/cobra"
)

var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage git worktrees",
	Long:  `Create, list, and remove git worktrees with automatic tmux session management.`,
}

func init() {
	RegisterCommand(worktreeCmd)
	// Subcommands are registered in their respective init() functions
}
