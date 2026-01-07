package agent

import "time"

// Decision represents the outcome of a hook evaluation.
type Decision int

const (
	// Continue passes evaluation to the next hook in the chain.
	Continue Decision = iota
	// Allow approves the operation and skips remaining hooks.
	Allow
	// Deny blocks the operation.
	Deny
)

// String returns a string representation of the Decision.
func (d Decision) String() string {
	switch d {
	case Continue:
		return "continue"
	case Allow:
		return "allow"
	case Deny:
		return "deny"
	default:
		return "unknown"
	}
}

// ToolCall represents a tool invocation that can be intercepted by hooks.
type ToolCall struct {
	Name  string
	Input map[string]any
}

// HookResult is the outcome of evaluating a hook.
type HookResult struct {
	// Decision determines whether to allow, deny, or continue to next hook.
	Decision Decision
	// Reason provides feedback to Claude when denying.
	Reason string
	// UpdatedInput optionally modifies the tool inputs.
	UpdatedInput map[string]any
}

// PreToolUseHook is called before a tool is executed.
// It can allow, deny, or modify the tool call.
type PreToolUseHook func(*ToolCall) HookResult

// hookChain evaluates multiple hooks in sequence.
type hookChain struct {
	hooks []PreToolUseHook
}

// newHookChain creates a new hook chain from the given hooks.
func newHookChain(hooks []PreToolUseHook) *hookChain {
	return &hookChain{hooks: hooks}
}

// evaluate runs the hook chain against a tool call.
// First Deny wins, Allow short-circuits, Continue passes to next.
// If all hooks return Continue, the result is Allow.
func (c *hookChain) evaluate(tc *ToolCall) HookResult {
	if len(c.hooks) == 0 {
		return HookResult{Decision: Allow}
	}

	// Track accumulated input updates
	var accumulatedUpdates map[string]any

	for _, hook := range c.hooks {
		// Apply accumulated updates before each hook evaluation
		if accumulatedUpdates != nil {
			tc.Input = mergeInputs(tc.Input, accumulatedUpdates)
		}

		result := hook(tc)

		switch result.Decision {
		case Deny:
			// First Deny wins immediately
			return result
		case Allow:
			// Allow short-circuits, apply any final updates
			if result.UpdatedInput != nil {
				accumulatedUpdates = mergeInputs(accumulatedUpdates, result.UpdatedInput)
			}
			return HookResult{
				Decision:     Allow,
				UpdatedInput: accumulatedUpdates,
			}
		case Continue:
			// Accumulate any input updates
			if result.UpdatedInput != nil {
				accumulatedUpdates = mergeInputs(accumulatedUpdates, result.UpdatedInput)
			}
		}
	}

	// All hooks returned Continue - default to Allow
	return HookResult{
		Decision:     Allow,
		UpdatedInput: accumulatedUpdates,
	}
}

// mergeInputs merges two input maps, with updates taking precedence.
func mergeInputs(base, updates map[string]any) map[string]any {
	if base == nil && updates == nil {
		return nil
	}
	if base == nil {
		result := make(map[string]any, len(updates))
		for k, v := range updates {
			result[k] = v
		}
		return result
	}
	if updates == nil {
		return base
	}

	result := make(map[string]any, len(base)+len(updates))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range updates {
		result[k] = v
	}
	return result
}

// ToolResultContext provides context about a completed tool execution.
// This is passed to PostToolUse hooks along with the original ToolCall.
type ToolResultContext struct {
	// ToolUseID is the unique identifier of the tool invocation.
	ToolUseID string
	// Content is the result returned by the tool.
	Content any
	// IsError indicates whether the tool execution resulted in an error.
	IsError bool
	// Duration is how long the tool took to execute.
	Duration time.Duration
}

// PostToolUseHook is called after a tool has executed.
// It receives the original tool call and the result context.
// PostToolUse hooks are for observation and logging - they cannot modify
// the result or prevent it from being returned.
type PostToolUseHook func(*ToolCall, *ToolResultContext) HookResult

// postToolUseChain evaluates multiple PostToolUse hooks in sequence.
type postToolUseChain struct {
	hooks []PostToolUseHook
}

// newPostToolUseChain creates a new PostToolUse hook chain.
func newPostToolUseChain(hooks []PostToolUseHook) *postToolUseChain {
	return &postToolUseChain{hooks: hooks}
}

// evaluate runs the PostToolUse hook chain against a completed tool execution.
// All hooks are called in order; the chain does not short-circuit.
func (c *postToolUseChain) evaluate(tc *ToolCall, tr *ToolResultContext) {
	if c == nil || len(c.hooks) == 0 {
		return
	}
	for _, hook := range c.hooks {
		hook(tc, tr)
	}
}

// StopReason describes why an agent session ended.
type StopReason string

const (
	// StopCompleted indicates the session ended normally after completing the task.
	StopCompleted StopReason = "completed"
	// StopMaxTurns indicates the session was stopped because max turns was reached.
	StopMaxTurns StopReason = "max_turns"
	// StopInterrupted indicates the session was interrupted (e.g., context cancelled).
	StopInterrupted StopReason = "interrupted"
	// StopError indicates the session ended due to an error.
	StopError StopReason = "error"
)

// StopEvent provides context about why an agent session ended.
type StopEvent struct {
	// SessionID is the session identifier.
	SessionID string
	// Reason describes why the session ended.
	Reason StopReason
	// NumTurns is the total number of turns in the session.
	NumTurns int
	// CostUSD is the total cost of the session in USD.
	CostUSD float64
}

