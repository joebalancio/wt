package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigSetCmd creates the config set command
func NewConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value (global config only)",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]

			// Always use global config path
			cfgPath := getGlobalConfigPath()
			cfg, err := loadOrCreateConfig(cfgPath)
			if err != nil {
				Fatal("loading config: %v", err)
			}

			// Set value
			if err := SetValue(cfg, key, value); err != nil {
				Fatal("%v", err)
			}

			// Validate schema
			if err := cfg.ValidateSchema(); err != nil {
				Fatal("config validation failed: %v", err)
			}

			// Save
			if err := cfg.Save(cfgPath); err != nil {
				Fatal("saving config: %v", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Updated %s: %s in %s\n", key, value, cfgPath)
		},
	}
}

// getGlobalConfigPath returns the global config file path
func getGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to environment variable
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "wt", "config.yaml")
}

// loadOrCreateConfig loads an existing config or creates a default one
func loadOrCreateConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.DefaultConfig(), nil
	}
	return cfg, nil
}
