package cli

import (
	"fmt"
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
			configPath, err := config.FindConfig("")
			if err != nil {
				fmt.Fprintln(cmd.OutOrStderr(), "✗ No config file found")
				os.Exit(1)
			}

			// Parse YAML
			cfg, err := config.Load(configPath)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(),
					"✗ YAML syntax error: %v\n", err)
				os.Exit(1)
			}

			// Validate schema
			if err := cfg.ValidateSchema(); err != nil {
				fmt.Fprintf(cmd.OutOrStderr(),
					"✗ Schema validation failed: %v\n", err)
				os.Exit(1)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Config is valid: %s\n", configPath)
			fmt.Fprintln(cmd.OutOrStdout(), "✓ YAML syntax valid")
			fmt.Fprintln(cmd.OutOrStdout(), "✓ Schema validation passed")
		},
	}
}
