# Hooks

Hooks provide a mechanism to intercept and control agent operations at defined points in the execution flow. They enable
security policies, observability, and customization without modifying the core SDK behavior. This document explains the
hook system, chain evaluation rules, built-in hooks, and how to write custom hooks.

## Why Hooks Matter

When Claude executes tools, it can read files, run commands, and modify your system. Hooks provide deterministic
enforcement over these operations:

- **Security**: Block dangerous commands or restrict file access to specific directories
- **Compliance**: Enforce organizational policies about what operations are permitted
- **Observability**: Log tool executions and their results for audit purposes
- **Customization**: Modify tool inputs or redirect operations without changing prompts

Without hooks, you would need to trust Claude to follow prompt-based instructions, which provides no guarantee of
enforcement. Hooks intercept operations at the SDK level, providing programmatic control.

## Hook Types

The SDK supports several hook types, each serving a different purpose:

| Hook Type          | When Called               | Can Modify | Primary Use                        |
|--------------------|---------------------------|------------|------------------------------------|
| `PreToolUse`       | Before tool execution     | Yes        | Block, allow, or modify tool calls |
| `PostToolUse`      | After tool execution      | No         | Observe results, log metrics       |
| `OnStop`           | When agent closes         | No         | Cleanup, final metrics             |
| `PreCompact`       | Before context compaction | No         | Archive transcript                 |
| `SubagentStop`     | When subagent completes   | No         | Track subagent costs               |
| `UserPromptSubmit` | Before prompt is sent     | Yes        | Modify prompts, add context        |

This document focuses on `PreToolUse` hooks, which are the most commonly used for security and control.

## PreToolUse Hook Signature

A `PreToolUse` hook receives a `ToolCall` and returns a `HookResult`:

```go
type PreToolUseHook func(*ToolCall) HookResult

type ToolCall struct {
    Name  string         // Tool name: "Bash", "Read", "Write", "Edit", etc.
    Input map[string]any // Tool-specific input parameters
}

type HookResult struct {
    Decision     Decision       // Allow, Deny, or Continue
    Reason       string         // Feedback to Claude when denying
    UpdatedInput map[string]any // Modified inputs (optional)
}
```

The `Decision` type determines what happens next:

```go
const (
    Continue Decision = iota // Pass to next hook in chain
    Allow                    // Approve operation, skip remaining hooks
    Deny                     // Block operation immediately
)
```

## Hook Chain Evaluation

When multiple hooks are registered, they form a chain that evaluates in order. The evaluation follows these rules:

1. **First Deny wins**: If any hook returns `Deny`, the operation is blocked immediately. Remaining hooks are not
   evaluated.

2. **Allow short-circuits**: If a hook returns `Allow`, the operation is approved and remaining hooks are skipped.

3. **Continue passes through**: If a hook returns `Continue`, evaluation proceeds to the next hook. If all hooks return
   `Continue`, the operation is allowed.

This diagram illustrates the evaluation flow:

```
Hook 1 ──Continue──> Hook 2 ──Continue──> Hook 3 ──Continue──> Allow
    │                    │                    │
    Deny                 Deny                 Deny
    │                    │                    │
    v                    v                    v
  Block                Block                Block

If any returns Allow: Skip remaining hooks, proceed with operation
```

### Evaluation Example

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        hookA, // Returns Continue
        hookB, // Returns Deny
        hookC, // Never evaluated
    ),
)
```

In this example, `hookA` passes evaluation to `hookB`, which denies the operation. `hookC` is never called.

### Input Accumulation

When hooks return `UpdatedInput`, the changes accumulate through the chain:

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        func(tc *ToolCall) HookResult {
            // Add timeout to all commands
            return HookResult{
                Decision:     Continue,
                UpdatedInput: map[string]any{"timeout": 30},
            }
        },
        func(tc *ToolCall) HookResult {
            // tc.Input now includes timeout from previous hook
            return HookResult{Decision: Continue}
        },
    ),
)
```

## Built-in Hooks

The SDK provides several pre-built hooks for common security patterns. These are defined in `agent/hooks_commands.go`
and `agent/hooks_paths.go`.

### DenyCommands

Blocks Bash commands containing any of the specified patterns. Pattern matching uses substring containment.

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        agent.DenyCommands("sudo", "rm -rf", "curl", "wget"),
    ),
)
```

When Claude attempts a denied command, it receives feedback:

```
command contains blocked pattern: sudo
```

Implementation details:

- Only applies to the `Bash` tool
- Checks the `command` input field
- Returns `Continue` for non-Bash tools
- Returns `Deny` on first pattern match

### RequireCommand

Blocks commands matching patterns and suggests an alternative. Use this to enforce build system conventions.

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        agent.RequireCommand("make", "go build", "go test"),
    ),
)
```

