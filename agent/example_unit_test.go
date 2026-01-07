package agent

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// These are unit tests that use fake CLI scripts to test SDK behavior.
// For real-world examples, see example_test.go (requires //go:build integration).

func TestExampleRunBasic(t *testing.T) {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"run-example"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"4"}]}}'
echo '{"type":"result","result":"4","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	result, err := a.Run(ctx, "What is 2 + 2?")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ResultText != "4" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "4")
	}
	if result.CostUSD != 0.001 {
		t.Errorf("CostUSD = %v, want 0.001", result.CostUSD)
	}
}

func TestExampleSessionIDCapture(t *testing.T) {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"sess-12345"}'
echo '{"type":"result","result":"OK","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	// Session ID is empty before first message exchange
	if a.SessionID() != "" {
		t.Errorf("SessionID() before message = %q, want empty", a.SessionID())
	}

	// Send a message to trigger init
	for range a.Stream(ctx, "test") {
	}

	if a.SessionID() != "sess-12345" {
		t.Errorf("SessionID() after message = %q, want %q", a.SessionID(), "sess-12345")
	}
}

func TestExampleStreamMultipleMessages(t *testing.T) {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"stream-example"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Go channels are typed conduits."}]}}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":" They enable communication between goroutines."}]}}'
echo '{"type":"result","result":"Done","num_turns":1,"total_cost_usd":0.002}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	var textParts []string
	var resultCost float64

	for msg := range a.Stream(ctx, "Explain Go channels") {
		switch m := msg.(type) {
		case *Text:
			textParts = append(textParts, m.Text)
		case *Result:
			resultCost = m.CostUSD
		}
	}

	fullText := strings.Join(textParts, "")
	expected := "Go channels are typed conduits. They enable communication between goroutines."
	if fullText != expected {
		t.Errorf("text = %q, want %q", fullText, expected)
	}
	if resultCost != 0.002 {
		t.Errorf("CostUSD = %v, want 0.002", resultCost)
	}
}

func TestExampleErrAfterStream(t *testing.T) {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"err-example"}'
echo '{"type":"result","result":"OK","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	// Consume all messages
	for range a.Stream(ctx, "test") {
	}

	// Check for errors
	if err := a.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func TestExampleWithSecurityHooks(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"hooks-example"}'
echo '{"type":"result","result":"Listed files","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with security hooks
	a, err := New(ctx,
		CLIPath(fakeClaude),
		PreToolUse(
			DenyCommands("sudo", "su", "chmod 777"),
			RequireCommand("make", "go build", "go test"),
			AllowPaths(".", "/tmp"),
			RedirectPath("/tmp", "./sandbox/tmp"),
		),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	result, err := a.Run(ctx, "List files in the current directory")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ResultText != "Listed files" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "Listed files")
	}
}

func TestExampleMaxTurnsExceeded(t *testing.T) {
	// Create a fake CLI that returns 5 turns (exceeding our limit of 3)
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"maxturns-example"}'
echo '{"type":"result","result":"Done","num_turns":5,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with a turn limit
	a, err := New(ctx,
		CLIPath(fakeClaude),
		MaxTurns(3), // Limit to 3 turns
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	// This run exceeds the limit (5 turns > 3 max)
	_, err = a.Run(ctx, "Do something")

	// Check for MaxTurnsError
	var maxErr *MaxTurnsError
	if !errors.As(err, &maxErr) {
		t.Fatalf("expected MaxTurnsError, got %T: %v", err, err)
	}
	if maxErr.Turns != 5 {
		t.Errorf("Turns = %d, want 5", maxErr.Turns)
	}
	if maxErr.MaxAllowed != 3 {
		t.Errorf("MaxAllowed = %d, want 3", maxErr.MaxAllowed)
	}
}

func TestExampleResumePreviousSession(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// First session: establish context
	script1 := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"sess-original"}'
echo '{"type":"result","result":"Context established","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script1), 0755)

	ctx := context.Background()

	// Create first session
	a1, err := New(ctx, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _ = a1.Run(ctx, "Remember: the project is called Rooikat")
	sessionID := a1.SessionID()
	_ = a1.Close()

	if sessionID != "sess-original" {
		t.Errorf("sessionID = %q, want %q", sessionID, "sess-original")
	}

	// Second session: resume and query
	script2 := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"sess-original"}'
