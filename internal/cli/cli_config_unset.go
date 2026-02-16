package cli

import (
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigUnsetCmd creates the config unset command
func NewConfigUnsetCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a config key",
		Long: `Remove a config key, reverting to default value.

By default, keys are removed from the project-local config (.wt.yaml).
Use --global to remove from the global config (~/.config/wt/config.yaml).`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

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

			// Load config
			cfg, err := config.Load(cfgPath)
			if err != nil {
				Fatal("loading config: %v", err)
			}

			// Unset value
			if err := UnsetValue(cfg, key); err != nil {
				Fatal("%v", err)
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
				"✓ Removed %s from %s (%s)\n", key, cfgPath, scopeLabel); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "modify global config instead of project-local")

	return cmd
}