When Claude attempts `go build`, it receives:

```
use make instead of go build
```

This guides Claude toward using your project's build system rather than invoking tools directly.

### AllowPaths

Restricts file operations to paths starting with allowed prefixes. All other paths are denied.

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        agent.AllowPaths("/sandbox", "/tmp", "./src"),
    ),
)
```

Applies to file tools: `Read`, `Write`, `Edit`, `MultiEdit`.

When Claude attempts to access a disallowed path:

```
path not in allowed list: /etc/passwd
```

This hook uses prefix matching, so `AllowPaths("/sandbox")` allows `/sandbox/foo/bar.txt`.

### DenyPaths

Blocks file operations on paths starting with denied prefixes.

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        agent.DenyPaths("/etc", "/usr", "~/.ssh", ".env"),
    ),
)
```

When Claude attempts to access a denied path:

```
path is in denied list: /etc/passwd
```

Use `DenyPaths` for blocklist-style security, `AllowPaths` for allowlist-style.

### RedirectPath

Rewrites file paths from one prefix to another. Use this to sandbox file operations without Claude knowing the actual
paths.

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        agent.RedirectPath("/tmp", "/sandbox/tmp"),
    ),
)
```

When Claude attempts to write to `/tmp/output.txt`, the operation is redirected to `/sandbox/tmp/output.txt`.

Unlike other hooks, `RedirectPath` returns `Allow` with `UpdatedInput` to apply the path change.

## Writing Custom Hooks

Custom hooks follow the same signature as built-in hooks. Here are common patterns.

### Basic Custom Hook

```go
func logAllTools(tc *agent.ToolCall) agent.HookResult {
    log.Printf("Tool: %s, Input: %v", tc.Name, tc.Input)
    return agent.HookResult{Decision: agent.Continue}
}

a, _ := agent.New(ctx,
    agent.PreToolUse(logAllTools),
)
```

### Conditional Allow

```go
func allowGitOnly(tc *agent.ToolCall) agent.HookResult {
    if tc.Name != "Bash" {
        return agent.HookResult{Decision: agent.Continue}
    }

    command, ok := tc.Input["command"].(string)
    if !ok {
        return agent.HookResult{Decision: agent.Continue}
    }

    if strings.HasPrefix(command, "git ") {
        return agent.HookResult{
            Decision: agent.Allow,
            Reason:   "git commands are always allowed",
        }
    }

    return agent.HookResult{Decision: agent.Continue}
}
```

### Input Modification

```go
func addWorkDir(tc *agent.ToolCall) agent.HookResult {
    if tc.Name != "Bash" {
        return agent.HookResult{Decision: agent.Continue}
    }

    command, _ := tc.Input["command"].(string)
    prefixed := "cd /workspace && " + command

    return agent.HookResult{
        Decision: agent.Allow,
        UpdatedInput: map[string]any{
            "command": prefixed,
        },
    }
}
```

### Conditional Deny with Context

```go
type rateLimiter struct {
    mu     sync.Mutex
    counts map[string]int
    limit  int
}

func (r *rateLimiter) hook(tc *agent.ToolCall) agent.HookResult {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.counts[tc.Name]++
    if r.counts[tc.Name] > r.limit {
        return agent.HookResult{
            Decision: agent.Deny,
            Reason:   fmt.Sprintf("rate limit exceeded for %s", tc.Name),
        }
    }

    return agent.HookResult{Decision: agent.Continue}
}

func RateLimit(limit int) agent.PreToolUseHook {
    r := &rateLimiter{
        counts: make(map[string]int),
        limit:  limit,
    }
    return r.hook
}
```

## Composing Hooks

Combine built-in and custom hooks to create layered security policies.

### Security-First Ordering

Order hooks from most restrictive to least restrictive:

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        // Layer 1: Hard blocks
        agent.DenyCommands("sudo", "rm -rf /"),
        agent.DenyPaths("/etc", "/usr", "~/.ssh"),

        // Layer 2: Sandboxing
        agent.AllowPaths("/sandbox", "/tmp"),
        agent.RedirectPath("/tmp", "/sandbox/tmp"),

        // Layer 3: Conventions
        agent.RequireCommand("make", "go build", "go test"),

        // Layer 4: Logging (always passes)
        logAllTools,
    ),
)
```

### Environment-Specific Policies

