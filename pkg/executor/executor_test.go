package executor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecutor_Run_Success(t *testing.T) {
	e := New()
	ctx := context.Background()

	result := e.Run(ctx, "", "echo hello")

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	if result.Output != "hello\n" {
		t.Fatalf("expected 'hello\\n', got %q", result.Output)
	}
}

func TestExecutor_Run_Failure(t *testing.T) {
	e := New()
	ctx := context.Background()

	result := e.Run(ctx, "", "false") // Unix command that exits with 1

	if result.Success {
		t.Fatal("expected failure for false command, got success")
	}

	if result.Error == nil {
		t.Fatal("expected non-nil error for failed command")
	}
}

func TestExecutor_Run_WithWorkdir(t *testing.T) {
	e := New()
	ctx := context.Background()

	// Use /tmp as a safe directory that always exists
	result := e.Run(ctx, "/tmp", "pwd")

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "/tmp") {
		t.Fatalf("expected output to contain /tmp, got %q", result.Output)
	}
}

func TestExecutor_Run_EmptyCommand(t *testing.T) {
	e := New()
	ctx := context.Background()

	result := e.Run(ctx, "", "")

	if result.Success {
		t.Fatal("expected failure for empty command, got success")
	}

	if result.Error == nil {
		t.Fatal("expected non-nil error for empty command")
	}
}

func TestExecutor_Run_ContextCancellation(t *testing.T) {
	e := New()
	e.SetTimeout(100 * time.Millisecond)

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := e.Run(ctx, "", "sleep 1")

	if result.Success {
		t.Fatal("expected failure for canceled context, got success")
	}
}

func TestExecutor_RunTimeout(t *testing.T) {
	e := New()
	e.SetTimeout(10 * time.Millisecond)
	ctx := context.Background()

	// This should timeout since sleep 1 takes longer than 10ms
	result := e.Run(ctx, "", "sleep 1")

	if result.Success {
		t.Fatal("expected failure due to timeout, got success")
	}

	// The error should be a context deadline exceeded
	if result.Error == nil {
		t.Fatal("expected non-nil error for timeout")
	}
}

func TestExecutor_VerboseLogging_Success(t *testing.T) {
	e := New()
	e.SetVerboseLevel(1)
	ctx := context.Background()

	result := e.Run(ctx, "", "echo hello")

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	// Verbose output should include completion status and timing info
	if !strings.Contains(result.Output, "[completed successfully after") {
		t.Logf("Output: %q", result.Output)
		t.Fatal("expected verbose output to contain completion status and timing info, but it didn't")
	}
}

func TestExecutor_VerboseLogging_Failure(t *testing.T) {
	e := New()
	e.SetVerboseLevel(1)
	ctx := context.Background()

	result := e.Run(ctx, "", "false")

	if result.Success {
		t.Fatal("expected failure for false command, got success")
	}

	// Verbose output should include error status and duration info
	output := result.Output
	if !strings.Contains(output, "[exited with error:") {
		t.Logf("Output: %q", output)
		t.Fatal("expected verbose output to contain error status info, but it didn't")
	}

	if !strings.Contains(output, "after") {
		t.Logf("Output: %q", output)
		t.Fatal("expected verbose output to contain timing info, but it didn't")
	}
}

func TestExecutor_VerboseLogging_LevelZero(t *testing.T) {
	e := New()
	e.SetVerboseLevel(0)
	ctx := context.Background()

	result := e.Run(ctx, "", "echo hello")

	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	// Non-verbose mode should NOT include completion status info
	if strings.Contains(result.Output, "[completed successfully") {
		t.Logf("Output: %q", result.Output)
		t.Fatal("expected non-verbose output without completion status info, but it was present")
	}
}

func TestExecutor_RunParallel(t *testing.T) {
	e := New()
	ctx := context.Background()

	hooks := []HookDefinition{
		{Command: "echo one"},
		{Command: "echo two"},
		{Command: "echo three"},
	}

	results := e.RunParallel(ctx, hooks)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Success {
			t.Fatalf("result %d expected success, got error: %v", i, result.Error)
		}
	}
}

func TestExecutor_GetVerboseLevel(t *testing.T) {
	e := New()

	// Default should be 0
	if level := e.GetVerboseLevel(); level != 0 {
		t.Fatalf("expected default verbose level 0, got %d", level)
	}

	e.SetVerboseLevel(2)
	if level := e.GetVerboseLevel(); level != 2 {
		t.Fatalf("expected verbose level 2, got %d", level)
	}
}
