package agent

import (
	"context"
	"encoding/json"
	"sync"
)

// Agent represents a Claude Code session.
type Agent struct {
	cfg               *config
	proc              *process
	bridge            *bridge
	hookChain         *hookChain
	postToolUseChain  *postToolUseChain
	preCompactChain   *preCompactChain
	subagentStopChain *subagentStopChain
	promptSubmitChain *promptSubmitChain
	auditor           *auditor
	sessionID         string
	totalTurns        int     // Cumulative turns across all Run() calls
	totalCost         float64 // Cumulative cost across all Run() calls
	stopReason        StopReason
	pendingToolCalls  map[string]*ToolCall // Tool calls awaiting results
	mu                sync.Mutex
	closed            bool
}

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

	// Check for schema errors (deferred from WithSchema option)
	if cfg.schemaError != nil {
		return nil, cfg.schemaError
	}

	proc, err := startProcess(ctx, cfg)
	if err != nil {
		return nil, err
	}

	bridge := newBridge(proc.reader())

	// Create hook chains from config
	chain := newHookChain(cfg.preToolUseHooks)
	postChain := newPostToolUseChain(cfg.postToolUseHooks)
	preCompact := newPreCompactChain(cfg.preCompactHooks)
	subagentStop := newSubagentStopChain(cfg.subagentStopHooks)
	promptSubmit := newPromptSubmitChain(cfg.userPromptSubmitHooks)

	// Create auditor from config
	aud := newAuditor(cfg.auditHandlers)

	agent := &Agent{
		cfg:               cfg,
		proc:              proc,
		bridge:            bridge,
		hookChain:         chain,
		postToolUseChain:  postChain,
		preCompactChain:   preCompact,
		subagentStopChain: subagentStop,
		promptSubmitChain: promptSubmit,
		auditor:           aud,
		stopReason:        StopCompleted, // Default to completed
		pendingToolCalls:  make(map[string]*ToolCall),
	}

	// Emit session.start event (sessionID captured later)
	agent.auditor.emit("", "session.start", nil)

	return agent, nil
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

	// Call UserPromptSubmit hooks before sending
	sessionID := a.sessionID
	turn := a.totalTurns + 1
	a.mu.Unlock()

	finalPrompt, metadata := a.callPromptSubmitHooks(prompt, sessionID, turn)

	a.mu.Lock()
	// Send prompt as JSON
	msg := userMessage{
		Type: "user",
		Message: userContent{
			Role: "user",
			Content: []userContentItem{
				{Type: "text", Text: finalPrompt},
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

	// Emit prompt event
	a.auditor.emit(a.sessionID, "message.prompt", map[string]any{
		"prompt":          prompt,
		"final_prompt":    finalPrompt,
		"prompt_modified": finalPrompt != prompt,
		"prompt_metadata": metadata,
	})

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
					sessionID := a.sessionID
					a.mu.Unlock()
					// Emit session.init event
					a.auditor.emit(sessionID, "session.init", map[string]any{
						"transcript_path": init.TranscriptPath,
						"tools":           init.Tools,
						"mcp_servers":     init.MCPServers,
					})
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
					_ = a.handleControlRequest(ctx, req)
					continue
				}

				// Handle compact events
				if compact, isCompact := msg.(*CompactMsg); isCompact {
					a.handleCompactEvent(compact)
					continue
				}

				// Handle subagent result events
				if subagent, isSubagent := msg.(*SubagentResultMsg); isSubagent {
					a.handleSubagentStopEvent(subagent)
					continue
				}

				// Track pending tool calls and call PostToolUse hooks
				a.processMessageHooks(msg)

				// Emit message events based on type
				a.emitMessageEvent(msg)

				select {
				case out <- msg:
				case <-ctx.Done():
					a.mu.Lock()
					a.stopReason = StopInterrupted
					a.mu.Unlock()
					return
				}
				// Stop after Result
				if _, isResult := msg.(*Result); isResult {
					return
				}
			case <-ctx.Done():
				a.mu.Lock()
				a.stopReason = StopInterrupted
				a.mu.Unlock()
				a.auditor.emit(a.sessionID, "error", map[string]any{
					"error": ctx.Err().Error(),
				})
				return
			}
		}
	}()

	return out
}

