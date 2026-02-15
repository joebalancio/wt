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
			customPath, _ := cmd.Flags().GetString("config")

			projectPath, globalPath, err := config.FindConfigs(customPath)
			if err != nil {
				fmt.Fprintln(cmd.OutOrStderr(), "✗ No config file found")
				os.Exit(1)
			}

			// Validate merged config
			cfg, err := config.LoadMerged(projectPath, globalPath)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(),
					"✗ Config load error: %v\n", err)
				os.Exit(1)
			}

			// Schema validation
			if err := cfg.ValidateSchema(); err != nil {
				fmt.Fprintf(cmd.OutOrStderr(),
					"✗ Schema validation error: %v\n", err)
				os.Exit(1)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "✓ Configuration is valid")

			// Show which configs are active
			if projectPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Project: %s\n", projectPath)
			}
			if globalPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Global: %s\n", globalPath)
			}
		},
	}
}
