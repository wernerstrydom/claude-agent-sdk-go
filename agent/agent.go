package agent

import (
	"context"
	"encoding/json"
	"sync"
)

// Agent represents a Claude Code session.
type Agent struct {
	cfg       *config
	proc      *process
	bridge    *bridge
	sessionID string
	mu        sync.Mutex
	closed    bool
}

// RunOption configures a single Run() call.
type RunOption func(*runConfig)

// runConfig holds per-run configuration.
type runConfig struct{}

// userMessage is the JSON structure for sending prompts.
type userMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// New creates a new Agent with the given options.
func New(ctx context.Context, opts ...Option) (*Agent, error) {
	cfg := newConfig(opts...)

	proc, err := startProcess(ctx, cfg)
	if err != nil {
		return nil, err
	}

	bridge := newBridge(proc.reader())

	// Wait for SystemInit message to get session ID
	var sessionID string
	select {
	case msg, ok := <-bridge.recv():
		if !ok {
			proc.close()
			if err := bridge.error(); err != nil {
				return nil, &StartError{Reason: "failed to read init message", Cause: err}
			}
			return nil, &StartError{Reason: "process closed before init"}
		}
		if init, ok := msg.(*SystemInit); ok {
			sessionID = init.SessionID
		}
	case <-ctx.Done():
		bridge.close()
		proc.close()
		return nil, &StartError{Reason: "context cancelled waiting for init", Cause: ctx.Err()}
	}

	return &Agent{
		cfg:       cfg,
		proc:      proc,
		bridge:    bridge,
		sessionID: sessionID,
	}, nil
}

// Run sends a prompt and waits for the result.
func (a *Agent) Run(ctx context.Context, prompt string, opts ...RunOption) (*Result, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil, &TaskError{SessionID: a.sessionID, Message: "agent is closed"}
	}

	// Send prompt as JSON
	msg := userMessage{
		Type:    "user",
		Content: prompt,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, &TaskError{SessionID: a.sessionID, Message: "failed to marshal prompt"}
	}
	data = append(data, '\n')

	if err := a.proc.write(data); err != nil {
		return nil, &TaskError{SessionID: a.sessionID, Message: "failed to write prompt: " + err.Error()}
	}

	// Collect messages until Result
	for {
		select {
		case msg, ok := <-a.bridge.recv():
			if !ok {
				if err := a.bridge.error(); err != nil {
					return nil, err
				}
				return nil, &TaskError{SessionID: a.sessionID, Message: "stream closed without result"}
			}

			switch m := msg.(type) {
			case *Result:
				return m, nil
			case *Error:
				return nil, m.Err
			}
			// Continue collecting other message types

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// SessionID returns the session identifier.
func (a *Agent) SessionID() string {
	return a.sessionID
}

// Close terminates the agent and releases resources.
func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	a.bridge.close()
	return a.proc.close()
}
