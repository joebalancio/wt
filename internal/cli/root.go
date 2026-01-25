package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "A high-level CLI tool for managing git worktrees with tmux integration",
	Long: `wt is a tool that simplifies git worktree management combined with tmux session
handling. It supports configurable hooks that run after worktree setup to automate
your development environment preparation.`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// RegisterCommand adds a subcommand to the root
func RegisterCommand(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path (default is $HOME/.config/wt/config.yaml or .wt.yaml in project)")
	rootCmd.PersistentFlags().CountP("verbose", "v", "verbose output (can be used multiple times)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without executing")
}

var dryRun bool

// GetDryRun returns the global dry-run flag value
func GetDryRun() bool {
	return dryRun
}

// Verbose returns the verbosity level
func Verbose() int {
	v, _ := rootCmd.PersistentFlags().GetCount("verbose")
	return v
}

// Fatal prints an error and exits with code 1
func Fatal(format string, args ...interface{}) {
	fmt.Fprintf(rootCmd.ErrOrStderr(), "Error: "+format+"\n", args...)
	os.Exit(1)
}
