//go:build integration
// +build integration

package tests

import (
	"testing"

	"github.com/joebalancio/wt/internal/cli"
)

func TestAddRunFlag_Available(t *testing.T) {
	cmd := cli.NewAddCmd()
	flag := cmd.Flags().Lookup("run")
	if flag == nil {
		t.Fatal("expected --run flag on wt add")
	}
	if flag.DefValue != "" {
		t.Fatalf("expected empty default for --run, got %q", flag.DefValue)
	}
}

func TestStackRunFlag_Available(t *testing.T) {
	cmd := cli.NewStackCmd()
	flag := cmd.Flags().Lookup("run")
	if flag == nil {
		t.Fatal("expected --run flag on wt stack")
	}
	if flag.DefValue != "" {
		t.Fatalf("expected empty default for --run, got %q", flag.DefValue)
	}
}
