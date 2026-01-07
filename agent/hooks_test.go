package agent

import (
	"testing"
)

func TestDecisionString(t *testing.T) {
	tests := []struct {
		d    Decision
		want string
	}{
		{Continue, "continue"},
		{Allow, "allow"},
		{Deny, "deny"},
		{Decision(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("Decision(%d).String() = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestEmptyChainReturnsAllow(t *testing.T) {
	chain := newHookChain(nil)
	tc := &ToolCall{Name: "Bash", Input: map[string]any{"command": "ls"}}

	result := chain.evaluate(tc)

	if result.Decision != Allow {
		t.Errorf("empty chain returned %v, want Allow", result.Decision)
	}
}

func TestSingleDenyReturnsDeny(t *testing.T) {
	denyHook := func(tc *ToolCall) HookResult {
		return HookResult{
			Decision: Deny,
			Reason:   "not allowed",
		}
	}

	chain := newHookChain([]PreToolUseHook{denyHook})
	tc := &ToolCall{Name: "Bash", Input: map[string]any{"command": "rm -rf /"}}

	result := chain.evaluate(tc)

	if result.Decision != Deny {
		t.Errorf("got %v, want Deny", result.Decision)
	}
	if result.Reason != "not allowed" {
		t.Errorf("got reason %q, want %q", result.Reason, "not allowed")
	}
}

func TestSingleAllowReturnsAllow(t *testing.T) {
	allowHook := func(tc *ToolCall) HookResult {
		return HookResult{Decision: Allow}
	}

	chain := newHookChain([]PreToolUseHook{allowHook})
	tc := &ToolCall{Name: "Read", Input: map[string]any{"file_path": "/tmp/test.txt"}}

	result := chain.evaluate(tc)

	if result.Decision != Allow {
		t.Errorf("got %v, want Allow", result.Decision)
	}
}

func TestSingleContinueFallsThroughToAllow(t *testing.T) {
	continueHook := func(tc *ToolCall) HookResult {
		return HookResult{Decision: Continue}
	}

	chain := newHookChain([]PreToolUseHook{continueHook})
	tc := &ToolCall{Name: "Bash", Input: map[string]any{"command": "echo hello"}}

	result := chain.evaluate(tc)

	if result.Decision != Allow {
		t.Errorf("got %v, want Allow (fallthrough)", result.Decision)
	}
}

func TestDenyShortCircuits(t *testing.T) {
	called := false

	denyHook := func(tc *ToolCall) HookResult {
		return HookResult{Decision: Deny, Reason: "blocked"}
	}

	shouldNotBeCalled := func(tc *ToolCall) HookResult {
		called = true
		return HookResult{Decision: Allow}
	}

	chain := newHookChain([]PreToolUseHook{denyHook, shouldNotBeCalled})
	tc := &ToolCall{Name: "Bash", Input: map[string]any{}}

	result := chain.evaluate(tc)

	if result.Decision != Deny {
		t.Errorf("got %v, want Deny", result.Decision)
	}
	if called {
		t.Error("second hook should not have been called after Deny")
	}
}

func TestAllowShortCircuits(t *testing.T) {
	called := false

	allowHook := func(tc *ToolCall) HookResult {
		return HookResult{Decision: Allow}
	}

	shouldNotBeCalled := func(tc *ToolCall) HookResult {
		called = true
		return HookResult{Decision: Deny, Reason: "should not reach here"}
	}

	chain := newHookChain([]PreToolUseHook{allowHook, shouldNotBeCalled})
	tc := &ToolCall{Name: "Read", Input: map[string]any{}}

	result := chain.evaluate(tc)

	if result.Decision != Allow {
		t.Errorf("got %v, want Allow", result.Decision)
	}
	if called {
		t.Error("second hook should not have been called after Allow")
	}
}

func TestUpdatedInputPassedThroughChain(t *testing.T) {
	hook1 := func(tc *ToolCall) HookResult {
		return HookResult{
			Decision:     Continue,
			UpdatedInput: map[string]any{"key1": "value1"},
		}
	}

	hook2 := func(tc *ToolCall) HookResult {
		// Verify we received the updated input
		if tc.Input["key1"] != "value1" {
			t.Errorf("hook2 did not receive updated input from hook1")
		}
		return HookResult{
			Decision:     Continue,
			UpdatedInput: map[string]any{"key2": "value2"},
		}
	}

	chain := newHookChain([]PreToolUseHook{hook1, hook2})
	tc := &ToolCall{Name: "Bash", Input: map[string]any{"original": "data"}}

	result := chain.evaluate(tc)

	if result.Decision != Allow {
		t.Errorf("got %v, want Allow", result.Decision)
	}

	// Check accumulated updates
	if result.UpdatedInput == nil {
		t.Fatal("UpdatedInput should not be nil")
	}
	if result.UpdatedInput["key1"] != "value1" {
		t.Errorf("missing key1 in UpdatedInput")
	}
	if result.UpdatedInput["key2"] != "value2" {
		t.Errorf("missing key2 in UpdatedInput")
	}
}

func TestMultipleHooksCompose(t *testing.T) {
	callOrder := []string{}

	hook1 := func(tc *ToolCall) HookResult {
		callOrder = append(callOrder, "hook1")
		return HookResult{Decision: Continue}
	}

	hook2 := func(tc *ToolCall) HookResult {
		callOrder = append(callOrder, "hook2")
		return HookResult{Decision: Continue}
	}

	hook3 := func(tc *ToolCall) HookResult {
		callOrder = append(callOrder, "hook3")
		return HookResult{Decision: Continue}
	}

	chain := newHookChain([]PreToolUseHook{hook1, hook2, hook3})
	tc := &ToolCall{Name: "Bash", Input: map[string]any{}}

	result := chain.evaluate(tc)

	if result.Decision != Allow {
		t.Errorf("got %v, want Allow", result.Decision)
	}

	expected := []string{"hook1", "hook2", "hook3"}
	if len(callOrder) != len(expected) {
		t.Errorf("got %d hooks called, want %d", len(callOrder), len(expected))
	}
	for i, name := range expected {
		if i < len(callOrder) && callOrder[i] != name {
			t.Errorf("hook %d was %q, want %q", i, callOrder[i], name)
		}
	}
}

func TestMergeInputsNilCases(t *testing.T) {
	// Both nil
	result := mergeInputs(nil, nil)
	if result != nil {
		t.Errorf("mergeInputs(nil, nil) = %v, want nil", result)
	}

	// Base nil, updates non-nil
	result = mergeInputs(nil, map[string]any{"a": 1})
	if result == nil || result["a"] != 1 {
		t.Errorf("mergeInputs(nil, updates) failed")
	}

	// Base non-nil, updates nil
	base := map[string]any{"b": 2}
	result = mergeInputs(base, nil)
	if result == nil || result["b"] != 2 {
		t.Errorf("mergeInputs(base, nil) failed")
	}

	// Both non-nil, updates override
	result = mergeInputs(map[string]any{"x": 1}, map[string]any{"x": 2, "y": 3})
	if result["x"] != 2 || result["y"] != 3 {
		t.Errorf("mergeInputs override failed: %v", result)
	}
}

func TestConditionalHookLogic(t *testing.T) {
	// Hook that only denies rm commands
	denyRmHook := func(tc *ToolCall) HookResult {
		if tc.Name != "Bash" {
			return HookResult{Decision: Continue}
		}
		cmd, ok := tc.Input["command"].(string)
		if !ok {
			return HookResult{Decision: Continue}
		}
		if len(cmd) >= 2 && cmd[:2] == "rm" {
			return HookResult{Decision: Deny, Reason: "rm commands not allowed"}
		}
		return HookResult{Decision: Continue}
	}

	chain := newHookChain([]PreToolUseHook{denyRmHook})

	// Should deny rm command
	tc1 := &ToolCall{Name: "Bash", Input: map[string]any{"command": "rm -rf /"}}
	result1 := chain.evaluate(tc1)
	if result1.Decision != Deny {
		t.Errorf("rm command should be denied, got %v", result1.Decision)
	}

	// Should allow ls command
	tc2 := &ToolCall{Name: "Bash", Input: map[string]any{"command": "ls -la"}}
	result2 := chain.evaluate(tc2)
	if result2.Decision != Allow {
		t.Errorf("ls command should be allowed, got %v", result2.Decision)
	}

	// Should allow Read tool
	tc3 := &ToolCall{Name: "Read", Input: map[string]any{"file_path": "/tmp/test"}}
	result3 := chain.evaluate(tc3)
	if result3.Decision != Allow {
		t.Errorf("Read tool should be allowed, got %v", result3.Decision)
	}
}
