package agent

import (
	"strings"
)

// DenyCommands returns a PreToolUseHook that blocks Bash commands matching any pattern.
// Patterns are matched using substring containment.
//
// Example:
//
//	agent.PreToolUse(
//	    agent.DenyCommands("sudo", "curl", "wget"),
//	)
func DenyCommands(patterns ...string) PreToolUseHook {
	return func(tc *ToolCall) HookResult {
		if tc.Name != "Bash" {
			return HookResult{Decision: Continue}
		}

		command, ok := tc.Input["command"].(string)
		if !ok {
			return HookResult{Decision: Continue}
		}

		for _, pattern := range patterns {
			if strings.Contains(command, pattern) {
				return HookResult{
					Decision: Deny,
					Reason:   "command contains blocked pattern: " + pattern,
				}
			}
		}

		return HookResult{Decision: Continue}
	}
}

// RequireCommand returns a PreToolUseHook that blocks commands matching any
// of the insteadOf patterns and suggests using the preferred command instead.
//
// Example:
//
//	agent.PreToolUse(
//	    agent.RequireCommand("make", "go build", "go test"),
//	)
//
// This will deny "go build" and "go test" commands, telling Claude to use "make" instead.
func RequireCommand(use string, insteadOf ...string) PreToolUseHook {
	return func(tc *ToolCall) HookResult {
		if tc.Name != "Bash" {
			return HookResult{Decision: Continue}
		}

		command, ok := tc.Input["command"].(string)
		if !ok {
			return HookResult{Decision: Continue}
		}

		for _, pattern := range insteadOf {
			if strings.Contains(command, pattern) {
				return HookResult{
					Decision: Deny,
					Reason:   "use " + use + " instead of " + pattern,
				}
			}
		}

		return HookResult{Decision: Continue}
	}
}
