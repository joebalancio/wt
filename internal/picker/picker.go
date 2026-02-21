// Package picker provides interactive TUI selection for wt commands.
package picker

import (
	"os"

	"golang.org/x/term"
)

// Picker provides interactive selection functionality.
type Picker struct{}

// NewPicker creates a new Picker instance.
func NewPicker() *Picker {
	return &Picker{}
}

// IsTerminal returns true if stdout is connected to a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