// processMessageHooks handles lifecycle hook processing for messages.
// It tracks pending tool calls and calls PostToolUse hooks when results arrive.
func (a *Agent) processMessageHooks(msg Message) {
	switch m := msg.(type) {
	case *ToolUse:
		// Track pending tool call for later PostToolUse hook
		a.mu.Lock()
		a.pendingToolCalls[m.ID] = &ToolCall{
			Name:  m.Name,
			Input: m.Input,
		}
		a.mu.Unlock()

	case *ToolResult:
		// Find the pending tool call
		a.mu.Lock()
		tc, found := a.pendingToolCalls[m.ToolUseID]
		if found {
			delete(a.pendingToolCalls, m.ToolUseID)
		}
		a.mu.Unlock()

		if found {
			// Build result context
			resultCtx := &ToolResultContext{
				ToolUseID: m.ToolUseID,
				Content:   m.Content,
				IsError:   m.IsError,
				Duration:  m.Duration,
			}

			// Call PostToolUse hooks
			a.postToolUseChain.evaluate(tc, resultCtx)

			// Emit audit event
			a.auditor.emit(a.sessionID, "hook.post_tool_use", map[string]any{
				"tool":        tc.Name,
				"input":       tc.Input,
				"is_error":    resultCtx.IsError,
				"duration":    resultCtx.Duration.String(),
				"tool_use_id": resultCtx.ToolUseID,
			})
		}

	case *Result:
		// Accumulate cost
		a.mu.Lock()
		a.totalCost += m.CostUSD
		a.mu.Unlock()
	}
}

// emitMessageEvent emits an audit event for the given message.
func (a *Agent) emitMessageEvent(msg Message) {
	switch m := msg.(type) {
	case *Text:
		a.auditor.emit(a.sessionID, "message.text", map[string]any{
			"text": m.Text,
		})
	case *Thinking:
		a.auditor.emit(a.sessionID, "message.thinking", map[string]any{
			"thinking": m.Thinking,
		})
	case *ToolUse:
		a.auditor.emit(a.sessionID, "message.tool_use", map[string]any{
			"id":    m.ID,
			"name":  m.Name,
			"input": m.Input,
		})
	case *ToolResult:
		a.auditor.emit(a.sessionID, "message.tool_result", map[string]any{
			"tool_use_id": m.ToolUseID,
			"is_error":    m.IsError,
			"duration":    m.Duration.String(),
		})
	case *Result:
		a.auditor.emit(a.sessionID, "message.result", map[string]any{
			"result_text":    m.ResultText,
			"num_turns":      m.NumTurns,
			"cost_usd":       m.CostUSD,
			"duration_total": m.DurationTotal.String(),
			"duration_api":   m.DurationAPI.String(),
			"is_error":       m.IsError,
		})
	case *Error:
		a.auditor.emit(a.sessionID, "error", map[string]any{
			"error": m.Err.Error(),
		})
	}
}

// Err returns any error that occurred during streaming.
// Call this after the Stream() channel closes.
func (a *Agent) Err() error {
	return a.bridge.error()
}

