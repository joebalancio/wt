package cli

import (
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage tmux sessions",
	Long:  `List, attach, and manage tmux sessions for your worktrees.`,
}

func init() {
	RegisterCommand(sessionCmd)
}
