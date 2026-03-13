// Package cli provides the command-line interface for wt using Cobra.
// It defines all subcommands (add, list, remove, done, stack, session, config, etc.)
// and handles global flags like --config, --verbose, --dry-run, and --no-tmux.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/picker"
	"github.com/joebalancio/wt/internal/tmux"
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

func runSetupHooksWithWarning(ctx context.Context, cmd *cobra.Command, worktreePath string) {
	if err := runSetupHooks(ctx, worktreePath); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Setup hooks failed: %v\n", err)
	}
}

func runCommandLocallyOrFatal(branch, worktreePath, runCmd string) {
	if runCmd == "" {
		return
	}
	if err := runCommandAfterHooks(RunCommandOpts{
		Command:      runCmd,
		WorktreePath: worktreePath,
		Branch:       branch,
		InTmux:       false,
	}); err != nil {
		Fatal("Failed to run command: %v", err)
	}
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

func isPickerCancelled(err error) bool {
	return errors.Is(err, picker.ErrCancelled)
}

// expandRunTemplate expands template variables in a run command.
// Only {worktree_path} is supported. Unknown templates pass through unchanged.
func expandRunTemplate(command, worktreePath string) string {
	return strings.ReplaceAll(command, "{worktree_path}", worktreePath)
}

// shouldSkipRun returns true if the --run command should be skipped.
// Skips if command is empty or window already existed (don't interrupt).
func shouldSkipRun(command string, windowExisted bool) bool {
	return command == "" || windowExisted
}

// buildShellCommand builds the arguments for sh -c execution.
func buildShellCommand(command string) []string {
	return []string{"sh", "-c", command}
}

// RunCommandOpts contains options for running the --run command.
type RunCommandOpts struct {
	Command       string
	WorktreePath  string
	Branch        string
	WindowName    string
	TmuxClient    *tmux.Client
	WindowExisted bool
	InTmux        bool
}

// runCommandAfterHooks executes the --run command in the appropriate context.
// In tmux: fire-and-forget via SendKeys.
// Outside tmux: exec replacement (process is replaced, never returns).
func runCommandAfterHooks(opts RunCommandOpts) error {
	if shouldSkipRun(opts.Command, opts.WindowExisted) {
		if opts.Command != "" && opts.WindowExisted {
			fmt.Printf("--run skipped: window '%s' already exists\n", opts.WindowName)
		}
		return nil
	}

	cmd := expandRunTemplate(opts.Command, opts.WorktreePath)
	cmd = strings.ReplaceAll(cmd, "{branch}", opts.Branch)
	if opts.InTmux && opts.TmuxClient != nil {
		return runCommandInTmuxWindow(opts.TmuxClient, opts.WindowName, cmd, opts.WorktreePath)
	}

	return execReplace(opts.WorktreePath, cmd)
}

// runCommandInTmuxWindow sends a command to a tmux window.
func runCommandInTmuxWindow(tmuxClient *tmux.Client, windowName, command, worktreePath string) error {
	fullCmd := fmt.Sprintf("cd %s && %s", worktreePath, command)
	if err := tmuxClient.RunInWindow(windowName, fullCmd); err != nil {
		Warn("Failed to run command in tmux: %v", err)
		return err
	}
	return nil
}

// execReplace replaces the current process with the command.
// This is used when running outside tmux. On success, this never returns.
func execReplace(worktreePath, command string) error {
	shPath, err := exec.LookPath("sh")
	if err != nil {
		return fmt.Errorf("sh not found: %w", err)
	}

	if err := os.Chdir(worktreePath); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	args := buildShellCommand(command)
	return syscall.Exec(shPath, args, os.Environ())
}
