package cli

import (
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigUnsetCmd creates the config unset command
func NewConfigUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a config key (global config only)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

			cfgPath := getGlobalConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				Fatal("loading config: %v", err)
			}

			if err := UnsetValue(cfg, key); err != nil {
				Fatal("%v", err)
			}

			if err := cfg.Save(cfgPath); err != nil {
				Fatal("saving config: %v", err)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Removed %s from %s\n", key, cfgPath); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}
}

// getGlobalConfigPath returns the global config file path
// TODO: Remove this function when cli_config_unset.go is updated to use ResolveConfigPaths (Task 3)
func getGlobalConfigPath() string {
	_, globalPath, err := ResolveConfigPaths(ScopeGlobal, OpWrite)
	if err != nil {
		Fatal("%v", err)
	}
	return globalPath
}
