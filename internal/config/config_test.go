package config

import (
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
