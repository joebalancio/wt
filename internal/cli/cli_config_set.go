package cli

import (
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigSetCmd creates the config set command
func NewConfigSetCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Long: `Set a config value.

By default, values are set in the project-local config (.wt.yaml).
Use --global to set values in the global config (~/.config/wt/config.yaml).`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]

			// Determine scope based on flag
			scope := ScopeMerged // Default: write to local
			if global {
				scope = ScopeGlobal
			}

			// Get config paths
			projectPath, globalPath, err := ResolveConfigPaths(scope, OpWrite)
			if err != nil {
				Fatal("%v", err)
			}

			// Determine which path to use
			var cfgPath string
			if global {
				cfgPath = globalPath
			} else {
				cfgPath = projectPath
			}

			// Load or create config
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

			scopeLabel := "local"
			if global {
				scopeLabel = "global"
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Updated %s: %s in %s (%s)\n", key, value, cfgPath, scopeLabel); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "modify global config instead of project-local")

	return cmd
}

// loadOrCreateConfig loads an existing config or creates a default one
func loadOrCreateConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.DefaultConfig(), nil
	}
	return cfg, nil
}
