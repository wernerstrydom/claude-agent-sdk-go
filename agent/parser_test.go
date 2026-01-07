package agent

import (
	"io"
	"strings"
	"testing"
)

// Test fixtures representing Claude Code CLI output (stream-json format)
const (
	// Tools is a string array, mcp_servers uses lowercase keys
	systemInitJSON = `{"type":"system","subtype":"init","session_id":"sess-abc123","transcript_path":"/tmp/transcript.jsonl","tools":["Bash"],"mcp_servers":[{"name":"github","status":"connected"}]}`

	// Assistant messages have content in message.content (stream-json format)
	textMessageJSON = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello, I'm Claude!"}]}}`

	thinkingMessageJSON = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Let me analyze this...","signature":"sig123"}]}}`

	toolUseMessageJSON = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"Bash","input":{"command":"ls -la"}}]}}`

	// Result uses total_cost_usd not cost_usd
	resultMessageJSON = `{"type":"result","duration_ms":1500.5,"duration_api_ms":1200.3,"num_turns":3,"total_cost_usd":0.0042,"is_error":false,"result":"Task completed successfully","usage":{"InputTokens":100,"OutputTokens":50,"CacheRead":10,"CacheWrite":5}}`

	resultErrorMessageJSON = `{"type":"result","is_error":true,"result":"An error occurred"}`
)

func TestParseSystemInit(t *testing.T) {
	p := newParser(strings.NewReader(systemInitJSON))

	msg, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	init, ok := msg.(*SystemInit)
	if !ok {
		t.Fatalf("expected *SystemInit, got %T", msg)
	}

	if init.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Errorf("TranscriptPath = %q, want %q", init.TranscriptPath, "/tmp/transcript.jsonl")
	}

	if len(init.Tools) != 1 {
		t.Errorf("len(Tools) = %d, want 1", len(init.Tools))
	} else if init.Tools[0].Name != "Bash" {
		t.Errorf("Tools[0].Name = %q, want %q", init.Tools[0].Name, "Bash")
	}

	if len(init.MCPServers) != 1 {
		t.Errorf("len(MCPServers) = %d, want 1", len(init.MCPServers))
	} else if init.MCPServers[0].Status != "connected" {
		t.Errorf("MCPServers[0].Status = %q, want %q", init.MCPServers[0].Status, "connected")
	}

	// Check session ID was captured
	if p.sessionID != "sess-abc123" {
		t.Errorf("parser.sessionID = %q, want %q", p.sessionID, "sess-abc123")
	}
}

func TestParseTextMessage(t *testing.T) {
	p := newParser(strings.NewReader(textMessageJSON))

	msg, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	text, ok := msg.(*Text)
	if !ok {
		t.Fatalf("expected *Text, got %T", msg)
	}

	if text.Text != "Hello, I'm Claude!" {
		t.Errorf("Text = %q, want %q", text.Text, "Hello, I'm Claude!")
	}
}

func TestParseThinkingMessage(t *testing.T) {
	p := newParser(strings.NewReader(thinkingMessageJSON))

	msg, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	thinking, ok := msg.(*Thinking)
	if !ok {
		t.Fatalf("expected *Thinking, got %T", msg)
	}

	if thinking.Thinking != "Let me analyze this..." {
		t.Errorf("Thinking = %q, want %q", thinking.Thinking, "Let me analyze this...")
	}

	if thinking.Signature != "sig123" {
		t.Errorf("Signature = %q, want %q", thinking.Signature, "sig123")
	}
}

func TestParseToolUseMessage(t *testing.T) {
	p := newParser(strings.NewReader(toolUseMessageJSON))

	msg, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	toolUse, ok := msg.(*ToolUse)
	if !ok {
		t.Fatalf("expected *ToolUse, got %T", msg)
	}

	if toolUse.ID != "tool-1" {
		t.Errorf("ID = %q, want %q", toolUse.ID, "tool-1")
	}

	if toolUse.Name != "Bash" {
		t.Errorf("Name = %q, want %q", toolUse.Name, "Bash")
	}

	cmd, ok := toolUse.Input["command"].(string)
	if !ok {
		t.Fatalf("Input[command] not a string")
	}
	if cmd != "ls -la" {
		t.Errorf("Input[command] = %q, want %q", cmd, "ls -la")
	}
}

func TestParseResultMessage(t *testing.T) {
	p := newParser(strings.NewReader(resultMessageJSON))

	msg, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	result, ok := msg.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", msg)
	}

	if result.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want 3", result.NumTurns)
	}

	if result.CostUSD != 0.0042 {
		t.Errorf("CostUSD = %f, want 0.0042", result.CostUSD)
	}

	if result.IsError {
		t.Error("IsError = true, want false")
	}

	if result.ResultText != "Task completed successfully" {
		t.Errorf("ResultText = %q, want %q", result.ResultText, "Task completed successfully")
	}

	if result.Usage.InputTokens != 100 {
		t.Errorf("Usage.InputTokens = %d, want 100", result.Usage.InputTokens)
	}

	if result.Usage.OutputTokens != 50 {
		t.Errorf("Usage.OutputTokens = %d, want 50", result.Usage.OutputTokens)
	}
}

func TestParseResultErrorMessage(t *testing.T) {
	p := newParser(strings.NewReader(resultErrorMessageJSON))

	msg, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	result, ok := msg.(*Result)
	if !ok {
		t.Fatalf("expected *Result, got %T", msg)
	}

	if !result.IsError {
		t.Error("IsError = false, want true")
	}
}

func TestParseMalformedJSON(t *testing.T) {
	p := newParser(strings.NewReader(`{invalid json`))

	_, err := p.next()
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseUnknownMessageType(t *testing.T) {
	p := newParser(strings.NewReader(`{"type":"unknown","content":"something"}`))

	msg, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v, want graceful handling", err)
	}

	// Should return as Text
	text, ok := msg.(*Text)
	if !ok {
		t.Fatalf("expected *Text for unknown type, got %T", msg)
	}

	// Content should be preserved
	if text.Text != `"something"` {
		t.Errorf("Text = %q, want %q", text.Text, `"something"`)
	}
}

func TestParseEmptyLines(t *testing.T) {
	input := "\n\n" + textMessageJSON + "\n\n"
	p := newParser(strings.NewReader(input))

	msg, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	_, ok := msg.(*Text)
	if !ok {
		t.Fatalf("expected *Text, got %T", msg)
	}
}

func TestParseEOF(t *testing.T) {
	p := newParser(strings.NewReader(""))

	_, err := p.next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestMessageMetaPopulation(t *testing.T) {
	input := systemInitJSON + "\n" + textMessageJSON
	p := newParser(strings.NewReader(input))

	// First message
	msg1, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	init := msg1.(*SystemInit)
	if init.Turn != 1 {
		t.Errorf("first message Turn = %d, want 1", init.Turn)
	}
	if init.Sequence != 1 {
		t.Errorf("first message Sequence = %d, want 1", init.Sequence)
	}
	if init.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	// Second message should have session ID from init
	msg2, err := p.next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}

	text := msg2.(*Text)
	if text.SessionID != "sess-abc123" {
		t.Errorf("SessionID = %q, want %q", text.SessionID, "sess-abc123")
	}
	if text.Sequence != 2 {
		t.Errorf("second message Sequence = %d, want 2", text.Sequence)
	}
}

func TestParseMultipleMessages(t *testing.T) {
	input := systemInitJSON + "\n" + textMessageJSON + "\n" + toolUseMessageJSON + "\n" + resultMessageJSON
	p := newParser(strings.NewReader(input))

	// Parse all messages
	var messages []Message
	for {
		msg, err := p.next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next() error = %v", err)
		}
		messages = append(messages, msg)
	}

	if len(messages) != 4 {
		t.Errorf("got %d messages, want 4", len(messages))
	}

	// Verify types in order
	if _, ok := messages[0].(*SystemInit); !ok {
		t.Errorf("message 0: expected *SystemInit, got %T", messages[0])
	}
	if _, ok := messages[1].(*Text); !ok {
		t.Errorf("message 1: expected *Text, got %T", messages[1])
	}
	if _, ok := messages[2].(*ToolUse); !ok {
		t.Errorf("message 2: expected *ToolUse, got %T", messages[2])
	}
	if _, ok := messages[3].(*Result); !ok {
		t.Errorf("message 3: expected *Result, got %T", messages[3])
	}
}
