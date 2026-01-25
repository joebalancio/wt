package tmux

import (
	"testing"
)

func TestParseSessionList(t *testing.T) {
	input := "$0 my-session\n$1 another-session"

	sessions, err := parseSessionList(input)
	if err != nil {
		t.Fatalf("parseSessionList() error = %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("got %d sessions, want 2", len(sessions))
	}

	if sessions[0].Name != "my-session" {
		t.Errorf("first session name = %v, want my-session", sessions[0].Name)
	}
}
