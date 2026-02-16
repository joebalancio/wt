package cli

import (
	"fmt"

	"github.com/joebalancio/wt/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigGetCmd creates the config get command
func NewConfigGetCmd() *cobra.Command {
	var local, global bool

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Long: `Get a config value.

By default, reads the merged config (project-local > global > defaults).
Use --local to read only from project-local config.
Use --global to read only from global config.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]

			// Determine scope
			scope := ScopeMerged // Default: merged read
			if local {
				scope = ScopeLocal
			} else if global {
				scope = ScopeGlobal
			}

			cfg := loadConfigForScope(scope)

			value, err := GetValue(cfg, key)
			if err != nil {
				Fatal("%v", err)
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), formatValue(value)); err != nil {
				Fatal("Failed to write output: %v", err)
			}
		},
	}

	cmd.Flags().BoolVarP(&local, "local", "l", false, "read from project-local config only")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "read from global config only")
	cmd.MarkFlagsMutuallyExclusive("local", "global")

	return cmd
}

// loadConfigForScope loads config based on the specified scope
func loadConfigForScope(scope ConfigScope) *config.Config {
	// Get config paths
	projectPath, globalPath, err := ResolveConfigPaths(scope, OpRead)
	if err != nil {
		Fatal("%v", err)
	}

	// Load appropriate config based on scope
	switch scope {
	case ScopeGlobal:
		return loadGlobalConfigOnly(globalPath)
	case ScopeLocal:
		return loadLocalConfigOnly(projectPath)
	default:
		return loadMergedConfigOrDie(projectPath, globalPath)
	}
}

// loadGlobalConfigOnly loads global config or returns default
func loadGlobalConfigOnly(globalPath string) *config.Config {
	if globalPath == "" {
		return config.DefaultConfig()
	}
	cfg, err := loadOrCreateConfig(globalPath)
	if err != nil {
		Fatal("loading global config: %v", err)
	}
	return cfg
}

// loadLocalConfigOnly loads local config, fails if not in git repo
func loadLocalConfigOnly(projectPath string) *config.Config {
	if projectPath == "" {
		Fatal("not in a git repository\nUse --global to read global config.")
	}
	cfg, err := loadOrCreateConfig(projectPath)
	if err != nil {
		Fatal("loading local config: %v", err)
	}
	return cfg
}

// loadMergedConfigOrDie loads merged config, exits on error
// Shows a warning if outside git repo but still returns config
func loadMergedConfigOrDie(projectPath, globalPath string) *config.Config {
	// Show warning if outside git repo (no project path) but we have global path
	if projectPath == "" && globalPath != "" {
		Warn("not in a git repository. Showing global config.")
	}

	cfg, err := loadMergedConfig(projectPath, globalPath)
	if err != nil {
		Fatal("loading config: %v", err)
	}
	return cfg
}

// loadMergedConfig loads and merges project and global configs
func loadMergedConfig(projectPath, globalPath string) (*config.Config, error) {
	// Handle the case where neither path exists
	if projectPath == "" && globalPath == "" {
		return config.DefaultConfig(), nil
	}
	return config.LoadMerged(projectPath, globalPath)
}
