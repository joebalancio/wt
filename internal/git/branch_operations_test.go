package git

import (
	"testing"
)

func TestClient_IsBranchMerged(t *testing.T) {
	// This is an integration test that requires a real git repo
	// Unit testing will be done via the mock in service_test.go
	t.Skip("requires integration test environment")
}

func TestClient_RemoteBranchExists(t *testing.T) {
	// This is an integration test that requires a real git repo
	// Unit testing will be done via the mock in service_test.go
	t.Skip("requires integration test environment")
}

func TestClient_DeleteRemoteBranch(t *testing.T) {
	// This is an integration test that requires a real git repo
	// Unit testing will be done via the mock in service_test.go
	t.Skip("requires integration test environment")
}
