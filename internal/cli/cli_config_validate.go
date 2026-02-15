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
	configPath, err := config.FindConfig("")
	if err != nil {
		if _, err := fmt.Fprintln(cmd.OutOrStderr(), "✗ No config file found"); err != nil {
			Fatal("Failed to write error: %v", err)
		}
		os.Exit(1)
	}

	// Parse YAML
	cfg, err := config.Load(configPath)
	if err != nil {
		if _, writeErr := fmt.Fprintf(cmd.OutOrStderr(),
			"✗ YAML syntax error: %v\n", err); writeErr != nil {
			Fatal("Failed to write error: %v", writeErr)
		}
		os.Exit(1)
	}

	// Validate schema
	if err := cfg.ValidateSchema(); err != nil {
		if _, writeErr := fmt.Fprintf(cmd.OutOrStderr(),
			"✗ Schema validation failed: %v\n", err); writeErr != nil {
			Fatal("Failed to write error: %v", writeErr)
		}
		os.Exit(1)
	}

	printValidationSuccess(cmd.OutOrStdout(), configPath)
}

func printValidationSuccess(out io.Writer, configPath string) {
	if _, err := fmt.Fprintf(out, "✓ Config is valid: %s\n", configPath); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if _, err := fmt.Fprintln(out, "✓ YAML syntax valid"); err != nil {
		Fatal("Failed to write output: %v", err)
	}
	if _, err := fmt.Fprintln(out, "✓ Schema validation passed"); err != nil {
		Fatal("Failed to write output: %v", err)
	}
}
