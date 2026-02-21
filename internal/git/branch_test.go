package git

import "testing"

func TestClient_ListAllBranches(t *testing.T) {
	// This is an integration test that requires a real git repo.
	t.Skip("requires integration test environment")
}