// StopHook is called when an agent session ends.
// It receives information about the session and why it ended.
// Stop hooks are for cleanup, metrics reporting, and logging.
type StopHook func(*StopEvent)

// PreCompactEvent provides context about an impending context window compaction.
// Claude Code compacts the context window when it approaches the token limit.
type PreCompactEvent struct {
	// SessionID is the session identifier.
	SessionID string
	// Trigger indicates what caused the compaction ("auto" or "manual").
	Trigger string
	// TranscriptPath is the path to the transcript file.
	TranscriptPath string
	// TokenCount is the approximate token count before compaction.
	TokenCount int
}

// PreCompactResult is returned from PreCompact hooks.
type PreCompactResult struct {
	// Archive indicates whether to archive the current transcript.
	Archive bool
	// ArchiveTo is the path to archive the transcript to (if Archive is true).
	ArchiveTo string
	// Extract allows the hook to extract and preserve arbitrary data
	// from the pre-compaction state.
	Extract any
}

// PreCompactHook is called before context window compaction.
// It can archive the transcript or extract important data before compaction.
type PreCompactHook func(*PreCompactEvent) PreCompactResult

// preCompactChain evaluates multiple PreCompact hooks.
type preCompactChain struct {
	hooks []PreCompactHook
}

// newPreCompactChain creates a new PreCompact hook chain.
func newPreCompactChain(hooks []PreCompactHook) *preCompactChain {
	return &preCompactChain{hooks: hooks}
}

// evaluate runs the PreCompact hook chain.
// All hooks are called and their results are aggregated.
// Archive is true if any hook requests it. ArchiveTo uses the first non-empty path.
func (c *preCompactChain) evaluate(e *PreCompactEvent) []PreCompactResult {
	if c == nil || len(c.hooks) == 0 {
		return nil
	}
	results := make([]PreCompactResult, 0, len(c.hooks))
	for _, hook := range c.hooks {
		results = append(results, hook(e))
	}
	return results
}

// SubagentStopEvent provides context about a completed subagent execution.
type SubagentStopEvent struct {
	// SessionID is the parent session identifier.
	SessionID string
	// SubagentID is the unique identifier of the subagent.
	SubagentID string
	// SubagentType describes the type of subagent (e.g., "Task", "Explore").
	SubagentType string
	// ParentToolUseID links to the tool_use that spawned this subagent.
	ParentToolUseID string
	// NumTurns is the number of turns the subagent took.
	NumTurns int
	// CostUSD is the cost incurred by the subagent.
	CostUSD float64
}

// SubagentStopHook is called when a subagent completes execution.
// It receives information about the subagent and its execution.
// These hooks are for observation and logging.
type SubagentStopHook func(*SubagentStopEvent)

// subagentStopChain evaluates multiple SubagentStop hooks.
type subagentStopChain struct {
	hooks []SubagentStopHook
}

// newSubagentStopChain creates a new SubagentStop hook chain.
func newSubagentStopChain(hooks []SubagentStopHook) *subagentStopChain {
	return &subagentStopChain{hooks: hooks}
}

// evaluate runs the SubagentStop hook chain.
// All hooks are called in order.
func (c *subagentStopChain) evaluate(e *SubagentStopEvent) {
	if c == nil || len(c.hooks) == 0 {
		return
	}
	for _, hook := range c.hooks {
		hook(e)
	}
}

// PromptSubmitEvent provides context for prompt interception.
type PromptSubmitEvent struct {
	// Prompt is the user prompt being submitted.
	Prompt string
	// SessionID is the session identifier.
	SessionID string
	// Turn is the current turn number.
	Turn int
}

// PromptSubmitResult is returned from UserPromptSubmit hooks.
type PromptSubmitResult struct {
	// UpdatedPrompt optionally replaces the original prompt.
	// If empty, the original prompt is used.
	UpdatedPrompt string
	// Metadata can store arbitrary data for audit purposes.
	Metadata any
}

// UserPromptSubmitHook is called before a prompt is sent to Claude.
// It can modify the prompt or attach metadata for audit purposes.
type UserPromptSubmitHook func(*PromptSubmitEvent) PromptSubmitResult

// promptSubmitChain evaluates multiple UserPromptSubmit hooks.
type promptSubmitChain struct {
	hooks []UserPromptSubmitHook
}

// newPromptSubmitChain creates a new UserPromptSubmit hook chain.
func newPromptSubmitChain(hooks []UserPromptSubmitHook) *promptSubmitChain {
	return &promptSubmitChain{hooks: hooks}
}

// evaluate runs the UserPromptSubmit hook chain.
// Hooks are called in order. If a hook returns a non-empty UpdatedPrompt,
// subsequent hooks receive that updated prompt.
// Returns the final prompt and aggregated metadata.
func (c *promptSubmitChain) evaluate(e *PromptSubmitEvent) (string, []any) {
	if c == nil || len(c.hooks) == 0 {
		return e.Prompt, nil
	}

	currentPrompt := e.Prompt
	var metadata []any

	for _, hook := range c.hooks {
		// Create event with current prompt state
		event := &PromptSubmitEvent{
			Prompt:    currentPrompt,
			SessionID: e.SessionID,
			Turn:      e.Turn,
		}
		result := hook(event)

		// Update prompt if hook provided a replacement
		if result.UpdatedPrompt != "" {
			currentPrompt = result.UpdatedPrompt
		}

		// Collect metadata
		if result.Metadata != nil {
			metadata = append(metadata, result.Metadata)
		}
	}

	return currentPrompt, metadata
}
