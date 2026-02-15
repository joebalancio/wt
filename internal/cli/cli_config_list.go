package cli

import (
	"fmt"

	configpkg "github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewConfigListCmd creates the config list command
func NewConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all config values",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			cfg, err := loadActiveConfig()
			if err != nil {
				Fatal("loading config: %v", err)
			}

			// Explicitly reference config package to ensure import is used
			_ = configpkg.DefaultConfig

			data, err := yaml.Marshal(cfg)
			if err != nil {
				Fatal("marshaling config: %v", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(data)); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}
}
