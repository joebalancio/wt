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
			if _, err := fmt.Fprintln(out, "Checking dependencies..."); err != nil {
				Fatal("Failed to write output: %v", err)
			}

			// Check git
			if err := checkGit(ctx, out); err != nil {
				Fatal("Git check failed: %v", err)
			}

			// Check git-spice
			if err := checkGitSpice(ctx, out); err != nil {
				Fatal("git-spice check failed: %v", err)
			}

			// Check gh CLI (optional)
			_ = checkGhCLIForInit(ctx, out)

			// Create config file
			if _, err := fmt.Fprintln(out, "\nChecking configuration..."); err != nil {
				Fatal("Failed to write output: %v", err)
			}
			if err := createConfigFile(out); err != nil {
				Fatal("Config setup failed: %v", err)
			}

			if _, err := fmt.Fprintln(out, "\n✓ wt initialized successfully"); err != nil {
				Fatal("Failed to write output: %v", err)
			}
			if _, err := fmt.Fprintf(out, "Config file: %s\n", getConfigPath()); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	return cmd
}

func checkGit(_ context.Context, out io.Writer) error {
	_, err := git.NewClient()
	if err != nil {
		_, _ = fmt.Fprintf(out, "✗ git not found\n")
		return err
	}

	_, _ = fmt.Fprintf(out, "✓ git installed\n")
	return nil
}

func checkGitSpice(_ context.Context, out io.Writer) error {
	// Use detection to check if git-spice is available
	path, err := detectGitSpice()
	if err != nil {
		_, _ = fmt.Fprintf(out, "✗ git-spice not found\n\n")
		_, _ = fmt.Fprintf(out, "  git-spice is required for stacking.\n\n")
		_, _ = fmt.Fprintf(out, "  Install with one of:\n")
		_, _ = fmt.Fprintf(out, "    cargo install git-spice\n")
		_, _ = fmt.Fprintf(out, "    brew install git-spice\n")
		_, _ = fmt.Fprintf(out, "    cargo-binstall git-spice\n\n")
		_, _ = fmt.Fprintf(out, "  Run 'wt init' again after installing.\n")
		return err
	}

	_, _ = fmt.Fprintf(out, "✓ git-spice installed: %s\n", path)
	return nil
}

func checkGhCLIForInit(ctx context.Context, out io.Writer) error {
	ghClient, err := git.NewGhClient()
	if err != nil {
		_, _ = fmt.Fprintln(out, "⚠ gh CLI not found (optional)")
		_, _ = fmt.Fprintln(out, "  gh CLI enables squash-merge detection for 'wt remove'.")
		_, _ = fmt.Fprintln(out, "  Install: brew install gh && gh auth login")
		_, _ = fmt.Fprintln(out)
		return err
	}

	if !ghClient.IsAvailable() {
		_, _ = fmt.Fprintln(out, "⚠ gh CLI unavailable (optional)")
		_, _ = fmt.Fprintln(out)
		return fmt.Errorf("gh client unavailable")
	}

	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintln(out, "⚠ gh CLI installed but not authenticated")
		_, _ = fmt.Fprintln(out, "  Run: gh auth login")
		_, _ = fmt.Fprintln(out)
		return err
	}

	_, _ = fmt.Fprintln(out, "✓ gh CLI installed and authenticated")
	return nil
}

func createConfigFile(out io.Writer) error {
	configPath := getConfigPath()

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		_, _ = fmt.Fprintf(out, "✓ Config exists: %s\n", configPath)
		return nil
	}

	// Create default config
	cfg := config.DefaultConfig()

	// Detect git-spice and add to config
	gitSpicePath, err := detectGitSpice()
	if err != nil {
		_, _ = fmt.Fprintf(out, "Warning: git-spice not found: %v\n", err)
		_, _ = fmt.Fprintf(out, "Stacking features will not work.\n")
		_, _ = fmt.Fprintf(out, "Install git-spice: cargo install git-spice\n")
		_, _ = fmt.Fprintf(out, "Then re-run: wt init\n")
	} else {
		cfg.Spice.BinaryPath = gitSpicePath
		_, _ = fmt.Fprintf(out, "Detected git-spice at: %s\n", gitSpicePath)
	}

	// Validate the default config structure
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	// Write config file
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	_, _ = fmt.Fprintf(out, "✓ Config created: %s\n", configPath)
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
