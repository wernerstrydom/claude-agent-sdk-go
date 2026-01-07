package agent

import (
	"strings"
)

// pathTools is the list of tools that operate on file paths.
var pathTools = []string{"Read", "Write", "Edit", "MultiEdit"}

// isPathTool checks if the tool name is a path-operating tool.
func isPathTool(name string) bool {
	for _, t := range pathTools {
		if name == t {
			return true
		}
	}
	return false
}

// extractPath gets the file path from tool input.
// Different tools use different field names.
func extractPath(input map[string]any) (string, bool) {
	// Try file_path first (most common)
	if p, ok := input["file_path"].(string); ok {
		return p, true
	}
	// Try path
	if p, ok := input["path"].(string); ok {
		return p, true
	}
	return "", false
}

// AllowPaths returns a PreToolUseHook that only allows file operations on paths
// that start with one of the allowed prefixes. All other paths are denied.
//
// Example:
//
//	agent.PreToolUse(
//	    agent.AllowPaths("/sandbox", "/tmp"),
//	)
func AllowPaths(paths ...string) PreToolUseHook {
	return func(tc *ToolCall) HookResult {
		if !isPathTool(tc.Name) {
			return HookResult{Decision: Continue}
		}

		path, ok := extractPath(tc.Input)
		if !ok {
			return HookResult{Decision: Continue}
		}

		for _, allowed := range paths {
			if strings.HasPrefix(path, allowed) {
				return HookResult{Decision: Continue}
			}
		}

		return HookResult{
			Decision: Deny,
			Reason:   "path not in allowed list: " + path,
		}
	}
}

// DenyPaths returns a PreToolUseHook that blocks file operations on paths
// that start with any of the denied prefixes.
//
// Example:
//
//	agent.PreToolUse(
//	    agent.DenyPaths("/etc", "/usr", "~/.ssh"),
//	)
func DenyPaths(paths ...string) PreToolUseHook {
	return func(tc *ToolCall) HookResult {
		if !isPathTool(tc.Name) {
			return HookResult{Decision: Continue}
		}

		path, ok := extractPath(tc.Input)
		if !ok {
			return HookResult{Decision: Continue}
		}

		for _, denied := range paths {
			if strings.HasPrefix(path, denied) {
				return HookResult{
					Decision: Deny,
					Reason:   "path is in denied list: " + path,
				}
			}
		}

		return HookResult{Decision: Continue}
	}
}

// RedirectPath returns a PreToolUseHook that rewrites file paths.
// If a path starts with 'from', it is rewritten to start with 'to'.
// The hook returns Allow with UpdatedInput to apply the rewrite.
//
// Example:
//
//	agent.PreToolUse(
//	    agent.RedirectPath("/tmp", "/sandbox/tmp"),
//	)
//
// A path like "/tmp/foo.txt" becomes "/sandbox/tmp/foo.txt".
func RedirectPath(from, to string) PreToolUseHook {
	return func(tc *ToolCall) HookResult {
		if !isPathTool(tc.Name) {
			return HookResult{Decision: Continue}
		}

		path, ok := extractPath(tc.Input)
		if !ok {
			return HookResult{Decision: Continue}
		}

		if !strings.HasPrefix(path, from) {
			return HookResult{Decision: Continue}
		}

		// Rewrite the path
		newPath := to + strings.TrimPrefix(path, from)

		// Determine which field to update
		fieldName := "file_path"
		if _, ok := tc.Input["path"]; ok {
			fieldName = "path"
		}

		return HookResult{
			Decision: Allow,
			UpdatedInput: map[string]any{
				fieldName: newPath,
			},
		}
	}
}
