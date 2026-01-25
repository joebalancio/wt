package util

import (
	"testing"

	"github.com/aidarkhanov/nanoid"
)

func TestGenerateSuffix(t *testing.T) {
	suffix, err := nanoid.Generate(nanoid.DefaultAlphabet, 4)
	if err != nil {
		t.Fatalf("failed to generate suffix: %v", err)
	}
	if len(suffix) != 4 {
		t.Errorf("expected length 4, got %d", len(suffix))
	}
}
