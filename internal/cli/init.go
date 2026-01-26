package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joebalancio/wt/internal/config"
	"github.com/joebalancio/wt/internal/git"
	"github.com/spf13/cobra"
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

func checkGitSpice(_ context.Context, out io.Writer) error {
	// Use detection to check if git-spice is available
	path, err := detectGitSpice()
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

	fmt.Fprintf(out, "✓ git-spice installed: %s\n", path)
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

	// Detect git-spice and add to config
	gitSpicePath, err := detectGitSpice()
	if err != nil {
		fmt.Fprintf(out, "Warning: git-spice not found: %v\n", err)
		fmt.Fprintf(out, "Stacking features will not work.\n")
		fmt.Fprintf(out, "Install git-spice: cargo install git-spice\n")
		fmt.Fprintf(out, "Then re-run: wt init\n")
	} else {
		cfg.Spice.BinaryPath = gitSpicePath
		fmt.Fprintf(out, "Detected git-spice at: %s\n", gitSpicePath)
	}

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

// detectGitSpice locates git-spice binary
// Tries "git-spice" first (most specific), then "gs" with verification
func detectGitSpice() (string, error) {
	// Try "git-spice" first
	if path, err := exec.LookPath("git-spice"); err == nil {
		if err := verifyGitSpice(path); err == nil {
			return path, nil
		}
	}

	// Try "gs" with verification
	if path, err := exec.LookPath("gs"); err == nil {
		if err := verifyGitSpice(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("git-spice not found in PATH (tried git-spice and gs)")
}

// verifyGitSpice checks that the path is actually git-spice
func verifyGitSpice(path string) error {
	cmd := exec.Command(path, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run --version: %w", err)
	}
	if !strings.Contains(string(output), "git-spice") {
		return fmt.Errorf("version output doesn't contain 'git-spice'")
	}
	return nil
}

func init() {
	RegisterCommand(NewInitCmd())
}
