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
