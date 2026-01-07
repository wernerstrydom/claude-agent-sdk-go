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
	// Create a fake CLI that waits for input (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
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
	// Create a fake CLI that outputs init after receiving first message (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// In stream-json mode, init is sent after receiving first message
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"sess-abc-123"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello"}]}}'
echo '{"type":"result","result":"Done","num_turns":1,"total_cost_usd":0.001}'
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

	// Session ID is empty before first message
	if a.SessionID() != "" {
		t.Errorf("SessionID() before message = %q, want empty", a.SessionID())
	}

	// Send a message to trigger init
	for range a.Stream(ctx, "test") {
	}

	// Session ID should be populated after first message exchange
	if a.SessionID() != "sess-abc-123" {
		t.Errorf("SessionID() after message = %q, want %q", a.SessionID(), "sess-abc-123")
	}
}

func TestRunReturnsResult(t *testing.T) {
	// Create a fake CLI that reads input and returns a result (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"test-run"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello!"}]}}'
echo '{"type":"result","result":"Task done","num_turns":1,"total_cost_usd":0.001}'
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
	// Create a fake CLI (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
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
	// Create a fake CLI that waits forever (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"test-cancel"}'
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

func TestNewCreatesAgent(t *testing.T) {
	// In stream-json mode, New() no longer waits for init.
	// It just starts the process and returns immediately.
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
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

	// Agent should be created even though CLI hasn't sent init yet
	if a == nil {
		t.Error("New() returned nil agent")
	}
}

func TestStreamReturnsMessages(t *testing.T) {
	// Create a fake CLI that outputs multiple messages (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"test-stream"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello!"}]}}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":" World!"}]}}'
echo '{"type":"result","result":"Done","num_turns":1,"total_cost_usd":0.001}'
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

	var messages []Message
	for msg := range a.Stream(ctx, "test") {
		messages = append(messages, msg)
	}

	if len(messages) != 3 {
		t.Errorf("got %d messages, want 3", len(messages))
	}

	// First two should be Text
	if _, ok := messages[0].(*Text); !ok {
		t.Errorf("message 0: expected *Text, got %T", messages[0])
	}
	if _, ok := messages[1].(*Text); !ok {
		t.Errorf("message 1: expected *Text, got %T", messages[1])
	}

	// Last should be Result
	if _, ok := messages[2].(*Result); !ok {
		t.Errorf("message 2: expected *Result, got %T", messages[2])
	}
}

func TestStreamAllMessageTypes(t *testing.T) {
	// Create a fake CLI that outputs various message types (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"test-types"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Thinking..."}]}}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Let me analyze","signature":"sig1"}]}}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"Bash","input":{"command":"ls"}}]}}'
echo '{"type":"result","result":"Complete","num_turns":1,"total_cost_usd":0.002}'
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

	var (
		hasText     bool
		hasThinking bool
		hasToolUse  bool
		hasResult   bool
	)

	for msg := range a.Stream(ctx, "test") {
		switch msg.(type) {
		case *Text:
			hasText = true
		case *Thinking:
			hasThinking = true
		case *ToolUse:
			hasToolUse = true
		case *Result:
			hasResult = true
		}
	}

	if !hasText {
		t.Error("stream should contain Text message")
	}
	if !hasThinking {
		t.Error("stream should contain Thinking message")
	}
	if !hasToolUse {
		t.Error("stream should contain ToolUse message")
	}
	if !hasResult {
		t.Error("stream should contain Result message")
	}
}

func TestErrReturnsNilOnSuccess(t *testing.T) {
	// Create a fake CLI that completes successfully (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"test-err-nil"}'
echo '{"type":"result","result":"OK","num_turns":1,"total_cost_usd":0.001}'
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

	// Drain the stream
	for range a.Stream(ctx, "test") {
	}

	if err := a.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func TestStreamContextCancellation(t *testing.T) {
	// Create a fake CLI that waits forever (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"test-stream-cancel"}'
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
	streamCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	messageCount := 0
	for range a.Stream(streamCtx, "test") {
		messageCount++
	}

	// Stream should close due to context cancellation
	// We don't expect many messages since it times out
	if messageCount > 10 {
		t.Errorf("got %d messages, expected stream to close quickly", messageCount)
	}
}

func TestStreamAfterClose(t *testing.T) {
	// Create a fake CLI (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
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

	// Stream after close should return closed channel
	messageCount := 0
	for range a.Stream(ctx, "test") {
		messageCount++
	}

	if messageCount != 0 {
		t.Errorf("got %d messages after Close(), want 0", messageCount)
	}
}
