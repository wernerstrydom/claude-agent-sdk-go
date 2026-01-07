package agent

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
