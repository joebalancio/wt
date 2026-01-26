package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/joebalancio/wt/internal/spice"
)

// NewInitCmd creates the init command
func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize wt configuration",
		Long: `Create default configuration file and check dependencies.

This command creates ~/.config/wt/config.yaml if it doesn't exist,
and verifies that required dependencies (git, git-spice) are installed.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := context.Background()
			out := cmd.OutOrStdout()

			// Check dependencies
			fmt.Fprintln(out, "Checking dependencies...")

			// Check git
			if err := checkGit(ctx, out); err != nil {
				Fatal("Git check failed: %v", err)
			}

			// Check git-spice
			if err := checkGitSpice(ctx, out); err != nil {
				Fatal("git-spice check failed: %v", err)
			}

			// Create config file
			fmt.Fprintln(out, "\nChecking configuration...")
			if err := createConfigFile(out); err != nil {
				Fatal("Config setup failed: %v", err)
			}

			fmt.Fprintln(out, "\n✓ wt initialized successfully")
			fmt.Fprintf(out, "Config file: %s\n", getConfigPath())
		},
	}

	return cmd
}

func checkGit(_ context.Context, out io.Writer) error {
	_, err := git.NewClient()
	if err != nil {
		fmt.Fprintf(out, "✗ git not found\n")
		return err
	}

	fmt.Fprintf(out, "✓ git installed\n")
	return nil
}

func checkGitSpice(ctx context.Context, out io.Writer) error {
	spiceClient, err := spice.NewClient()
	if err != nil {
		fmt.Fprintf(out, "✗ git-spice not found\n\n")
		fmt.Fprintf(out, "  git-spice is required for stacking.\n\n")
		fmt.Fprintf(out, "  Install with one of:\n")
		fmt.Fprintf(out, "    cargo install git-spice\n")
		fmt.Fprintf(out, "    brew install git-spice\n")
		fmt.Fprintf(out, "    cargo-binstall git-spice\n\n")
		fmt.Fprintf(out, "  Run 'wt init' again after installing.\n")
		return err
	}

	version, err := spiceClient.GetVersion(ctx)
	if err != nil {
		fmt.Fprintf(out, "✗ git-spice installed but version check failed\n")
		return err
	}

	fmt.Fprintf(out, "✓ git-spice installed: %s\n", version)
	return nil
}

func createConfigFile(out io.Writer) error {
	configPath := getConfigPath()

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Fprintf(out, "✓ Config exists: %s\n", configPath)
		return nil
	}

	// Create default config
	cfg := config.DefaultConfig()

	// Validate the default config structure
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	// Write config file
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(out, "✓ Config created: %s\n", configPath)
	return nil
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to HOME environment variable
		home = os.Getenv("HOME")
		if home == "" {
			// Last resort: use current directory
			home = "."
		}
	}
	return filepath.Join(home, ".config", "wt", "config.yaml")
}

func init() {
	RegisterCommand(NewInitCmd())
}
