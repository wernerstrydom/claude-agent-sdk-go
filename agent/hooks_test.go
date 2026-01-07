package agent

import (
	"testing"
	"time"
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
	tc := &ToolCall{Name: "Bash", Input: map[string]any{"command": "echo hello"}}

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
	// Hook that only denies sudo commands
	denySudoHook := func(tc *ToolCall) HookResult {
		if tc.Name != "Bash" {
			return HookResult{Decision: Continue}
		}
		cmd, ok := tc.Input["command"].(string)
		if !ok {
			return HookResult{Decision: Continue}
		}
		if len(cmd) >= 4 && cmd[:4] == "sudo" {
			return HookResult{Decision: Deny, Reason: "sudo commands not allowed"}
		}
		return HookResult{Decision: Continue}
	}

	chain := newHookChain([]PreToolUseHook{denySudoHook})

	// Should deny sudo command
	tc1 := &ToolCall{Name: "Bash", Input: map[string]any{"command": "sudo apt update"}}
	result1 := chain.evaluate(tc1)
	if result1.Decision != Deny {
		t.Errorf("sudo command should be denied, got %v", result1.Decision)
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

// Tests for PostToolUseHook

func TestPostToolUseChainEmpty(t *testing.T) {
	chain := newPostToolUseChain(nil)
	tc := &ToolCall{Name: "Bash", Input: map[string]any{"command": "ls"}}
	tr := &ToolResultContext{
		ToolUseID: "test-123",
		Content:   "output",
		IsError:   false,
		Duration:  100 * time.Millisecond,
	}

	// Should not panic with empty chain
	chain.evaluate(tc, tr)
}

func TestPostToolUseChainCallsAllHooks(t *testing.T) {
	callOrder := []string{}

	hook1 := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		callOrder = append(callOrder, "hook1")
		return HookResult{Decision: Continue}
	}

	hook2 := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		callOrder = append(callOrder, "hook2")
		return HookResult{Decision: Continue}
	}

	hook3 := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		callOrder = append(callOrder, "hook3")
		return HookResult{Decision: Continue}
	}

	chain := newPostToolUseChain([]PostToolUseHook{hook1, hook2, hook3})
	tc := &ToolCall{Name: "Read", Input: map[string]any{"file_path": "/test"}}
	tr := &ToolResultContext{
		ToolUseID: "test-456",
		Content:   "file contents",
		IsError:   false,
		Duration:  50 * time.Millisecond,
	}

	chain.evaluate(tc, tr)

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

func TestPostToolUseHookReceivesCorrectData(t *testing.T) {
	var receivedTC *ToolCall
	var receivedTR *ToolResultContext

	hook := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		receivedTC = tc
		receivedTR = tr
		return HookResult{Decision: Continue}
	}

	chain := newPostToolUseChain([]PostToolUseHook{hook})

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "echo hello"},
	}
	tr := &ToolResultContext{
		ToolUseID: "toolu_123",
		Content:   "hello\n",
		IsError:   false,
		Duration:  25 * time.Millisecond,
	}

	chain.evaluate(tc, tr)

	if receivedTC.Name != "Bash" {
		t.Errorf("got tool name %q, want %q", receivedTC.Name, "Bash")
	}
	if receivedTC.Input["command"] != "echo hello" {
		t.Errorf("got command %q, want %q", receivedTC.Input["command"], "echo hello")
	}
	if receivedTR.ToolUseID != "toolu_123" {
		t.Errorf("got tool use ID %q, want %q", receivedTR.ToolUseID, "toolu_123")
	}
	if receivedTR.Content != "hello\n" {
		t.Errorf("got content %q, want %q", receivedTR.Content, "hello\n")
	}
	if receivedTR.IsError {
		t.Error("expected IsError to be false")
	}
	if receivedTR.Duration != 25*time.Millisecond {
		t.Errorf("got duration %v, want %v", receivedTR.Duration, 25*time.Millisecond)
	}
}

func TestPostToolUseHookReceivesError(t *testing.T) {
	var receivedIsError bool

	hook := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		receivedIsError = tr.IsError
		return HookResult{Decision: Continue}
	}

	chain := newPostToolUseChain([]PostToolUseHook{hook})
	tc := &ToolCall{Name: "Bash", Input: map[string]any{"command": "false"}}
	tr := &ToolResultContext{
		ToolUseID: "toolu_err",
		Content:   "command failed",
		IsError:   true,
		Duration:  10 * time.Millisecond,
	}

	chain.evaluate(tc, tr)

	if !receivedIsError {
		t.Error("expected IsError to be true")
	}
}

