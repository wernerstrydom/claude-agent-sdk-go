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
	hookChain *hookChain
	sessionID string
	mu        sync.Mutex
	closed    bool
}

// RunOption configures a single Run() call.
type RunOption func(*runConfig)

// runConfig holds per-run configuration.
type runConfig struct{}

// userMessage is the JSON structure for sending prompts to Claude CLI.
type userMessage struct {
	Type    string      `json:"type"`
	Message userContent `json:"message"`
}

// userContent represents the message content structure.
type userContent struct {
	Role    string            `json:"role"`
	Content []userContentItem `json:"content"`
}

// userContentItem represents a content block in the message.
type userContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// New creates a new Agent with the given options.
// Note: With stream-json input format, the CLI waits for the first message
// before outputting anything (including init). The session ID is captured
// lazily when the first message is sent.
func New(ctx context.Context, opts ...Option) (*Agent, error) {
	cfg := newConfig(opts...)

	proc, err := startProcess(ctx, cfg)
	if err != nil {
		return nil, err
	}

	bridge := newBridge(proc.reader())

	// Create hook chain from config
	chain := newHookChain(cfg.preToolUseHooks)

	return &Agent{
		cfg:       cfg,
		proc:      proc,
		bridge:    bridge,
		hookChain: chain,
	}, nil
}

// Stream sends a prompt and returns a channel of messages.
// The channel closes when the result is received or an error occurs.
// Call Err() after the channel closes to check for errors.
func (a *Agent) Stream(ctx context.Context, prompt string, opts ...RunOption) <-chan Message {
	out := make(chan Message, 32)

	a.mu.Lock()

	if a.closed {
		a.mu.Unlock()
		close(out)
		return out
	}

	// Send prompt as JSON
	msg := userMessage{
		Type: "user",
		Message: userContent{
			Role: "user",
			Content: []userContentItem{
				{Type: "text", Text: prompt},
			},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		a.mu.Unlock()
		close(out)
		return out
	}
	data = append(data, '\n')

	if err := a.proc.write(data); err != nil {
		a.mu.Unlock()
		close(out)
		return out
	}

	a.mu.Unlock()

	// Forward messages until Result or context cancellation
	go func() {
		defer close(out)
		for {
			select {
			case msg, ok := <-a.bridge.recv():
				if !ok {
					return
				}

				// Capture session ID from SystemInit (sent after first message with stream-json)
				if init, isInit := msg.(*SystemInit); isInit {
					a.mu.Lock()
					if a.sessionID == "" {
						a.sessionID = init.SessionID
					}
					a.mu.Unlock()
					// Don't send SystemInit to caller
					continue
				}

				// Handle control requests internally
				if ctrlReq, isCtrl := msg.(*ControlRequestMsg); isCtrl {
					req := &ControlRequest{
						RequestID: ctrlReq.RequestID,
						Type:      ctrlReq.Type,
						Tool: &ToolCall{
							Name:  ctrlReq.ToolName,
							Input: ctrlReq.ToolInput,
						},
					}
					// Ignore error - best effort response
					_ = a.handleControlRequest(req)
					continue
				}

				select {
				case out <- msg:
				case <-ctx.Done():
					return
				}
				// Stop after Result
				if _, isResult := msg.(*Result); isResult {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}

// Err returns any error that occurred during streaming.
// Call this after the Stream() channel closes.
func (a *Agent) Err() error {
	return a.bridge.error()
}

// Run sends a prompt and waits for the result.
func (a *Agent) Run(ctx context.Context, prompt string, opts ...RunOption) (*Result, error) {
	var result *Result
	for msg := range a.Stream(ctx, prompt, opts...) {
		switch m := msg.(type) {
		case *Result:
			result = m
		case *Error:
			return nil, m.Err
		}
	}
	if err := a.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, &TaskError{SessionID: a.sessionID, Message: "no result received"}
	}
	return result, nil
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
