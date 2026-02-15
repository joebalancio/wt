package domain

import (
	"testing"
)

func TestForceLevel_String(t *testing.T) {
	tests := []struct {
		name  string
		level ForceLevel
		want  string
	}{
		{"none", ForceNone, "none"},
		{"local", ForceLocal, "local"},
		{"remote", ForceRemote, "remote"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("ForceLevel.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseForceLevel(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ForceLevel
		wantErr bool
	}{
		{"empty", "", ForceNone, false},
		{"false", "false", ForceNone, false},
		{"true", "true", ForceLocal, false},
		{"local", "local", ForceLocal, false},
		{"remote", "remote", ForceRemote, false},
		{"all", "all", ForceRemote, false},
		{"invalid", "invalid", ForceNone, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseForceLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseForceLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseForceLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
