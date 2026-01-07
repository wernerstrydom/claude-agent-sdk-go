package agent

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// AuditEvent represents an event that occurred during agent execution.
// Events are emitted at key points: session start/end, messages, hooks, and errors.
type AuditEvent struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"session_id"`
	Type      string    `json:"type"`
	Data      any       `json:"data,omitempty"`
}

// AuditHandler is a function that receives audit events.
// Handlers are called synchronously. If a handler panics, the panic
// is recovered and the event is skipped for that handler.
type AuditHandler func(AuditEvent)

// auditor manages audit handlers and event emission.
type auditor struct {
	handlers []AuditHandler
	mu       sync.RWMutex
}

// newAuditor creates a new auditor with the given handlers.
func newAuditor(handlers []AuditHandler) *auditor {
	if len(handlers) == 0 {
		return nil
	}
	return &auditor{handlers: handlers}
}

// emit sends an event to all handlers.
// Panics in handlers are recovered to prevent one bad handler from
// affecting others or crashing the agent.
func (a *auditor) emit(sessionID, eventType string, data any) {
	if a == nil || len(a.handlers) == 0 {
		return
	}

	event := AuditEvent{
		Time:      time.Now(),
		SessionID: sessionID,
		Type:      eventType,
		Data:      data,
	}

	a.mu.RLock()
	handlers := a.handlers
	a.mu.RUnlock()

	for _, h := range handlers {
		func() {
			defer func() {
				// Recover from panics in handlers
				_ = recover()
			}()
			h(event)
		}()
	}
}

// AuditWriterHandler creates an AuditHandler that writes JSONL to the given writer.
// Each event is written as a single JSON line.
func AuditWriterHandler(w io.Writer) AuditHandler {
	var mu sync.Mutex
	enc := json.NewEncoder(w)

	return func(e AuditEvent) {
		mu.Lock()
		defer mu.Unlock()
		_ = enc.Encode(e) // Best effort - ignore write errors
	}
}

// AuditFileHandler creates an AuditHandler that writes JSONL to a file.
// It returns the handler and a cleanup function that should be called
// to close the file when the agent is done.
//
// Example:
//
//	handler, cleanup, err := AuditFileHandler("audit.jsonl")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	a, _ := agent.New(ctx, agent.AuditHandler(handler))
//	defer cleanup()
func AuditFileHandler(path string) (AuditHandler, func() error, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644) // #nosec G302,G304 -- Path provided by caller; 0644 is intentional for log files
	if err != nil {
		return nil, nil, err
	}

	handler := AuditWriterHandler(f)
	cleanup := func() error {
		return f.Close()
	}

	return handler, cleanup, nil
}
