package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMerged(t *testing.T) {
	tests := []struct {
		name                 string
		globalYAML           string
		projectYAML          string
		wantLocation         string // empty means don't check
		wantDedicatedPath    string // empty means don't check
		wantOnWorktreeCreate []Hook // nil means don't check
	}{
		{
			name: "project scalar overrides global",
			globalYAML: `
worktree:
  location: "dedicated"
`,
			projectYAML: `
worktree:
  location: "per-repo"
`,
			wantLocation: "per-repo",
		},
		{
			name: "project array replaces global entirely",
			globalYAML: `
hooks:
  on_worktree_create:
    - run: "npm install"
      cwd: "{worktree_path}"
`,
			projectYAML: `
hooks:
  on_worktree_create:
    - run: "cargo fetch"
      cwd: "{worktree_path}"
`,
			wantOnWorktreeCreate: []Hook{
				{Run: "cargo fetch", Cwd: "{worktree_path}"},
			},
		},
		{
			name: "undefined project field inherits global",
			globalYAML: `
worktree:
  location: "per-repo"
  dedicated_path: "/custom/path"
`,
			projectYAML: `
worktree:
  location: "dedicated"
`,
			wantDedicatedPath: "/custom/path", // inherited from global
			wantLocation:      "dedicated",    // overridden by project
		},
		{
			name:       "project only (no global)",
			globalYAML: "",
			projectYAML: `
worktree:
  location: "per-repo"
`,
			wantLocation: "per-repo",
		},
		{
			name: "global only (no project)",
			globalYAML: `
worktree:
  location: "per-repo"
`,
			projectYAML:  "",
			wantLocation: "per-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			var globalPath, projectPath string

			// Write global config if provided
			if tt.globalYAML != "" {
				globalPath = filepath.Join(tempDir, "global.yaml")
				if err := os.WriteFile(globalPath, []byte(tt.globalYAML), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			// Write project config if provided
			if tt.projectYAML != "" {
				projectPath = filepath.Join(tempDir, "project.yaml")
				if err := os.WriteFile(projectPath, []byte(tt.projectYAML), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			cfg, err := LoadMerged(projectPath, globalPath)
			if err != nil {
				t.Fatalf("LoadMerged() error = %v", err)
			}

			// Compare only the fields we care about
			if tt.wantLocation != "" && cfg.Worktree.Location != tt.wantLocation {
				t.Errorf("Worktree.Location = %q, want %q", cfg.Worktree.Location, tt.wantLocation)
			}
			if tt.wantDedicatedPath != "" && cfg.Worktree.DedicatedPath != tt.wantDedicatedPath {
				t.Errorf("Worktree.DedicatedPath = %q, want %q", cfg.Worktree.DedicatedPath, tt.wantDedicatedPath)
			}
			if tt.wantOnWorktreeCreate != nil {
				if len(cfg.Hooks.OnWorktreeCreate) != len(tt.wantOnWorktreeCreate) {
					t.Errorf("OnWorktreeCreate hooks = %d, want %d",
						len(cfg.Hooks.OnWorktreeCreate), len(tt.wantOnWorktreeCreate))
				} else {
					for i, got := range cfg.Hooks.OnWorktreeCreate {
						want := tt.wantOnWorktreeCreate[i]
						if got.Run != want.Run {
							t.Errorf("Hook[%d].Run = %q, want %q", i, got.Run, want.Run)
						}
						if got.Cwd != want.Cwd {
							t.Errorf("Hook[%d].Cwd = %q, want %q", i, got.Cwd, want.Cwd)
						}
					}
				}
			}
		})
	}
}

func TestWorktreeConfig_IsDedicated(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     bool
	}{
		{"empty defaults to dedicated", "", true},
		{"explicit dedicated", "dedicated", true},
		{"per-repo", "per-repo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := WorktreeConfig{Location: tt.location}
			if got := cfg.IsDedicated(); got != tt.want {
				t.Errorf("IsDedicated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorktreeConfig_GetDedicatedPath(t *testing.T) {
	tests := []struct {
		name          string
		dedicatedPath string
		want          string
	}{
		{"custom path", "/custom/worktrees", "/custom/worktrees"},
		{"empty uses default", "", "~/worktrees"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := WorktreeConfig{DedicatedPath: tt.dedicatedPath}
			if got := cfg.GetDedicatedPath(); got != tt.want {
				t.Errorf("GetDedicatedPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig_HasWorktreeSettings(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Worktree.IsDedicated() {
		t.Error("default config should use dedicated worktree location")
	}
	if cfg.Worktree.GetDedicatedPath() != "~/worktrees" {
		t.Errorf("default dedicated path = %v, want ~/worktrees", cfg.Worktree.GetDedicatedPath())
	}
}

func TestSpiceConfig_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Spice.BinaryPath != "" {
		t.Errorf("expected empty BinaryPath, got %q", cfg.Spice.BinaryPath)
	}
}

func TestFindGitRoot(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T, dir string)
		wantErr     bool
		errContains string
	}{
		{
			name: "in git repo returns root",
			setupFunc: func(t *testing.T, dir string) {
				runGitCommand(t, dir, "init")
				runGitCommand(t, dir, "config", "user.email", "test@test.com")
				runGitCommand(t, dir, "config", "user.name", "Test")
			},
			wantErr: false,
		},
		{
			name: "not in git repo returns error",
			setupFunc: func(_ *testing.T, _ string) {
				// Do nothing - not a git repo
			},
			wantErr:     true,
			errContains: "not in a git repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			tt.setupFunc(t, tempDir)

			// Change to temp dir for test
			originalWd, _ := os.Getwd()
			defer os.Chdir(originalWd)
			os.Chdir(tempDir)

			root, err := FindGitRoot()

			if tt.wantErr {
				if err == nil {
					t.Errorf("FindGitRoot() expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("FindGitRoot() error = %v, want containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("FindGitRoot() unexpected error: %v", err)
				}
				if root == "" {
					t.Error("FindGitRoot() returned empty root")
				}
			}
		})
	}
}

// Helper for git commands in tests
func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, output)
	}
}

func TestFindConfigs(t *testing.T) {
	tests := []struct {
		name        string
		customPath  string
		setupFunc   func(t *testing.T, dir string) (globalPath string)
		wantProject bool
		wantGlobal  bool
		wantErr     bool
	}{
		{
			name:       "custom path skips discovery",
			customPath: "/custom/config.yaml",
			setupFunc: func(t *testing.T, dir string) string {
				runGitCommand(t, dir, "init")
				// Set HOME to temp dir to avoid finding real global config
				os.Setenv("HOME", dir)
				return ""
			},
			wantProject: false,
			wantGlobal:  false,
			wantErr:     true, // custom path doesn't exist
		},
		{
			name: "project config only at git root",
			setupFunc: func(t *testing.T, dir string) string {
				runGitCommand(t, dir, "init")
				runGitCommand(t, dir, "config", "user.email", "test@test.com")
				runGitCommand(t, dir, "config", "user.name", "Test")
				// Create .wt.yaml at root
				if err := os.WriteFile(filepath.Join(dir, ".wt.yaml"), []byte("tmux:\n  layout: test\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				// Set HOME to temp dir (no global config there)
				os.Setenv("HOME", dir)
				return ""
			},
			wantProject: true,
			wantGlobal:  false,
			wantErr:     false,
		},
		{
			name: "global config only (not in git repo)",
			setupFunc: func(_ *testing.T, dir string) string {
				// Create global config in temp location
				globalDir := filepath.Join(dir, ".config", "wt")
				os.MkdirAll(globalDir, 0o755)
				globalPath := filepath.Join(globalDir, "config.yaml")
				os.WriteFile(globalPath, []byte("tmux:\n  layout: global\n"), 0o644)
				// Set XDG_CONFIG_HOME equivalent via HOME
				os.Setenv("HOME", dir)
				return globalPath
			},
			wantProject: false,
			wantGlobal:  true,
			wantErr:     false,
		},
		{
			name: "both project and global configs",
			setupFunc: func(t *testing.T, dir string) string {
				runGitCommand(t, dir, "init")
				runGitCommand(t, dir, "config", "user.email", "test@test.com")
				runGitCommand(t, dir, "config", "user.name", "Test")
				// Project config
				os.WriteFile(filepath.Join(dir, ".wt.yaml"), []byte("tmux:\n  layout: project\n"), 0o644)
				// Global config
				globalDir := filepath.Join(dir, ".config", "wt")
				os.MkdirAll(globalDir, 0o755)
				globalPath := filepath.Join(globalDir, "config.yaml")
				os.WriteFile(globalPath, []byte("tmux:\n  layout: global\n"), 0o644)
				os.Setenv("HOME", dir)
				return globalPath
			},
			wantProject: true,
			wantGlobal:  true,
			wantErr:     false,
		},
		{
			name: "no configs found",
			setupFunc: func(_ *testing.T, dir string) string {
				// Not in git repo, no global config
				os.Setenv("HOME", dir) // HOME points to empty dir
				return ""
			},
			wantProject: false,
			wantGlobal:  false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Save original HOME
			origHome := os.Getenv("HOME")
			defer os.Setenv("HOME", origHome)

			// Setup
			_ = tt.setupFunc(t, tempDir)

			// Change to temp dir
			originalWd, _ := os.Getwd()
			defer os.Chdir(originalWd)
			os.Chdir(tempDir)

			projectPath, globalPath, err := FindConfigs(tt.customPath)

			if tt.wantErr {
				if err == nil {
					t.Error("FindConfigs() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("FindConfigs() unexpected error: %v", err)
				return
			}

			if tt.wantProject && projectPath == "" {
				t.Error("FindConfigs() expected project path, got empty")
			}
			if !tt.wantProject && projectPath != "" {
				t.Errorf("FindConfigs() expected no project path, got %q", projectPath)
			}
			if tt.wantGlobal && globalPath == "" {
				t.Error("FindConfigs() expected global path, got empty")
			}
			if !tt.wantGlobal && globalPath != "" {
				t.Errorf("FindConfigs() expected no global path, got %q", globalPath)
			}
		})
	}
}
