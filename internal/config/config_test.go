package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Global.WorktreeRoot == "" {
		t.Error("WorktreeRoot should have a default value")
	}
	if cfg.Global.TmuxSessionPrefix == "" {
		t.Error("TmuxSessionPrefix should have a default value")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty worktree root",
			cfg: &Config{
				Global: GlobalConfig{WorktreeRoot: ""},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "test.yaml")

	yamlContent := `
global:
  worktree_root: ~/dev/worktrees
  tmux_session_prefix: "wt-"
tmux:
  layout: main-vertical
  window_name: work
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Tmux.Layout != "main-vertical" {
		t.Errorf("Layout = %v, want main-vertical", cfg.Tmux.Layout)
	}
}
