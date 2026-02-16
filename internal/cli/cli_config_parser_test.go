package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joebalancio/wt/internal/config"
)

func TestGetValue(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		name      string
		key       string
		expected  string
		wantError bool
	}{
		{"worktree location", "worktree.location", "", false}, // empty means per-repo (default)
		{"worktree dedicated_path", "worktree.dedicated_path", "~/worktrees", false},
		{"invalid key format", "invalid", "", true},
		{"unsupported key", "hooks.on_worktree_create", "", true},
		{"unknown section", "unknown.field", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := GetValue(cfg, tt.key)
			if tt.wantError {
				if err == nil {
					t.Errorf("GetValue() expected error for %q, got nil", tt.key)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetValue() unexpected error: %v", err)
			}
			if formatValue(value) != tt.expected {
				t.Errorf("GetValue() = %q, want %q", formatValue(value), tt.expected)
			}
		})
	}
}

func TestSetValue(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		wantError bool
	}{
		{"valid string", "worktree.dedicated_path", "/tmp/wt", false},
		{"valid enum dedicated", "worktree.location", "dedicated", false},
		{"valid enum per-repo", "worktree.location", "per-repo", false},
		{"invalid enum", "worktree.location", "invalid", true},
		{"unsupported key", "hooks.on_worktree_create", "echo hi", true},
		{"invalid key format", "invalid", "value", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			err := SetValue(cfg, tt.key, tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("SetValue() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestUnsetValue(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantError bool
		validate  func(*config.Config) // Check the value was reverted to default
	}{
		{
			name:      "unset worktree location",
			key:       "worktree.location",
			wantError: false,
			validate: func(cfg *config.Config) {
				if cfg.Worktree.Location != "" {
					t.Errorf("expected default '' (per-repo), got %q", cfg.Worktree.Location)
				}
			},
		},
		{
			name:      "unset worktree dedicated_path",
			key:       "worktree.dedicated_path",
			wantError: false,
			validate: func(cfg *config.Config) {
				if cfg.Worktree.DedicatedPath != "" {
					t.Errorf("expected empty string, got %q", cfg.Worktree.DedicatedPath)
				}
			},
		},
		{
			name:      "unsupported key",
			key:       "hooks.on_worktree_create",
			wantError: true,
			validate:  nil,
		},
		{
			name:      "invalid key format",
			key:       "invalid",
			wantError: true,
			validate:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			// First set a non-default value
			_ = SetValue(cfg, tt.key, "non-default")

			err := UnsetValue(cfg, tt.key)
			if (err != nil) != tt.wantError {
				t.Errorf("UnsetValue() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && tt.validate != nil {
				tt.validate(cfg)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"boolean true", true, "true"},
		{"boolean false", false, "false"},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatValue(tt.input); got != tt.want {
				t.Errorf("formatValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      bool
		wantError bool
	}{
		{"true lowercase", "true", true, false},
		{"true uppercase", "TRUE", true, false},
		{"true mixed case", "True", true, false},
		{"1", "1", true, false},
		{"yes", "yes", true, false},
		{"yes uppercase", "YES", true, false},
		{"on", "on", true, false},
		{"false lowercase", "false", false, false},
		{"false uppercase", "FALSE", false, false},
		{"0", "0", false, false},
		{"no", "no", false, false},
		{"off", "off", false, false},
		{"invalid", "maybe", false, true},
		{"empty", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBool(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("parseBool() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got != tt.want {
				t.Errorf("parseBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		min       int
		max       int
		expected  int
		wantError bool
	}{
		{"valid in range", "10", 1, 32, 10, false},
		{"valid at min", "1", 1, 32, 1, false},
		{"valid at max", "32", 1, 32, 32, false},
		{"invalid non-numeric", "abc", 1, 32, 0, true},
		{"invalid too small", "0", 1, 32, 0, true},
		{"invalid too large", "33", 1, 32, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInt(tt.input, tt.min, tt.max)
			if tt.wantError {
				if err == nil {
					t.Errorf("parseInt() expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseInt() unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("parseInt() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestResolveConfigPaths(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Create a temp git repo for testing
	tmpDir := t.TempDir()
	// Initialize a real git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("initializing git repo: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	// Get expected global path
	home, _ := os.UserHomeDir()
	expectedGlobalPath := filepath.Join(home, ".config", "wt", "config.yaml")
	expectedProjectPath := filepath.Join(tmpDir, ".wt.yaml")

	// Check if global config actually exists
	globalConfigExists := false
	if _, err := os.Stat(expectedGlobalPath); err == nil {
		globalConfigExists = true
	}

	tests := []struct {
		name           string
		scope          ConfigScope
		op             Operation
		wantProject    string
		wantGlobal     string
		wantGlobalFunc func() string // Used when global path depends on environment
		wantError      bool
		errorContains  string
	}{
		{
			name:        "ScopeGlobal returns global path only",
			scope:       ScopeGlobal,
			op:          OpRead,
			wantProject: "",
			wantGlobal:  expectedGlobalPath,
			wantError:   false,
		},
		{
			name:        "ScopeLocal read inside git repo",
			scope:       ScopeLocal,
			op:          OpRead,
			wantProject: expectedProjectPath,
			wantGlobal:  "",
			wantError:   false,
		},
		{
			name:        "ScopeLocal write inside git repo",
			scope:       ScopeLocal,
			op:          OpWrite,
			wantProject: expectedProjectPath,
			wantGlobal:  "",
			wantError:   false,
		},
		{
			name:      "ScopeMerged read calls FindConfigs",
			scope:     ScopeMerged,
			op:        OpRead,
			wantError: false,
			wantGlobalFunc: func() string {
				if globalConfigExists {
					return expectedGlobalPath
				}
				return ""
			},
		},
		{
			name:        "ScopeMerged write defaults to local",
			scope:       ScopeMerged,
			op:          OpWrite,
			wantProject: expectedProjectPath,
			wantGlobal:  "",
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath, globalPath, err := ResolveConfigPaths(tt.scope, tt.op)

			if tt.wantError {
				if err == nil {
					t.Errorf("ResolveConfigPaths() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveConfigPaths() unexpected error: %v", err)
				return
			}
			if projectPath != tt.wantProject {
				t.Errorf("ResolveConfigPaths() projectPath = %q, want %q", projectPath, tt.wantProject)
			}
			expectedGlobal := tt.wantGlobal
			if tt.wantGlobalFunc != nil {
				expectedGlobal = tt.wantGlobalFunc()
			}
			if globalPath != expectedGlobal {
				t.Errorf("ResolveConfigPaths() globalPath = %q, want %q", globalPath, expectedGlobal)
			}
		})
	}
}

func TestResolveConfigPathsOutsideGit(t *testing.T) {
	// Save original working directory
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)

	// Change to a temp directory without .git
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to tmp dir: %v", err)
	}

	tests := []struct {
		name          string
		scope         ConfigScope
		op            Operation
		wantError     bool
		errorContains string
	}{
		{
			name:          "ScopeLocal read outside git repo errors",
			scope:         ScopeLocal,
			op:            OpRead,
			wantError:     true,
			errorContains: "not in a git repository",
		},
		{
			name:          "ScopeLocal write outside git repo errors",
			scope:         ScopeLocal,
			op:            OpWrite,
			wantError:     true,
			errorContains: "not in a git repository",
		},
		{
			name:          "ScopeMerged write outside git repo errors",
			scope:         ScopeMerged,
			op:            OpWrite,
			wantError:     true,
			errorContains: "not in a git repository",
		},
		{
			name:      "ScopeGlobal works outside git repo",
			scope:     ScopeGlobal,
			op:        OpWrite,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ResolveConfigPaths(tt.scope, tt.op)

			if tt.wantError {
				if err == nil {
					t.Errorf("ResolveConfigPaths() expected error, got nil")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("ResolveConfigPaths() error = %q, want containing %q", err.Error(), tt.errorContains)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveConfigPaths() unexpected error: %v", err)
			}
		})
	}
}