// Tests for StopReason and StopEvent

func TestStopReasonConstants(t *testing.T) {
	// Verify stop reason constants are distinct
	reasons := map[StopReason]bool{
		StopCompleted:   true,
		StopMaxTurns:    true,
		StopInterrupted: true,
		StopError:       true,
	}
	if len(reasons) != 4 {
		t.Error("stop reasons should be distinct")
	}
}

func TestStopEventFields(t *testing.T) {
	event := &StopEvent{
		SessionID: "session-abc",
		Reason:    StopCompleted,
		NumTurns:  5,
		CostUSD:   0.0042,
	}

	if event.SessionID != "session-abc" {
		t.Errorf("got session ID %q, want %q", event.SessionID, "session-abc")
	}
	if event.Reason != StopCompleted {
		t.Errorf("got reason %v, want %v", event.Reason, StopCompleted)
	}
	if event.NumTurns != 5 {
		t.Errorf("got num turns %d, want %d", event.NumTurns, 5)
	}
	if event.CostUSD != 0.0042 {
		t.Errorf("got cost $%.4f, want $%.4f", event.CostUSD, 0.0042)
	}
}

// Tests for ToolResultContext

func TestToolResultContextFields(t *testing.T) {
	ctx := &ToolResultContext{
		ToolUseID: "toolu_xyz",
		Content:   map[string]any{"result": "success"},
		IsError:   false,
		Duration:  150 * time.Millisecond,
	}

	if ctx.ToolUseID != "toolu_xyz" {
		t.Errorf("got tool use ID %q, want %q", ctx.ToolUseID, "toolu_xyz")
	}
	content, ok := ctx.Content.(map[string]any)
	if !ok || content["result"] != "success" {
		t.Error("content not preserved correctly")
	}
	if ctx.IsError {
		t.Error("expected IsError to be false")
	}
	if ctx.Duration != 150*time.Millisecond {
		t.Errorf("got duration %v, want %v", ctx.Duration, 150*time.Millisecond)
	}
}

// Tests for PreCompactHook

func TestPreCompactChainEmpty(t *testing.T) {
	chain := newPreCompactChain(nil)
	event := &PreCompactEvent{
		SessionID:      "session-123",
		Trigger:        "auto",
		TranscriptPath: "/tmp/transcript.json",
		TokenCount:     50000,
	}

	results := chain.evaluate(event)
	if results != nil {
		t.Errorf("empty chain should return nil, got %v", results)
	}
}

