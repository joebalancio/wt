package picker

import "testing"

func TestIsTerminal(t *testing.T) {
	_ = IsTerminal()
}

func TestNewPicker(t *testing.T) {
	picker := NewPicker()
	if picker == nil {
		t.Error("NewPicker() should not return nil")
	}
}
