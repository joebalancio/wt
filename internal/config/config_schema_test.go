package config

import (
	"testing"
)

func TestValidateSchema(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default",
			modify:  func(*Config) {},
			wantErr: false,
		},
		{
			name: "valid dedicated",
			modify: func(c *Config) {
				c.Worktree.Location = "dedicated"
			},
			wantErr: false,
		},
		{
			name: "valid per-repo",
			modify: func(c *Config) {
				c.Worktree.Location = "per-repo"
			},
			wantErr: false,
		},
		{
			name: "valid empty location (defaults to dedicated)",
			modify: func(c *Config) {
				c.Worktree.Location = ""
			},
			wantErr: false,
		},
		{
			name: "invalid location",
			modify: func(c *Config) {
				c.Worktree.Location = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid location random",
			modify: func(c *Config) {
				c.Worktree.Location = "something-else"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.ValidateSchema()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSchema() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSchema_ErrorMessages(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Worktree.Location = "invalid-value"

	err := cfg.ValidateSchema()
	if err == nil {
		t.Fatal("ValidateSchema() expected error for invalid location, got nil")
	}

	errMsg := err.Error()
	// Check that error message contains useful information
	expectedSubstrings := []string{
		"invalid worktree.location",
		"invalid-value",
		"dedicated",
		"per-repo",
	}

	for _, substr := range expectedSubstrings {
		if !contains(errMsg, substr) {
			t.Errorf("Error message should contain %q, got: %s", substr, errMsg)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
