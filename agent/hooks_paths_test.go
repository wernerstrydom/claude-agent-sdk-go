package agent

import (
	"testing"
)

func TestAllowPaths_AllowsMatchingPath(t *testing.T) {
	hook := AllowPaths("/sandbox")

	tc := &ToolCall{
		Name:  "Read",
		Input: map[string]any{"file_path": "/sandbox/file.txt"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for allowed path, got %v", result.Decision)
	}
}

func TestAllowPaths_DeniesNonMatchingPath(t *testing.T) {
	hook := AllowPaths("/sandbox")

	tc := &ToolCall{
		Name:  "Read",
		Input: map[string]any{"file_path": "/etc/passwd"},
	}

	result := hook(tc)

	if result.Decision != Deny {
		t.Errorf("expected Deny for non-allowed path, got %v", result.Decision)
	}
}

func TestAllowPaths_MultiplePaths(t *testing.T) {
	hook := AllowPaths("/sandbox", "/tmp", "/home/user/project")

	tests := []struct {
		path     string
		expected Decision
	}{
		{"/sandbox/foo.txt", Continue},
		{"/tmp/bar.txt", Continue},
		{"/home/user/project/main.go", Continue},
		{"/etc/passwd", Deny},
		{"/usr/bin/ls", Deny},
	}

	for _, tt := range tests {
		tc := &ToolCall{
			Name:  "Write",
			Input: map[string]any{"file_path": tt.path},
		}
		result := hook(tc)
		if result.Decision != tt.expected {
			t.Errorf("path %q: expected %v, got %v", tt.path, tt.expected, result.Decision)
		}
	}
}

func TestAllowPaths_ContinuesForNonPathTools(t *testing.T) {
	hook := AllowPaths("/sandbox")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "cat /etc/passwd"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for non-path tool, got %v", result.Decision)
	}
}

func TestDenyPaths_DeniesMatchingPath(t *testing.T) {
	hook := DenyPaths("/etc")

	tc := &ToolCall{
		Name:  "Read",
		Input: map[string]any{"file_path": "/etc/passwd"},
	}

	result := hook(tc)

	if result.Decision != Deny {
		t.Errorf("expected Deny for denied path, got %v", result.Decision)
	}
}

func TestDenyPaths_AllowsNonMatchingPath(t *testing.T) {
	hook := DenyPaths("/etc")

	tc := &ToolCall{
		Name:  "Read",
		Input: map[string]any{"file_path": "/tmp/file.txt"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for non-denied path, got %v", result.Decision)
	}
}

func TestDenyPaths_MultiplePaths(t *testing.T) {
	hook := DenyPaths("/etc", "/usr", "/var")

	tests := []struct {
		path     string
		expected Decision
	}{
		{"/etc/passwd", Deny},
		{"/usr/bin/ls", Deny},
		{"/var/log/syslog", Deny},
		{"/tmp/file.txt", Continue},
		{"/home/user/file.txt", Continue},
	}

	for _, tt := range tests {
		tc := &ToolCall{
			Name:  "Edit",
			Input: map[string]any{"file_path": tt.path},
		}
		result := hook(tc)
		if result.Decision != tt.expected {
			t.Errorf("path %q: expected %v, got %v", tt.path, tt.expected, result.Decision)
		}
	}
}

func TestDenyPaths_ContinuesForNonPathTools(t *testing.T) {
	hook := DenyPaths("/etc")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "cat /etc/passwd"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for non-path tool, got %v", result.Decision)
	}
}

func TestRedirectPath_RewritesMatchingPath(t *testing.T) {
	hook := RedirectPath("/tmp", "/sandbox/tmp")

	tc := &ToolCall{
		Name:  "Write",
		Input: map[string]any{"file_path": "/tmp/foo.txt"},
	}

	result := hook(tc)

	if result.Decision != Allow {
		t.Errorf("expected Allow for redirected path, got %v", result.Decision)
	}

	newPath, ok := result.UpdatedInput["file_path"].(string)
	if !ok {
		t.Fatal("expected UpdatedInput to contain file_path")
	}
	if newPath != "/sandbox/tmp/foo.txt" {
		t.Errorf("expected /sandbox/tmp/foo.txt, got %q", newPath)
	}
}

func TestRedirectPath_ContinuesForNonMatchingPath(t *testing.T) {
	hook := RedirectPath("/tmp", "/sandbox/tmp")

	tc := &ToolCall{
		Name:  "Read",
		Input: map[string]any{"file_path": "/home/user/file.txt"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for non-matching path, got %v", result.Decision)
	}
}

func TestRedirectPath_ReturnsAllowNotContinue(t *testing.T) {
	// RedirectPath should return Allow (not Continue) to ensure the rewrite is applied
	hook := RedirectPath("/tmp", "/sandbox/tmp")

	tc := &ToolCall{
		Name:  "Write",
		Input: map[string]any{"file_path": "/tmp/test.txt"},
	}

	result := hook(tc)

	if result.Decision != Allow {
		t.Errorf("expected Allow (to apply rewrite), got %v", result.Decision)
	}
}

func TestRedirectPath_ContinuesForNonPathTools(t *testing.T) {
	hook := RedirectPath("/tmp", "/sandbox/tmp")

	tc := &ToolCall{
		Name:  "Bash",
		Input: map[string]any{"command": "ls /tmp"},
	}

	result := hook(tc)

	if result.Decision != Continue {
		t.Errorf("expected Continue for non-path tool, got %v", result.Decision)
	}
}

func TestRedirectPath_HandlesPathField(t *testing.T) {
	// Some tools might use "path" instead of "file_path"
	hook := RedirectPath("/tmp", "/sandbox/tmp")

	tc := &ToolCall{
		Name:  "Read",
		Input: map[string]any{"path": "/tmp/data.json"},
	}

	result := hook(tc)

	if result.Decision != Allow {
		t.Errorf("expected Allow for redirected path, got %v", result.Decision)
	}

	newPath, ok := result.UpdatedInput["path"].(string)
	if !ok {
		t.Fatal("expected UpdatedInput to contain path")
	}
	if newPath != "/sandbox/tmp/data.json" {
		t.Errorf("expected /sandbox/tmp/data.json, got %q", newPath)
	}
}

func TestPathTools_AllRecognized(t *testing.T) {
	// Test that all path tools are recognized
	tools := []string{"Read", "Write", "Edit", "MultiEdit"}

	for _, tool := range tools {
		if !isPathTool(tool) {
			t.Errorf("expected %q to be recognized as path tool", tool)
		}
	}
}

func TestPathTools_NonPathToolsNotRecognized(t *testing.T) {
	nonPathTools := []string{"Bash", "Glob", "Grep", "Task", "WebSearch"}

	for _, tool := range nonPathTools {
		if isPathTool(tool) {
			t.Errorf("expected %q to NOT be recognized as path tool", tool)
		}
	}
}
