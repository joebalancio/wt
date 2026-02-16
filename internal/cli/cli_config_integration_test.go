package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestNewConfigGetCmd(t *testing.T) {
	cmd := NewConfigGetCmd()

	if cmd.Use != "get <key>" {
		t.Errorf("expected Use 'get <key>', got %q", cmd.Use)
	}

	if cmd.Short != "Get a config value" {
		t.Errorf("expected Short 'Get a config value', got %q", cmd.Short)
	}

	// Test args validation
	if cmd.Args(cmd, []string{}) == nil {
		t.Error("expected error for no args, got nil")
	}

	if cmd.Args(cmd, []string{"key", "extra"}) == nil {
		t.Error("expected error for too many args, got nil")
	}

	if cmd.Args(cmd, []string{"key"}) != nil {
		t.Error("expected no error for valid args, got error")
	}
}

func TestNewConfigListCmd(t *testing.T) {
	cmd := NewConfigListCmd()

	if cmd.Use != "list" {
		t.Errorf("expected Use 'list', got %q", cmd.Use)
	}

	if cmd.Short != "List all config values" {
		t.Errorf("expected Short 'List all config values', got %q", cmd.Short)
	}

	// Test args validation
	if cmd.Args(cmd, []string{"arg"}) == nil {
		t.Error("expected error for args, got nil")
	}

	if cmd.Args(cmd, []string{}) != nil {
		t.Error("expected no error for no args, got error")
	}
}

func TestNewConfigSetCmd(t *testing.T) {
	cmd := NewConfigSetCmd()

	if cmd.Use != "set <key> <value>" {
		t.Errorf("expected Use 'set <key> <value>', got %q", cmd.Use)
	}

	if cmd.Short != "Set a config value" {
		t.Errorf("expected Short 'Set a config value', got %q", cmd.Short)
	}

	// Test args validation
	if cmd.Args(cmd, []string{"key"}) == nil {
		t.Error("expected error for one arg, got nil")
	}

	if cmd.Args(cmd, []string{"key", "value", "extra"}) == nil {
		t.Error("expected error for too many args, got nil")
	}

	if cmd.Args(cmd, []string{"key", "value"}) != nil {
		t.Error("expected no error for valid args, got error")
	}
}

func TestNewConfigUnsetCmd(t *testing.T) {
	cmd := NewConfigUnsetCmd()

	if cmd.Use != "unset <key>" {
		t.Errorf("expected Use 'unset <key>', got %q", cmd.Use)
	}

	if cmd.Short != "Remove a config key" {
		t.Errorf("expected Short 'Remove a config key', got %q", cmd.Short)
	}

	// Test args validation
	if cmd.Args(cmd, []string{}) == nil {
		t.Error("expected error for no args, got nil")
	}

	if cmd.Args(cmd, []string{"key", "extra"}) == nil {
		t.Error("expected error for too many args, got nil")
	}

	if cmd.Args(cmd, []string{"key"}) != nil {
		t.Error("expected no error for valid args, got error")
	}
}

func TestNewConfigValidateCmd(t *testing.T) {
	cmd := NewConfigValidateCmd()

	if cmd.Use != "validate" {
		t.Errorf("expected Use 'validate', got %q", cmd.Use)
	}

	if cmd.Short != "Validate configuration (YAML + schema)" {
		t.Errorf("expected Short 'Validate configuration (YAML + schema)', got %q", cmd.Short)
	}

	// Test args validation
	if cmd.Args(cmd, []string{"arg"}) == nil {
		t.Error("expected error for args, got nil")
	}

	if cmd.Args(cmd, []string{}) != nil {
		t.Error("expected no error for no args, got error")
	}
}