echo '{"type":"result","result":"The project is called Rooikat","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script2), 0755)

	a2, err := New(ctx,
		CLIPath(fakeClaude),
		Resume(sessionID),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a2)

	result, err := a2.Run(ctx, "What is the project name?")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.ResultText != "The project is called Rooikat" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "The project is called Rooikat")
	}
}

func TestExampleForkSession(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"sess-forked"}'
echo '{"type":"result","result":"Forked session active","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Fork from an existing session
	a, err := New(ctx,
		CLIPath(fakeClaude),
		Fork("sess-original"), // Fork from original session
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	result, err := a.Run(ctx, "Continue from where we left off")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if a.SessionID() != "sess-forked" {
		t.Errorf("SessionID() = %q, want %q", a.SessionID(), "sess-forked")
	}
	if result.ResultText != "Forked session active" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "Forked session active")
	}
}

func TestExampleWithSchema(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"schema-example"}'
echo '{"type":"result","result":"{\"answer\":\"4\",\"explanation\":\"2+2=4\"}","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	type MathAnswer struct {
		Answer      string `json:"answer" desc:"The numeric answer"`
		Explanation string `json:"explanation" desc:"Brief explanation"`
	}

	ctx := context.Background()

	// Create agent with structured output schema
	a, err := New(ctx,
		CLIPath(fakeClaude),
		WithSchema(MathAnswer{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	var answer MathAnswer
	_, err = a.RunWithSchema(ctx, "What is 2+2?", &answer)
	if err != nil {
		t.Fatalf("RunWithSchema() error = %v", err)
	}

	if answer.Answer != "4" {
		t.Errorf("Answer = %q, want %q", answer.Answer, "4")
	}
	if answer.Explanation != "2+2=4" {
		t.Errorf("Explanation = %q, want %q", answer.Explanation, "2+2=4")
	}
}

func TestExampleRunStructured(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"structured-example"}'
echo '{"type":"result","result":"{\"name\":\"Scrambled Eggs\",\"steps\":[\"Crack eggs\",\"Beat with fork\",\"Cook in pan\"]}","num_turns":1,"total_cost_usd":0.002}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	type Recipe struct {
		Name  string   `json:"name" desc:"Recipe name"`
		Steps []string `json:"steps" desc:"Cooking steps"`
	}

	ctx := context.Background()

	var recipe Recipe
	result, err := RunStructured(ctx, "Create a simple recipe", &recipe, CLIPath(fakeClaude))
	if err != nil {
		t.Fatalf("RunStructured() error = %v", err)
	}

	if recipe.Name != "Scrambled Eggs" {
		t.Errorf("Name = %q, want %q", recipe.Name, "Scrambled Eggs")
	}
	if len(recipe.Steps) != 3 {
		t.Errorf("len(Steps) = %d, want 3", len(recipe.Steps))
	}
	if result.CostUSD != 0.002 {
		t.Errorf("CostUSD = %v, want 0.002", result.CostUSD)
	}
}

