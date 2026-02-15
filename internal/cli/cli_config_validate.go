package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigValidateCmd creates the config validate command
func NewConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration (YAML + schema)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			runConfigValidate(cmd)
		},
	}
}

func runConfigValidate(cmd *cobra.Command) {
	customPath, _ := cmd.Flags().GetString("config")

	projectPath, globalPath, err := config.FindConfigs(customPath)
	if err != nil {
		if _, err := fmt.Fprintln(cmd.OutOrStderr(), "✗ No config file found"); err != nil {
			Fatal("Failed to write error: %v", err)
		}
		os.Exit(1)
	}

	// Validate merged config
	cfg, err := config.LoadMerged(projectPath, globalPath)
	if err != nil {
		if _, writeErr := fmt.Fprintf(cmd.OutOrStderr(),
			"✗ Config load error: %v\n", err); writeErr != nil {
			Fatal("Failed to write error: %v", writeErr)
		}
		os.Exit(1)
	}

	// Schema validation
	if err := cfg.ValidateSchema(); err != nil {
		if _, writeErr := fmt.Fprintf(cmd.OutOrStderr(),
			"✗ Schema validation error: %v\n", err); writeErr != nil {
			Fatal("Failed to write error: %v", writeErr)
		}
		os.Exit(1)
	}

	printValidationSuccess(cmd.OutOrStdout(), projectPath, globalPath)
}

func printValidationSuccess(out io.Writer, projectPath, globalPath string) {
	if _, err := fmt.Fprintln(out, "✓ Configuration is valid"); err != nil {
		Fatal("Failed to write output: %v", err)
	}

	// Show which configs are active
	if projectPath != "" {
		if _, err := fmt.Fprintf(out, "  Project: %s\n", projectPath); err != nil {
			Fatal("Failed to write output: %v", err)
		}
	}
	if globalPath != "" {
		if _, err := fmt.Fprintf(out, "  Global: %s\n", globalPath); err != nil {
			Fatal("Failed to write output: %v", err)
		}
	}

	if _, err := fmt.Fprintln(out, "✓ YAML syntax valid"); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if _, err := fmt.Fprintln(out, "✓ Schema validation passed"); err != nil {
		Fatal("Failed to write output: %v", err)
	}
}
