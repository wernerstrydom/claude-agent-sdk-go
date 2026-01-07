package agent

import (
	"strings"
	"testing"
)

func TestDenyCommands_BlocksMatchingCommand(t *testing.T) {
	hook := DenyCommands("sudo")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "sudo apt update"},
	}

	result := hook(tc)

	if result.Decision != Deny {
		t.Errorf("expected Deny, got %v", result.Decision)
	}
	if !strings.Contains(result.Reason, "sudo") {
		t.Errorf("expected reason to mention blocked pattern, got %q", result.Reason)
	}
}

func TestDenyCommands_AllowsNonMatchingCommand(t *testing.T) {
	hook := DenyCommands("sudo")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "ls -la"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue, got %v", result.Decision)
	}
}

func TestDenyCommands_MultiplePatterns(t *testing.T) {
	hook := DenyCommands("sudo", "curl", "wget")

	tests := []struct {
		command  string
		expected Decision
	}{
		{"sudo apt install", Deny},
		{"curl http://example.com", Deny},
		{"wget http://example.com", Deny},
		{"ls -la", Continue},
		{"cat README.md", Continue},
	}

	for _, tt := range tests {
		tc := &ToolCall{
			Name:  "Bash",
			Input: map[string]any{"command": tt.command},
		}
		result := hook(tc)
		if result.Decision != tt.expected {
			t.Errorf("command %q: expected %v, got %v", tt.command, tt.expected, result.Decision)
		}
	}
}

func TestDenyCommands_ContinuesForNonBashTools(t *testing.T) {
	hook := DenyCommands("sudo")

	tc := &ToolCall{
		Name:  "Read",
		Input: map[string]any{"file_path": "/tmp/test.txt"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for non-Bash tool, got %v", result.Decision)
	}
}

func TestDenyCommands_ContinuesForMissingCommand(t *testing.T) {
	hook := DenyCommands("sudo")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{}, // no command field
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for missing command, got %v", result.Decision)
	}
}

func TestRequireCommand_BlocksMatchingPattern(t *testing.T) {
	hook := RequireCommand("make", "go build", "go test")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "go build ./..."},
	}

	result := hook(tc)

	if result.Decision != Deny {
		t.Errorf("expected Deny, got %v", result.Decision)
	}
	if !strings.Contains(result.Reason, "make") {
		t.Errorf("expected reason to suggest 'make', got %q", result.Reason)
	}
	if !strings.Contains(result.Reason, "go build") {
		t.Errorf("expected reason to mention blocked pattern, got %q", result.Reason)
	}
}

func TestRequireCommand_AllowsPreferredCommand(t *testing.T) {
	hook := RequireCommand("make", "go build", "go test")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "make build"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for preferred command, got %v", result.Decision)
	}
}

func TestRequireCommand_ReasonMessageIsHelpful(t *testing.T) {
	hook := RequireCommand("make test", "go test")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "go test -v ./..."},
	}

	result := hook(tc)

	if result.Decision != Deny {
		t.Errorf("expected Deny, got %v", result.Decision)
	}
	// Reason should be "use make test instead of go test"
	expected := "use make test instead of go test"
	if result.Reason != expected {
		t.Errorf("expected reason %q, got %q", expected, result.Reason)
	}
}

func TestRequireCommand_ContinuesForNonBashTools(t *testing.T) {
	hook := RequireCommand("make", "go build")

	tc := &ToolCall{
		Name:  "Write",
		Input: map[string]any{"file_path": "/tmp/test.go"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for non-Bash tool, got %v", result.Decision)
	}
}
