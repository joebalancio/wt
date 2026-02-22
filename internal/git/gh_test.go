package git

import "testing"

func TestNewGhClient(t *testing.T) {
	client, err := NewGhClient()
	if err != nil {
		if client != nil {
			t.Error("client should be nil when error occurs")
		}
		return
	}
	if client == nil {
		t.Error("client should not be nil when no error")
	}
}

func TestGhClient_IsAvailable(t *testing.T) {
	client, err := NewGhClient()
	if err != nil {
		t.Skip("gh not available")
	}

	if !client.IsAvailable() {
		t.Error("IsAvailable() should return true when client exists")
	}
}

func TestParsePRStateJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		state    string
		hasError bool
	}{
		{
			name:     "merged PR",
			input:    `{"state": "MERGED"}`,
			state:    "MERGED",
			hasError: false,
		},
		{
			name:     "open PR",
			input:    `{"state": "OPEN"}`,
			state:    "OPEN",
			hasError: false,
		},
		{
			name:     "closed PR (not merged)",
			input:    `{"state": "CLOSED"}`,
			state:    "CLOSED",
			hasError: false,
		},
		{
			name:     "invalid JSON",
			input:    `not json`,
			state:    "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := parsePRStateJSON(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if state != tt.state {
				t.Errorf("state = %q, want %q", state, tt.state)
			}
		})
	}
}

func TestGhClient_IsBranchPRMerged_NoClient(t *testing.T) {
	client := &GhClient{ghPath: ""}
	if client.IsAvailable() {
		t.Error("IsAvailable should return false for empty path")
	}
}
