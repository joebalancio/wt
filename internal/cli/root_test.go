package cli

import (
	"errors"
	"testing"

	"github.com/joebalancio/wt/internal/picker"
)

func TestIsPickerCancelled(t *testing.T) {
	if !isPickerCancelled(picker.ErrCancelled) {
		t.Fatal("ErrCancelled should be recognized as a picker cancellation")
	}

	if isPickerCancelled(errors.New("boom")) {
		t.Fatal("non-picker error should not be recognized as a picker cancellation")
	}
}
