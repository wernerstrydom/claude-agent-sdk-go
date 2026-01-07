package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWithInvalidCLIPath(t *testing.T) {
	ctx := context.Background()
	_, err := New(ctx, CLIPath("/nonexistent/claude"))
	if err == nil {
		t.Error("New() should return error for invalid CLI path")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	// Create a fake CLI that outputs init and waits
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"test-123"}'
sleep 10
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Close multiple times should not panic or error
	if err := a.Close(); err != nil {
		t.Errorf("first Close() error = %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("third Close() error = %v", err)
	}
}

func TestSessionID(t *testing.T) {
	// Create a fake CLI that outputs init
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"sess-abc-123"}'
sleep 10
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer a.Close()

	if a.SessionID() != "sess-abc-123" {
		t.Errorf("SessionID() = %q, want %q", a.SessionID(), "sess-abc-123")
	}
}

func TestRunReturnsResult(t *testing.T) {
	// Create a fake CLI that reads input and returns a result
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"test-run"}'
read line
echo '{"type":"assistant","content":[{"type":"text","text":"Hello!"}]}'
echo '{"type":"result","result":"Task done","num_turns":1,"cost_usd":0.001}'
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer a.Close()

	result, err := a.Run(ctx, "test prompt")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if result.ResultText != "Task done" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "Task done")
	}

	if result.NumTurns != 1 {
		t.Errorf("NumTurns = %d, want 1", result.NumTurns)
	}
}

func TestRunAfterClose(t *testing.T) {
	// Create a fake CLI
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"test-closed"}'
sleep 10
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	a.Close()

	_, err = a.Run(ctx, "test")
	if err == nil {
		t.Error("Run() after Close() should return error")
	}
}

func TestContextCancellation(t *testing.T) {
	// Create a fake CLI that waits forever
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"test-cancel"}'
read line
sleep 60
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer a.Close()

	// Create a context that cancels quickly
	runCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = a.Run(runCtx, "test prompt")
	if err == nil {
		t.Error("Run() should return error when context is cancelled")
	}
}

func TestNewContextCancellation(t *testing.T) {
	// Create a fake CLI that takes too long to output init
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
sleep 60
echo '{"type":"system","subtype":"init","session_id":"test"}'
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := New(ctx, CLIPath(fakeClaude))
	if err == nil {
		t.Error("New() should return error when context is cancelled")
	}
}
