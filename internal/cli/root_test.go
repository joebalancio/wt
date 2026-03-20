package cli

import (
	"errors"
	"testing"

	"github.com/joebalancio/wt/internal/picker"
)

func TestIsPickerCanceled(t *testing.T) {
	if !isPickerCanceled(picker.ErrCanceled) {
		t.Fatal("ErrCanceled should be recognized as a picker cancellation")
	}

	if isPickerCanceled(errors.New("boom")) {
		t.Fatal("non-picker error should not be recognized as a picker cancellation")
	}
}