// Run sends a prompt and waits for the result.
func (a *Agent) Run(ctx context.Context, prompt string, opts ...RunOption) (*Result, error) {
	rc := newRunConfig(opts...)

	// Apply timeout if specified
	runCtx := ctx
	if rc.timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, rc.timeout)
		defer cancel()
	}

	// Pre-run check: have we already exceeded max turns?
	maxTurns := a.effectiveMaxTurns(rc)
	a.mu.Lock()
	if maxTurns > 0 && a.totalTurns >= maxTurns {
		sessionID := a.sessionID
		totalTurns := a.totalTurns
		a.stopReason = StopMaxTurns
		a.mu.Unlock()
		return nil, &MaxTurnsError{
			Turns:      totalTurns,
			MaxAllowed: maxTurns,
			SessionID:  sessionID,
		}
	}
	a.mu.Unlock()

	var result *Result
	for msg := range a.Stream(runCtx, prompt) {
		switch m := msg.(type) {
		case *Result:
			result = m
			// Track turns from result
			a.mu.Lock()
			a.totalTurns += m.NumTurns
			a.mu.Unlock()
		case *Error:
			a.mu.Lock()
			a.stopReason = StopError
			a.mu.Unlock()
			return nil, m.Err
		}
	}
	if err := a.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, &TaskError{SessionID: a.sessionID, Message: "no result received"}
	}

	// Post-run check: did this run push us over the limit?
	a.mu.Lock()
	totalTurns := a.totalTurns
	a.mu.Unlock()
	if maxTurns > 0 && totalTurns > maxTurns {
		a.mu.Lock()
		a.stopReason = StopMaxTurns
		a.mu.Unlock()
		return result, &MaxTurnsError{
			Turns:      totalTurns,
			MaxAllowed: maxTurns,
			SessionID:  a.sessionID,
		}
	}

	return result, nil
}

// effectiveMaxTurns returns the max turns to use, preferring run-level over agent-level.
func (a *Agent) effectiveMaxTurns(rc *runConfig) int {
	if rc != nil && rc.maxTurns > 0 {
		return rc.maxTurns
	}
	return a.cfg.maxTurns
}

// SessionID returns the session identifier.
func (a *Agent) SessionID() string {
	return a.sessionID
}

// Close terminates the agent and releases resources.
func (a *Agent) Close() error {
	a.mu.Lock()

	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true

	// Capture session state for hooks
	sessionID := a.sessionID
	totalTurns := a.totalTurns
	totalCost := a.totalCost
	stopReason := a.stopReason

	a.mu.Unlock()

	// Call Stop hooks
	a.callStopHooks(sessionID, stopReason, totalTurns, totalCost)

	// Emit session.end event
	a.auditor.emit(sessionID, "session.end", map[string]any{
		"total_turns": totalTurns,
		"total_cost":  totalCost,
		"stop_reason": string(stopReason),
	})

	a.bridge.close()
	procErr := a.proc.close()

	// Call audit cleanup functions
	for _, cleanup := range a.cfg.auditCleanup {
		_ = cleanup() // Best effort cleanup
	}

	return procErr
}

// callStopHooks calls all registered Stop hooks.
func (a *Agent) callStopHooks(sessionID string, reason StopReason, numTurns int, costUSD float64) {
	if len(a.cfg.stopHooks) == 0 {
		return
	}

	event := &StopEvent{
		SessionID: sessionID,
		Reason:    reason,
		NumTurns:  numTurns,
		CostUSD:   costUSD,
	}

	// Call each hook, recovering from panics
	for _, hook := range a.cfg.stopHooks {
		func() {
			defer func() {
				_ = recover()
			}()
			hook(event)
		}()
	}

	// Emit audit event
	a.auditor.emit(sessionID, "hook.stop", map[string]any{
		"reason":    string(reason),
		"num_turns": numTurns,
		"cost_usd":  costUSD,
	})
}

// RunWithSchema runs a prompt and unmarshals the structured response into ptr.
// The agent must have been created with WithSchema or WithSchemaRaw option.
// The ptr must be a pointer to the same type used in WithSchema.
func (a *Agent) RunWithSchema(ctx context.Context, prompt string, ptr any, opts ...RunOption) (*Result, error) {
	result, err := a.Run(ctx, prompt, opts...)
	if err != nil {
		return nil, err
	}

	// Unmarshal the result into the provided pointer
	if ptr != nil && a.cfg.jsonSchema != "" {
		if err := json.Unmarshal([]byte(result.ResultText), ptr); err != nil {
			return result, &SchemaError{
				Reason: "failed to unmarshal response",
				Cause:  err,
			}
		}
	}

	return result, nil
}

