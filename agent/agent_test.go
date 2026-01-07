package agent

import (
	"context"
	"errors"
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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	result, err := a.Run(ctx, "test prompt")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// result is non-nil when err is nil (enforced by Run's contract)
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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_ = a.Close() // Intentionally close to test behavior

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

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
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_ = a.Close() // Intentionally close to test behavior

	// Stream after close should return closed channel
	messageCount := 0
	for range a.Stream(ctx, "test") {
		messageCount++
	}

	if messageCount != 0 {
		t.Errorf("got %d messages after Close(), want 0", messageCount)
	}
}

func TestMaxTurnsEnforcement(t *testing.T) {
	// Create a fake CLI that returns 6 turns in a single run, exceeding max of 5
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"max-turns-test"}'
echo '{"type":"result","result":"Done","num_turns":6,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude), MaxTurns(5))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	// Run should complete but return error since 6 turns > 5 max
	result, err := a.Run(ctx, "test")

	// We should get both a result AND an error since the run completed but exceeded limit
	var maxErr *MaxTurnsError
	if !errors.As(err, &maxErr) {
		t.Fatalf("expected MaxTurnsError, got %T: %v", err, err)
	}
	if maxErr.MaxAllowed != 5 {
		t.Errorf("MaxAllowed = %d, want 5", maxErr.MaxAllowed)
	}
	if maxErr.Turns != 6 {
		t.Errorf("Turns = %d, want 6", maxErr.Turns)
	}
	if result == nil {
		t.Error("expected result to be returned along with MaxTurnsError")
	}
}

func TestMaxTurnsPreRunCheck(t *testing.T) {
	// Test that pre-run check blocks when already at limit
	// We manually set totalTurns to simulate having reached the limit
	a := &Agent{
		cfg:        &config{maxTurns: 5},
		totalTurns: 5, // Already at limit
		sessionID:  "test-session",
	}

	ctx := context.Background()
	_, err := a.Run(ctx, "should be blocked")

	var maxErr *MaxTurnsError
	if !errors.As(err, &maxErr) {
		t.Fatalf("expected MaxTurnsError on pre-run check, got %T: %v", err, err)
	}
	if maxErr.Turns != 5 {
		t.Errorf("Turns = %d, want 5 (from pre-check)", maxErr.Turns)
	}
	if maxErr.MaxAllowed != 5 {
		t.Errorf("MaxAllowed = %d, want 5", maxErr.MaxAllowed)
	}
}

func TestMaxTurnsRunOverride(t *testing.T) {
	// Create a fake CLI that returns 3 turns per run
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"max-turns-override"}'
echo '{"type":"result","result":"Done","num_turns":3,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude), MaxTurns(10)) // Agent-level: 10
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	// Run with override: max 2 turns (should trigger error since result has 3)
	_, err = a.Run(ctx, "test", MaxTurnsRun(2))

	var maxErr *MaxTurnsError
	if !errors.As(err, &maxErr) {
		t.Fatalf("expected MaxTurnsError with per-run override, got %T: %v", err, err)
	}
	if maxErr.MaxAllowed != 2 {
		t.Errorf("MaxAllowed = %d, want 2 (per-run override)", maxErr.MaxAllowed)
	}
}

func TestTimeoutRunOptionEnforcement(t *testing.T) {
	// Create a fake CLI that waits briefly then outputs
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// Script that outputs init, then waits. The timeout should trigger error.
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"timeout-test"}'
sleep 0.5
echo '{"type":"result","result":"Done","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	// Timeout of 100ms should cause context cancellation before the 500ms sleep completes
	_, err = a.Run(ctx, "test", Timeout(100*time.Millisecond))

	// We expect an error (context deadline or no result)
	// Note: The subprocess continues running, but Run() returns early
	if err == nil {
		t.Error("expected error from timeout, got nil")
	}
}

func TestEffectiveMaxTurns(t *testing.T) {
	// Create a minimal agent for testing
	a := &Agent{
		cfg: &config{maxTurns: 10},
	}

	// No run config - use agent default
	if got := a.effectiveMaxTurns(nil); got != 10 {
		t.Errorf("effectiveMaxTurns(nil) = %d, want 10", got)
	}

	// Run config with 0 - use agent default
	rc := &runConfig{maxTurns: 0}
	if got := a.effectiveMaxTurns(rc); got != 10 {
		t.Errorf("effectiveMaxTurns(rc with 0) = %d, want 10", got)
	}

	// Run config with value - override
	rc = &runConfig{maxTurns: 5}
	if got := a.effectiveMaxTurns(rc); got != 5 {
		t.Errorf("effectiveMaxTurns(rc with 5) = %d, want 5", got)
	}
}
