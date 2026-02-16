// Package cli provides the command-line interface for wt using Cobra.
// It defines all subcommands (add, list, remove, done, stack, session, config, etc.)
// and handles global flags like --config, --verbose, --dry-run, and --no-tmux.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/pkg/executor"
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
	rootCmd.PersistentFlags().Bool("no-tmux", false, "skip tmux window creation")
	rootCmd.PersistentFlags().Bool("dry-run", false, "show what would be done without making changes")
}

// Verbose returns the verbosity level
func Verbose() int {
	v, _ := rootCmd.PersistentFlags().GetCount("verbose")
	return v
}

// NoTmux returns the value of the --no-tmux flag
func NoTmux() bool {
	noTmux, _ := rootCmd.PersistentFlags().GetBool("no-tmux")
	return noTmux
}

// GetDryRun returns the value of the --dry-run flag
func GetDryRun() bool {
	dryRun, _ := rootCmd.PersistentFlags().GetBool("dry-run")
	return dryRun
}

// loadConfigForCommand loads config from flags, returning defaults if not found
func loadConfigForCommand() (*config.Config, error) {
	// Check for --config flag
	customPath, _ := rootCmd.PersistentFlags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		// No config found - return defaults
		return config.DefaultConfig(), nil
	}

	return config.LoadMerged(projectPath, globalPath)
}

// runSetupHooks executes post-create hooks for a worktree
func runSetupHooks(ctx context.Context, worktreePath string) error {
	cfg, err := loadConfigForCommand()
	if err != nil {
		return err
	}

	runner := executor.NewHookRunner(worktreePath)
	return runner.RunHooks(ctx, cfg.Hooks.OnWorktreeCreate)
}

// Fatal prints an error and exits with code 1
func Fatal(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(rootCmd.ErrOrStderr(), "Error: "+format+"\n", args...)
	os.Exit(1)
}

// Warn prints a warning to stderr (does not exit)
func Warn(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}