// RunStructured is a convenience function that creates a one-shot agent for
// structured output. It generates a schema from ptr's type, sends the prompt,
// unmarshals the response into ptr, and closes the agent.
//
// Use this for single structured queries. For multiple structured queries
// with the same schema, create an agent with WithSchema for better performance.
//
// Example:
//
//	type Answer struct {
//	    Value int `json:"value" desc:"The numeric answer"`
//	}
//	var answer Answer
//	result, err := agent.RunStructured(ctx, "What is 2+2?", &answer)
func RunStructured(ctx context.Context, prompt string, ptr any, opts ...Option) (*Result, error) {
	// Add WithSchema to options
	allOpts := append([]Option{WithSchema(ptr)}, opts...)

	a, err := New(ctx, allOpts...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = a.Close() // Ignore close error; result already obtained
	}()

	return a.RunWithSchema(ctx, prompt, ptr)
}

// callPromptSubmitHooks runs UserPromptSubmit hooks and returns the final prompt.
func (a *Agent) callPromptSubmitHooks(prompt, sessionID string, turn int) (string, []any) {
	if a.promptSubmitChain == nil || len(a.cfg.userPromptSubmitHooks) == 0 {
		return prompt, nil
	}

	event := &PromptSubmitEvent{
		Prompt:    prompt,
		SessionID: sessionID,
		Turn:      turn,
	}

	finalPrompt, metadata := a.promptSubmitChain.evaluate(event)

	// Emit audit event
	a.auditor.emit(sessionID, "hook.user_prompt_submit", map[string]any{
		"original_prompt": prompt,
		"final_prompt":    finalPrompt,
		"modified":        finalPrompt != prompt,
		"metadata":        metadata,
	})

	return finalPrompt, metadata
}

// handleCompactEvent processes a context compaction event.
func (a *Agent) handleCompactEvent(compact *CompactMsg) {
	if a.preCompactChain == nil || len(a.cfg.preCompactHooks) == 0 {
		return
	}

	a.mu.Lock()
	sessionID := a.sessionID
	a.mu.Unlock()

	event := &PreCompactEvent{
		SessionID:      sessionID,
		Trigger:        compact.Trigger,
		TranscriptPath: compact.TranscriptPath,
		TokenCount:     compact.TokenCount,
	}

	// Call hooks and collect results
	results := a.preCompactChain.evaluate(event)

	// Emit audit event with results
	a.auditor.emit(sessionID, "hook.pre_compact", map[string]any{
		"trigger":         compact.Trigger,
		"transcript_path": compact.TranscriptPath,
		"token_count":     compact.TokenCount,
		"results":         results,
	})
}

// handleSubagentStopEvent processes a subagent completion event.
func (a *Agent) handleSubagentStopEvent(subagent *SubagentResultMsg) {
	if a.subagentStopChain == nil || len(a.cfg.subagentStopHooks) == 0 {
		return
	}

	a.mu.Lock()
	sessionID := a.sessionID
	a.mu.Unlock()

	event := &SubagentStopEvent{
		SessionID:       sessionID,
		SubagentID:      subagent.SubagentID,
		SubagentType:    subagent.SubagentType,
		ParentToolUseID: subagent.ParentToolUseID,
		NumTurns:        subagent.NumTurns,
		CostUSD:         subagent.CostUSD,
	}

	// Call all hooks
	a.subagentStopChain.evaluate(event)

	// Emit audit event
	a.auditor.emit(sessionID, "hook.subagent_stop", map[string]any{
		"subagent_id":        subagent.SubagentID,
		"subagent_type":      subagent.SubagentType,
		"parent_tool_use_id": subagent.ParentToolUseID,
		"num_turns":          subagent.NumTurns,
		"cost_usd":           subagent.CostUSD,
	})
}