```go
func productionHooks() []agent.PreToolUseHook {
    return []agent.PreToolUseHook{
        agent.DenyCommands("sudo", "rm -rf", "curl", "wget"),
        agent.DenyPaths("/etc", "/var", "/usr"),
        agent.AllowPaths("/app/data"),
    }
}

func developmentHooks() []agent.PreToolUseHook {
    return []agent.PreToolUseHook{
        agent.DenyCommands("sudo", "rm -rf /"),
        // More permissive in development
    }
}

var hooks []agent.PreToolUseHook
if os.Getenv("ENV") == "production" {
    hooks = productionHooks()
} else {
    hooks = developmentHooks()
}

a, _ := agent.New(ctx, agent.PreToolUse(hooks...))
```

## Other Hook Types

### PostToolUse

Observe tool results after execution. Cannot modify or block results.

```go
a, _ := agent.New(ctx,
    agent.PostToolUse(func(tc *agent.ToolCall, tr *agent.ToolResultContext) agent.HookResult {
        log.Printf("Tool %s completed in %v (error: %v)",
            tc.Name, tr.Duration, tr.IsError)
        return agent.HookResult{Decision: agent.Continue}
    }),
)
```

The `ToolResultContext` provides:

| Field       | Type            | Description                               |
|-------------|-----------------|-------------------------------------------|
| `ToolUseID` | `string`        | Unique identifier for the tool invocation |
| `Content`   | `any`           | Result returned by the tool               |
| `IsError`   | `bool`          | Whether execution resulted in an error    |
| `Duration`  | `time.Duration` | Execution time                            |

### OnStop

Called when the agent closes. Use for cleanup and final metrics.

```go
a, _ := agent.New(ctx,
    agent.OnStop(func(e *agent.StopEvent) {
        log.Printf("Session %s ended: reason=%s, turns=%d, cost=$%.4f",
            e.SessionID, e.Reason, e.NumTurns, e.CostUSD)
    }),
)
```

The `StopReason` can be:

- `StopCompleted` - Normal completion
- `StopMaxTurns` - Hit turn limit
- `StopInterrupted` - Context cancelled
- `StopError` - Error occurred

### UserPromptSubmit

Modify prompts before they are sent to Claude.

```go
a, _ := agent.New(ctx,
    agent.UserPromptSubmit(func(e *agent.PromptSubmitEvent) agent.PromptSubmitResult {
        // Add context to all prompts
        return agent.PromptSubmitResult{
            UpdatedPrompt: e.Prompt + "\n[Environment: staging]",
            Metadata: map[string]any{
                "original_length": len(e.Prompt),
            },
        }
    }),
)
```

## Complete Example

The following example demonstrates a comprehensive hook setup for a sandboxed code execution environment:

```go
package main

import (
    "context"
    "log"
    "strings"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.WorkDir("/sandbox/project"),

        // PreToolUse hooks for security
        agent.PreToolUse(
            // Block dangerous commands
            agent.DenyCommands("sudo", "rm -rf /", "curl", "wget", "nc"),

            // Block sensitive paths
            agent.DenyPaths("/etc", "/var", "/usr", "~/.ssh", ".env"),

            // Restrict file access to sandbox
            agent.AllowPaths("/sandbox"),

            // Redirect /tmp writes
            agent.RedirectPath("/tmp", "/sandbox/tmp"),

            // Enforce build system
            agent.RequireCommand("make", "go build", "go test", "npm run"),

            // Custom: log all tool calls
            func(tc *agent.ToolCall) agent.HookResult {
                log.Printf("[TOOL] %s: %v", tc.Name, tc.Input)
                return agent.HookResult{Decision: agent.Continue}
            },
        ),

        // PostToolUse for metrics
        agent.PostToolUse(func(tc *agent.ToolCall, tr *agent.ToolResultContext) agent.HookResult {
            if tr.IsError {
                log.Printf("[ERROR] %s failed in %v", tc.Name, tr.Duration)
            }
            return agent.HookResult{Decision: agent.Continue}
        }),

        // OnStop for session summary
        agent.OnStop(func(e *agent.StopEvent) {
            log.Printf("[SESSION END] %s: %s, %d turns, $%.4f",
                e.SessionID, e.Reason, e.NumTurns, e.CostUSD)
        }),
    )
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }
    defer a.Close()

    result, err := a.Run(ctx, "Create a hello world Go program in /sandbox/project")
    if err != nil {
        log.Fatalf("Run failed: %v", err)
    }

    log.Printf("Result: %s", result.ResultText)
}
```

## Related Documentation

- [Agents](agents.md) - Agent lifecycle and execution modes
- [Sessions](sessions.md) - Session management and state
- Source files:
    - `/Users/wstrydom/Developer/wernerstrydom/claude-agent-sdk-go/agent/hooks.go`
    - `/Users/wstrydom/Developer/wernerstrydom/claude-agent-sdk-go/agent/hooks_commands.go`
    - `/Users/wstrydom/Developer/wernerstrydom/claude-agent-sdk-go/agent/hooks_paths.go`
