package config

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

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

func TestTmuxWindowNamingConfig_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Tmux.WindowNaming.MaxLength != 16 {
		t.Errorf("expected MaxLength = 16, got %d", cfg.Tmux.WindowNaming.MaxLength)
	}
	if cfg.Tmux.WindowNaming.AbbreviateIssueID != true {
		t.Errorf("expected AbbreviateIssueID = true, got %v", cfg.Tmux.WindowNaming.AbbreviateIssueID)
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
