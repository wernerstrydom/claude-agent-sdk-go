package agent

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAuditorEmit(t *testing.T) {
	var received []AuditEvent
	var mu sync.Mutex

	handler := func(e AuditEvent) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, e)
	}

	a := newAuditor([]AuditHandler{handler})
	a.emit("sess-123", "test.event", map[string]any{"key": "value"})

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}

	e := received[0]
	if e.SessionID != "sess-123" {
		t.Errorf("expected session ID 'sess-123', got %q", e.SessionID)
	}
	if e.Type != "test.event" {
		t.Errorf("expected type 'test.event', got %q", e.Type)
	}
	data, ok := e.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map[string]any, got %T", e.Data)
	}
	if data["key"] != "value" {
		t.Errorf("expected data[\"key\"] = \"value\", got %v", data["key"])
	}
	if e.Time.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestAuditorEmit_NilAuditor(t *testing.T) {
	var a *auditor
	// Should not panic
	a.emit("sess-123", "test.event", nil)
}

func TestAuditorEmit_NoHandlers(t *testing.T) {
	a := newAuditor(nil)
	// Should not panic and auditor should be nil
	if a != nil {
		t.Error("expected nil auditor for no handlers")
	}
	a.emit("sess-123", "test.event", nil) // Should not panic
}

func TestAuditorEmit_MultipleHandlers(t *testing.T) {
	var count1, count2 int
	var mu sync.Mutex

	handler1 := func(e AuditEvent) {
		mu.Lock()
		defer mu.Unlock()
		count1++
	}
	handler2 := func(e AuditEvent) {
		mu.Lock()
		defer mu.Unlock()
		count2++
	}

	a := newAuditor([]AuditHandler{handler1, handler2})
	a.emit("sess-123", "test.event", nil)

	mu.Lock()
	defer mu.Unlock()

	if count1 != 1 {
		t.Errorf("expected handler1 called once, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("expected handler2 called once, got %d", count2)
	}
}

func TestAuditorEmit_HandlerPanicRecovery(t *testing.T) {
	var count int
	var mu sync.Mutex

	panicHandler := func(e AuditEvent) {
		panic("test panic")
	}
	normalHandler := func(e AuditEvent) {
		mu.Lock()
		defer mu.Unlock()
		count++
	}

	a := newAuditor([]AuditHandler{panicHandler, normalHandler})

	// Should not panic
	a.emit("sess-123", "test.event", nil)

	mu.Lock()
	defer mu.Unlock()

	// Second handler should still be called after first one panics
	if count != 1 {
		t.Errorf("expected second handler to be called, got count %d", count)
	}
}

func TestAuditWriterHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := AuditWriterHandler(&buf)

	// Emit an event
	handler(AuditEvent{
		Time:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		SessionID: "sess-abc",
		Type:      "test.event",
		Data:      map[string]any{"foo": "bar"},
	})

	// Parse the output
	var parsed AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if parsed.SessionID != "sess-abc" {
		t.Errorf("expected session ID 'sess-abc', got %q", parsed.SessionID)
	}
	if parsed.Type != "test.event" {
		t.Errorf("expected type 'test.event', got %q", parsed.Type)
	}
}

func TestAuditWriterHandler_MultipleEvents(t *testing.T) {
	var buf bytes.Buffer
	handler := AuditWriterHandler(&buf)

	// Emit multiple events
	for i := 0; i < 3; i++ {
		handler(AuditEvent{
			Time:      time.Now(),
			SessionID: "sess-multi",
			Type:      "test.event",
			Data:      map[string]any{"index": i},
		})
	}

	// Count the lines (JSONL format)
	lines := bytes.Count(buf.Bytes(), []byte("\n"))
	if lines != 3 {
		t.Errorf("expected 3 lines, got %d", lines)
	}
}

