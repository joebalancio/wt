package cli

import (
	"testing"
)

func TestNewDoctorCmd(t *testing.T) {
	cmd := NewDoctorCmd()
	if cmd == nil {
		t.Fatal("NewDoctorCmd() returned nil")
	}
	if cmd.Use != "doctor" {
		t.Errorf("Use = %v, want doctor", cmd.Use)
	}
}