func TestExampleStructuredOutputWithNestedTypes(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"nested-example"}'
echo '{"type":"result","result":"{\"title\":\"Pasta\",\"ingredients\":[{\"item\":\"Pasta\",\"amount\":\"500g\"},{\"item\":\"Salt\",\"amount\":\"1 tsp\"}]}","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	type Ingredient struct {
		Item   string `json:"item" desc:"Ingredient name"`
		Amount string `json:"amount" desc:"Quantity needed"`
	}

	type Recipe struct {
		Title       string       `json:"title" desc:"Recipe title"`
		Ingredients []Ingredient `json:"ingredients" desc:"List of ingredients"`
	}

	ctx := context.Background()

	a, err := New(ctx,
		CLIPath(fakeClaude),
		WithSchema(Recipe{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer mustClose(t, a)

	var recipe Recipe
	_, err = a.RunWithSchema(ctx, "Create a pasta recipe", &recipe)
	if err != nil {
		t.Fatalf("RunWithSchema() error = %v", err)
	}

	if recipe.Title != "Pasta" {
		t.Errorf("Title = %q, want %q", recipe.Title, "Pasta")
	}
	if len(recipe.Ingredients) != 2 {
		t.Errorf("len(Ingredients) = %d, want 2", len(recipe.Ingredients))
	}
	if recipe.Ingredients[0].Item != "Pasta" {
		t.Errorf("Ingredients[0].Item = %q, want %q", recipe.Ingredients[0].Item, "Pasta")
	}
}

func TestExampleAuditHandler(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"audit-example"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello!"}]}}'
echo '{"type":"result","result":"Done","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Collect audit events
	var events []string
	handler := func(e AuditEvent) {
		events = append(events, e.Type)
	}

	// Create agent with audit handler
	a, err := New(ctx,
		CLIPath(fakeClaude),
		Audit(handler),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = a.Run(ctx, "Hello")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	_ = a.Close()

	// Verify expected event types
	expectedEvents := []string{
		"session.start",
		"message.prompt",
		"session.init",
		"message.text",
		"message.result",
		"session.end",
	}

	if len(events) != len(expectedEvents) {
		t.Errorf("event count = %d, want %d", len(events), len(expectedEvents))
	}

	for i, expected := range expectedEvents {
		if i < len(events) && events[i] != expected {
			t.Errorf("events[%d] = %q, want %q", i, events[i], expected)
		}
	}
}

func TestExampleAuditToFile(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"audit-file-example"}'
echo '{"type":"result","result":"Done","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	auditPath := filepath.Join(tmpDir, "audit.jsonl")

	ctx := context.Background()

	// Create agent with file audit
	a, err := New(ctx,
		CLIPath(fakeClaude),
		AuditToFile(auditPath),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _ = a.Run(ctx, "Hello")
	_ = a.Close()

	// Read and verify the audit file
	data := mustReadFile(t, auditPath)
	lines := bytes.Count(data, []byte("\n"))
	if lines != 5 {
		t.Errorf("audit event count = %d, want 5", lines)
	}
}

func TestExampleAuditWithHooks(t *testing.T) {
	// Create a fake CLI for testing
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"audit-hooks-example"}'
echo '{"type":"permission","request_id":"req-1","tool":"Bash","tool_input":{"command":"ls -la"}}'
read response
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"Bash","input":{"command":"ls -la"}}]}}'
echo '{"type":"result","result":"Listed files","num_turns":1,"total_cost_usd":0.001}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Collect specific audit events
	var hookEvents []string
	handler := func(e AuditEvent) {
		if strings.HasPrefix(e.Type, "hook.") {
			hookEvents = append(hookEvents, e.Type)
		}
	}

	// Create agent with audit and hooks
	a, err := New(ctx,
		CLIPath(fakeClaude),
		Audit(handler),
		PreToolUse(
			DenyCommands("rm -rf"),
		),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _ = a.Run(ctx, "List files")
	_ = a.Close()

	if len(hookEvents) != 1 {
		t.Errorf("hook event count = %d, want 1", len(hookEvents))
	}
	if len(hookEvents) > 0 && hookEvents[0] != "hook.pre_tool_use" {
		t.Errorf("hookEvents[0] = %q, want %q", hookEvents[0], "hook.pre_tool_use")
	}
}

func TestExampleLifecycleHooks(t *testing.T) {
	// Create a fake CLI that simulates tool use with a result
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
printf '%s\n' '{"type":"system","subtype":"init","session_id":"lifecycle-example"}'
printf '%s\n' '{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_123","name":"Bash","input":{"command":"echo hello"}}]}}'
printf '%s\n' '{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_result","tool_use_id":"toolu_123","content":"hello","is_error":false,"duration_ms":25}]}}'
printf '%s\n' '{"type":"result","result":"Done","num_turns":1,"total_cost_usd":0.0042}'
`
	mustWriteFile(t, fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	var postToolCalled bool
	var stopCalled bool
	var stopReason string
	var stopCost float64

	// Create agent with PostToolUse and OnStop hooks
	a, err := New(ctx,
		CLIPath(fakeClaude),
		PostToolUse(func(tc *ToolCall, tr *ToolResultContext) HookResult {
			postToolCalled = true
			if tc.Name != "Bash" {
				t.Errorf("PostToolUse tool name = %q, want %q", tc.Name, "Bash")
			}
			if tr.IsError {
				t.Errorf("PostToolUse IsError = true, want false")
			}
			return HookResult{Decision: Continue}
		}),
		OnStop(func(e *StopEvent) {
			stopCalled = true
			stopReason = string(e.Reason)
			stopCost = e.CostUSD
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _ = a.Run(ctx, "Run echo hello")
	_ = a.Close()

	if !postToolCalled {
		t.Error("PostToolUse hook was not called")
	}
	if !stopCalled {
		t.Error("OnStop hook was not called")
	}
	if stopReason != "completed" {
		t.Errorf("stopReason = %q, want %q", stopReason, "completed")
	}
	if stopCost != 0.0042 {
		t.Errorf("stopCost = %v, want 0.0042", stopCost)
	}
}