func TestPreCompactChainCallsAllHooks(t *testing.T) {
	callOrder := []string{}

	hook1 := func(e *PreCompactEvent) PreCompactResult {
		callOrder = append(callOrder, "hook1")
		return PreCompactResult{Archive: true, ArchiveTo: "/archive/1.json"}
	}

	hook2 := func(e *PreCompactEvent) PreCompactResult {
		callOrder = append(callOrder, "hook2")
		return PreCompactResult{Extract: "important data"}
	}

	chain := newPreCompactChain([]PreCompactHook{hook1, hook2})
	event := &PreCompactEvent{
		SessionID:      "session-456",
		Trigger:        "manual",
		TranscriptPath: "/tmp/transcript.json",
		TokenCount:     100000,
	}

	results := chain.evaluate(event)

	if len(callOrder) != 2 {
		t.Errorf("got %d hooks called, want 2", len(callOrder))
	}
	if callOrder[0] != "hook1" || callOrder[1] != "hook2" {
		t.Errorf("hooks called out of order: %v", callOrder)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
	if !results[0].Archive {
		t.Error("first hook result should have Archive=true")
	}
	if results[1].Extract != "important data" {
		t.Errorf("second hook extract = %v, want 'important data'", results[1].Extract)
	}
}

func TestPreCompactEventFields(t *testing.T) {
	event := &PreCompactEvent{
		SessionID:      "session-compact",
		Trigger:        "auto",
		TranscriptPath: "/path/to/transcript.json",
		TokenCount:     75000,
	}

	if event.SessionID != "session-compact" {
		t.Errorf("got session ID %q, want %q", event.SessionID, "session-compact")
	}
	if event.Trigger != "auto" {
		t.Errorf("got trigger %q, want %q", event.Trigger, "auto")
	}
	if event.TranscriptPath != "/path/to/transcript.json" {
		t.Errorf("got transcript path %q", event.TranscriptPath)
	}
	if event.TokenCount != 75000 {
		t.Errorf("got token count %d, want %d", event.TokenCount, 75000)
	}
}

func TestPreCompactResultFields(t *testing.T) {
	result := PreCompactResult{
		Archive:   true,
		ArchiveTo: "/archive/backup.json",
		Extract:   map[string]any{"key": "value"},
	}

	if !result.Archive {
		t.Error("expected Archive to be true")
	}
	if result.ArchiveTo != "/archive/backup.json" {
		t.Errorf("got ArchiveTo %q", result.ArchiveTo)
	}
	extract, ok := result.Extract.(map[string]any)
	if !ok || extract["key"] != "value" {
		t.Error("Extract not preserved correctly")
	}
}

// Tests for SubagentStopHook

func TestSubagentStopChainEmpty(t *testing.T) {
	chain := newSubagentStopChain(nil)
	event := &SubagentStopEvent{
		SessionID:       "parent-123",
		SubagentID:      "subagent-456",
		SubagentType:    "Task",
		ParentToolUseID: "toolu_abc",
		NumTurns:        3,
		CostUSD:         0.005,
	}

	// Should not panic with empty chain
	chain.evaluate(event)
}

func TestSubagentStopChainCallsAllHooks(t *testing.T) {
	callOrder := []string{}
	var receivedEvents []*SubagentStopEvent

	hook1 := func(e *SubagentStopEvent) {
		callOrder = append(callOrder, "hook1")
		receivedEvents = append(receivedEvents, e)
	}

	hook2 := func(e *SubagentStopEvent) {
		callOrder = append(callOrder, "hook2")
		receivedEvents = append(receivedEvents, e)
	}

	chain := newSubagentStopChain([]SubagentStopHook{hook1, hook2})
	event := &SubagentStopEvent{
		SessionID:       "parent-789",
		SubagentID:      "subagent-xyz",
		SubagentType:    "Explore",
		ParentToolUseID: "toolu_parent",
		NumTurns:        5,
		CostUSD:         0.0123,
	}

	chain.evaluate(event)

	if len(callOrder) != 2 {
		t.Errorf("got %d hooks called, want 2", len(callOrder))
	}
	if callOrder[0] != "hook1" || callOrder[1] != "hook2" {
		t.Errorf("hooks called out of order: %v", callOrder)
	}
	if len(receivedEvents) != 2 {
		t.Errorf("got %d events, want 2", len(receivedEvents))
	}
	for i, e := range receivedEvents {
		if e.SubagentID != "subagent-xyz" {
			t.Errorf("hook %d got wrong subagent ID: %q", i+1, e.SubagentID)
		}
	}
}

func TestSubagentStopEventFields(t *testing.T) {
	event := &SubagentStopEvent{
		SessionID:       "session-parent",
		SubagentID:      "subagent-child",
		SubagentType:    "Task",
		ParentToolUseID: "toolu_spawn",
		NumTurns:        10,
		CostUSD:         0.025,
	}

	if event.SessionID != "session-parent" {
		t.Errorf("got session ID %q", event.SessionID)
	}
	if event.SubagentID != "subagent-child" {
		t.Errorf("got subagent ID %q", event.SubagentID)
	}
	if event.SubagentType != "Task" {
		t.Errorf("got subagent type %q", event.SubagentType)
	}
	if event.ParentToolUseID != "toolu_spawn" {
		t.Errorf("got parent tool use ID %q", event.ParentToolUseID)
	}
	if event.NumTurns != 10 {
		t.Errorf("got num turns %d, want 10", event.NumTurns)
	}
	if event.CostUSD != 0.025 {
		t.Errorf("got cost $%.4f, want $%.4f", event.CostUSD, 0.025)
	}
}

// Tests for UserPromptSubmitHook

func TestPromptSubmitChainEmpty(t *testing.T) {
	chain := newPromptSubmitChain(nil)
	event := &PromptSubmitEvent{
		Prompt:    "Hello, world!",
		SessionID: "session-123",
		Turn:      1,
	}

	finalPrompt, metadata := chain.evaluate(event)
	if finalPrompt != "Hello, world!" {
		t.Errorf("empty chain should return original prompt, got %q", finalPrompt)
	}
	if metadata != nil {
		t.Errorf("empty chain should return nil metadata, got %v", metadata)
	}
}

func TestPromptSubmitChainModifiesPrompt(t *testing.T) {
	hook := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{
			UpdatedPrompt: e.Prompt + " [modified]",
		}
	}

	chain := newPromptSubmitChain([]UserPromptSubmitHook{hook})
	event := &PromptSubmitEvent{
		Prompt:    "Original prompt",
		SessionID: "session-456",
		Turn:      1,
	}

	finalPrompt, _ := chain.evaluate(event)
	if finalPrompt != "Original prompt [modified]" {
		t.Errorf("got %q, want 'Original prompt [modified]'", finalPrompt)
	}
}