func TestAuditFileHandler(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	handler, cleanup, err := AuditFileHandler(path)
	if err != nil {
		t.Fatalf("failed to create file handler: %v", err)
	}

	// Emit an event
	handler(AuditEvent{
		Time:      time.Now(),
		SessionID: "sess-file",
		Type:      "test.event",
		Data:      map[string]any{"file": true},
	})

	// Close the file
	if err := cleanup(); err != nil {
		t.Fatalf("failed to close file: %v", err)
	}

	// Read and verify
	data := mustReadFile(t, path)

	var parsed AuditEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse file content: %v", err)
	}

	if parsed.SessionID != "sess-file" {
		t.Errorf("expected session ID 'sess-file', got %q", parsed.SessionID)
	}
}

func TestAuditFileHandler_AppendMode(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	// Write first event
	handler1, cleanup1, err := AuditFileHandler(path)
	if err != nil {
		t.Fatalf("failed to create first file handler: %v", err)
	}
	handler1(AuditEvent{SessionID: "sess-1", Type: "first"})
	if err := cleanup1(); err != nil {
		t.Fatalf("failed to close first file: %v", err)
	}

	// Write second event (appending)
	handler2, cleanup2, err := AuditFileHandler(path)
	if err != nil {
		t.Fatalf("failed to create second file handler: %v", err)
	}
	handler2(AuditEvent{SessionID: "sess-2", Type: "second"})
	if err := cleanup2(); err != nil {
		t.Fatalf("failed to close second file: %v", err)
	}

	// Verify both events are present
	data := mustReadFile(t, path)

	lines := bytes.Count(data, []byte("\n"))
	if lines != 2 {
		t.Errorf("expected 2 lines, got %d", lines)
	}
}

func TestAuditFileHandler_InvalidPath(t *testing.T) {
	_, _, err := AuditFileHandler("/nonexistent/directory/audit.jsonl")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestAuditEventTypes(t *testing.T) {
	// Verify expected event types are strings
	types := []string{
		"session.start",
		"session.init",
		"session.end",
		"message.prompt",
		"message.text",
		"message.thinking",
		"message.tool_use",
		"message.tool_result",
		"message.result",
		"hook.pre_tool_use",
		"error",
	}

	for _, eventType := range types {
		if eventType == "" {
			t.Error("event type should not be empty")
		}
	}
}

func TestAuditOption(t *testing.T) {
	var received []AuditEvent
	var mu sync.Mutex

	handler := func(e AuditEvent) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, e)
	}

	cfg := newConfig(Audit(handler))

	if len(cfg.auditHandlers) != 1 {
		t.Errorf("expected 1 audit handler, got %d", len(cfg.auditHandlers))
	}
}

func TestAuditOption_Multiple(t *testing.T) {
	handler1 := func(e AuditEvent) {}
	handler2 := func(e AuditEvent) {}

	cfg := newConfig(
		Audit(handler1),
		Audit(handler2),
	)

	if len(cfg.auditHandlers) != 2 {
		t.Errorf("expected 2 audit handlers, got %d", len(cfg.auditHandlers))
	}
}

func TestAuditToFileOption(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.jsonl")

	cfg := newConfig(AuditToFile(path))

	if len(cfg.auditHandlers) != 1 {
		t.Errorf("expected 1 audit handler, got %d", len(cfg.auditHandlers))
	}
	if len(cfg.auditCleanup) != 1 {
		t.Errorf("expected 1 audit cleanup function, got %d", len(cfg.auditCleanup))
	}

	// Cleanup
	for _, cleanup := range cfg.auditCleanup {
		if err := cleanup(); err != nil {
			t.Errorf("cleanup error: %v", err)
		}
	}
}

func TestAuditToFileOption_InvalidPath(t *testing.T) {
	cfg := newConfig(AuditToFile("/nonexistent/directory/audit.jsonl"))

	// Error is stored in schemaError for later reporting
	if cfg.schemaError == nil {
		t.Error("expected error for invalid path")
	}
}