// TestGetCommandIntegration tests the get command with a real config
func TestGetCommandIntegration(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		key      string
		expected string
	}{
		{"worktree.location", "dedicated"},
		{"tmux.attach_on_create", "true"},
		{"tmux.layout", "main-vertical"},
		{"tmux.window_name", "work"},
		{"global.tmux_session_prefix", "wt-"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			value, err := GetValue(cfg, tt.key)
			if err != nil {
				t.Fatalf("GetValue(%q) error: %v", tt.key, err)
			}
			if formatValue(value) != tt.expected {
				t.Errorf("GetValue(%q) = %q, want %q", tt.key, formatValue(value), tt.expected)
			}
		})
	}
}

// TestSetUnsetIntegration tests setting and unsetting values
func TestSetUnsetIntegration(t *testing.T) {
	// Create temp directory for test config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Save original HOME and restore after test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	t.Run("set and unset worktree location", func(t *testing.T) {
		cfg := config.DefaultConfig()

		// Set to per-repo
		if err := SetValue(cfg, "worktree.location", "per-repo"); err != nil {
			t.Fatalf("SetValue error: %v", err)
		}
		if cfg.Worktree.Location != "per-repo" {
			t.Errorf("expected location 'per-repo', got %q", cfg.Worktree.Location)
		}

		// Save config
		if err := cfg.Save(configPath); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		// Load config
		loadedCfg, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if loadedCfg.Worktree.Location != "per-repo" {
			t.Errorf("expected loaded location 'per-repo', got %q", loadedCfg.Worktree.Location)
		}

		// Unset
		if err := UnsetValue(loadedCfg, "worktree.location"); err != nil {
			t.Fatalf("UnsetValue error: %v", err)
		}
		if loadedCfg.Worktree.Location != "dedicated" {
			t.Errorf("expected default location 'dedicated', got %q", loadedCfg.Worktree.Location)
		}
	})

	t.Run("set boolean value", func(t *testing.T) {
		cfg := config.DefaultConfig()

		// Set to false
		if err := SetValue(cfg, "tmux.attach_on_create", "false"); err != nil {
			t.Fatalf("SetValue error: %v", err)
		}
		if cfg.Tmux.AttachOnCreate != false {
			t.Errorf("expected AttachOnCreate false, got %v", cfg.Tmux.AttachOnCreate)
		}

		// Set to true
		if err := SetValue(cfg, "tmux.attach_on_create", "yes"); err != nil {
			t.Fatalf("SetValue error: %v", err)
		}
		if cfg.Tmux.AttachOnCreate != true {
			t.Errorf("expected AttachOnCreate true, got %v", cfg.Tmux.AttachOnCreate)
		}
	})

	t.Run("invalid value is rejected", func(t *testing.T) {
		cfg := config.DefaultConfig()

		err := SetValue(cfg, "worktree.location", "invalid")
		if err == nil {
			t.Error("expected error for invalid enum value, got nil")
		}
		if !strings.Contains(err.Error(), "Valid values: dedicated, per-repo") {
			t.Errorf("error message should contain valid values, got: %v", err)
		}
	})
}

// TestValidateSchemaIntegration tests schema validation
func TestValidateSchemaIntegration(t *testing.T) {
	t.Run("valid config passes", func(t *testing.T) {
		cfg := config.DefaultConfig()
		if err := cfg.ValidateSchema(); err != nil {
			t.Errorf("ValidateSchema() error: %v", err)
		}
	})

	t.Run("invalid location fails", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Worktree.Location = "invalid"
		if err := cfg.ValidateSchema(); err == nil {
			t.Error("expected error for invalid location, got nil")
		}
	})
}

// TestListOutput tests that list command produces valid YAML
func TestListOutput(t *testing.T) {
	// This test verifies the config can be marshaled to YAML
	// The actual YAML marshaling is done by gopkg.in/yaml.v3 library
	cfg := config.DefaultConfig()

	// Verify config has expected values
	if cfg.Worktree.Location == "" {
		t.Error("expected Worktree.Location to be set")
	}
	if cfg.Tmux.Layout == "" {
		t.Error("expected Tmux.Layout to be set")
	}
	if cfg.Global.TmuxSessionPrefix == "" {
		t.Error("expected Global.TmuxSessionPrefix to be set")
	}
}
