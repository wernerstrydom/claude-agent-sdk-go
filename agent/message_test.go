package agent

import (
	"testing"
	"time"
)

func TestMessageTypesImplementMessage(t *testing.T) {
	// Compile-time check that all types implement Message
	var _ Message = SystemInit{}
	var _ Message = Text{}
	var _ Message = Thinking{}
	var _ Message = ToolUse{}
	var _ Message = ToolResult{}
	var _ Message = Result{}
	var _ Message = Error{}
}

func TestMessageMetaAccessible(t *testing.T) {
	now := time.Now()
	meta := MessageMeta{
		Timestamp:  now,
		SessionID:  "test-session",
		Turn:       1,
		Sequence:   2,
		ParentID:   "parent-123",
		SubagentID: "subagent-456",
	}

	tests := []struct {
		name string
		msg  Message
		meta MessageMeta
	}{
		{
			name: "SystemInit",
			msg:  SystemInit{MessageMeta: meta, TranscriptPath: "/tmp/transcript"},
			meta: meta,
		},
		{
			name: "Text",
			msg:  Text{MessageMeta: meta, Text: "hello"},
			meta: meta,
		},
		{
			name: "Thinking",
			msg:  Thinking{MessageMeta: meta, Thinking: "reasoning", Signature: "sig"},
			meta: meta,
		},
		{
			name: "ToolUse",
			msg:  ToolUse{MessageMeta: meta, ID: "tool-1", Name: "Bash", Input: map[string]any{"command": "ls"}},
			meta: meta,
		},
		{
			name: "ToolResult",
			msg:  ToolResult{MessageMeta: meta, ToolUseID: "tool-1", Content: "output", IsError: false, Duration: time.Second},
			meta: meta,
		},
		{
			name: "Result",
			msg:  Result{MessageMeta: meta, NumTurns: 5, CostUSD: 0.01, ResultText: "done"},
			meta: meta,
		},
		{
			name: "Error",
			msg:  Error{MessageMeta: meta, Err: nil},
			meta: meta,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Access MessageMeta through type assertion
			switch m := tt.msg.(type) {
			case SystemInit:
				if m.SessionID != tt.meta.SessionID {
					t.Errorf("SessionID = %q, want %q", m.SessionID, tt.meta.SessionID)
				}
			case Text:
				if m.SessionID != tt.meta.SessionID {
					t.Errorf("SessionID = %q, want %q", m.SessionID, tt.meta.SessionID)
				}
			case Thinking:
				if m.SessionID != tt.meta.SessionID {
					t.Errorf("SessionID = %q, want %q", m.SessionID, tt.meta.SessionID)
				}
			case ToolUse:
				if m.SessionID != tt.meta.SessionID {
					t.Errorf("SessionID = %q, want %q", m.SessionID, tt.meta.SessionID)
				}
			case ToolResult:
				if m.SessionID != tt.meta.SessionID {
					t.Errorf("SessionID = %q, want %q", m.SessionID, tt.meta.SessionID)
				}
			case Result:
				if m.SessionID != tt.meta.SessionID {
					t.Errorf("SessionID = %q, want %q", m.SessionID, tt.meta.SessionID)
				}
			case Error:
				if m.SessionID != tt.meta.SessionID {
					t.Errorf("SessionID = %q, want %q", m.SessionID, tt.meta.SessionID)
				}
			}
		})
	}
}

func TestSupportingTypes(t *testing.T) {
	// Test ToolInfo
	ti := ToolInfo{Name: "Bash", Description: "Execute commands"}
	if ti.Name != "Bash" {
		t.Errorf("ToolInfo.Name = %q, want %q", ti.Name, "Bash")
	}

	// Test MCPStatus
	ms := MCPStatus{Name: "github", Status: "connected"}
	if ms.Name != "github" {
		t.Errorf("MCPStatus.Name = %q, want %q", ms.Name, "github")
	}

	// Test Usage
	u := Usage{InputTokens: 100, OutputTokens: 50, CacheRead: 10, CacheWrite: 5}
	if u.InputTokens != 100 {
		t.Errorf("Usage.InputTokens = %d, want %d", u.InputTokens, 100)
	}
}