func TestPromptSubmitChainChainsModifications(t *testing.T) {
	hook1 := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{
			UpdatedPrompt: e.Prompt + " [hook1]",
		}
	}

	hook2 := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{
			UpdatedPrompt: e.Prompt + " [hook2]",
		}
	}

	chain := newPromptSubmitChain([]UserPromptSubmitHook{hook1, hook2})
	event := &PromptSubmitEvent{
		Prompt:    "Base",
		SessionID: "session-789",
		Turn:      1,
	}

	finalPrompt, _ := chain.evaluate(event)
	if finalPrompt != "Base [hook1] [hook2]" {
		t.Errorf("got %q, want 'Base [hook1] [hook2]'", finalPrompt)
	}
}

func TestPromptSubmitChainEmptyUpdateUsesOriginal(t *testing.T) {
	hook := func(e *PromptSubmitEvent) PromptSubmitResult {
		// Return empty UpdatedPrompt - should use original
		return PromptSubmitResult{
			Metadata: map[string]any{"observed": true},
		}
	}

	chain := newPromptSubmitChain([]UserPromptSubmitHook{hook})
	event := &PromptSubmitEvent{
		Prompt:    "Keep me unchanged",
		SessionID: "session-abc",
		Turn:      2,
	}

	finalPrompt, metadata := chain.evaluate(event)
	if finalPrompt != "Keep me unchanged" {
		t.Errorf("got %q, want 'Keep me unchanged'", finalPrompt)
	}
	if len(metadata) != 1 {
		t.Errorf("got %d metadata entries, want 1", len(metadata))
	}
}

func TestPromptSubmitChainCollectsMetadata(t *testing.T) {
	hook1 := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{
			Metadata: map[string]any{"from": "hook1"},
		}
	}

	hook2 := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{
			UpdatedPrompt: e.Prompt + "!",
			Metadata:      "string metadata",
		}
	}

	hook3 := func(e *PromptSubmitEvent) PromptSubmitResult {
		// No metadata
		return PromptSubmitResult{}
	}

	chain := newPromptSubmitChain([]UserPromptSubmitHook{hook1, hook2, hook3})
	event := &PromptSubmitEvent{
		Prompt:    "Hello",
		SessionID: "session-meta",
		Turn:      3,
	}

	finalPrompt, metadata := chain.evaluate(event)
	if finalPrompt != "Hello!" {
		t.Errorf("got %q, want 'Hello!'", finalPrompt)
	}
	if len(metadata) != 2 {
		t.Errorf("got %d metadata entries, want 2", len(metadata))
	}
}

func TestPromptSubmitEventFields(t *testing.T) {
	event := &PromptSubmitEvent{
		Prompt:    "What is 2+2?",
		SessionID: "session-prompt",
		Turn:      5,
	}

	if event.Prompt != "What is 2+2?" {
		t.Errorf("got prompt %q", event.Prompt)
	}
	if event.SessionID != "session-prompt" {
		t.Errorf("got session ID %q", event.SessionID)
	}
	if event.Turn != 5 {
		t.Errorf("got turn %d, want 5", event.Turn)
	}
}

func TestPromptSubmitResultFields(t *testing.T) {
	result := PromptSubmitResult{
		UpdatedPrompt: "Modified prompt",
		Metadata:      map[string]any{"key": "value"},
	}

	if result.UpdatedPrompt != "Modified prompt" {
		t.Errorf("got UpdatedPrompt %q", result.UpdatedPrompt)
	}
	meta, ok := result.Metadata.(map[string]any)
	if !ok || meta["key"] != "value" {
		t.Error("Metadata not preserved correctly")
	}
}
