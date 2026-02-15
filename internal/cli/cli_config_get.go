package cli

import (
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigGetCmd creates the config get command
func NewConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			cfg, err := loadActiveConfig()
			if err != nil {
				Fatal("loading config: %v", err)
			}

			value, err := GetValue(cfg, key)
			if err != nil {
				Fatal("%v", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), formatValue(value)); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}
}

// loadActiveConfig loads the active config (respects discovery order)
func loadActiveConfig() (*config.Config, error) {
	configPath, err := config.FindConfig("")
	if err != nil {
		return config.DefaultConfig(), nil
	}
	return config.Load(configPath)
}
