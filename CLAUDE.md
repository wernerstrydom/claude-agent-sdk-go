# Claude Agent SDK for Go

A Go SDK for programmatic automation of Claude Code CLI.

## Project Structure

```
claude-agent-sdk-go/
├── agent/           # Main package - all SDK code
│   ├── agent.go     # Agent struct, New(), Run(), Stream(), Close()
│   ├── bridge.go    # Message channel pump from process to consumer
│   ├── control.go   # Permission request/response handling
│   ├── errors.go    # Typed errors (StartError, ProcessError, etc.)
│   ├── hooks.go     # Hook system (Decision, HookResult, hookChain)
│   ├── hooks_commands.go  # DenyCommands(), RequireCommand()
│   ├── hooks_paths.go     # AllowPaths(), DenyPaths(), RedirectPath()
│   ├── message.go   # Message types (Text, ToolUse, Result, etc.)
│   ├── options.go   # Functional options pattern
│   ├── parser.go    # JSON line parser for CLI output
│   └── process.go   # CLI process spawning and management
├── spec.md          # API specification (needs updating)
├── plan.md          # 15-step implementation roadmap
└── go.mod
```

## Critical: Stream-JSON Protocol Behavior

The CLI uses `--input-format stream-json` which has specific behavior:

**The CLI waits for the first user message before outputting anything, including the init message.**

This means:
- `New()` does NOT block waiting for `SystemInit`
- Session ID is captured lazily when the first message arrives
- The `SystemInit` message comes AFTER the first `Stream()`/`Run()` call

```go
// This is how it actually works:
a, _ := agent.New(ctx, opts...)  // Starts process, but no init yet
// ... CLI is waiting for input ...
result, _ := a.Run(ctx, "hello") // NOW CLI sends init, then processes
```

## CLI Communication Protocol

### Sending Messages (SDK → CLI)

User messages use this JSON structure:
```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [
      {"type": "text", "text": "the prompt"}
    ]
  }
}
```

Control responses (for permission requests):
```json
{
  "request_id": "...",
  "decision": "allow|deny",
  "reason": "optional feedback",
  "updated_input": {}
}
```

### Receiving Messages (CLI → SDK)

Message types from CLI:
- `system` (subtype: `init`) - Session initialization
- `assistant` - Text, thinking, tool_use content blocks
- `result` - Final result with cost/usage
- `permission` or `control` - Permission requests for tool execution

Key field mappings:
- Cost is `total_cost_usd` (not `cost_usd`)
- Tools in init are `[]string` (just names, not full ToolInfo)
- Duration fields are `duration_ms` and `duration_api_ms`

## Implementation Status

**Completed (Steps 1-6):**
- Core types and message parsing
- Process management
- Agent with Run() and Stream()
- Hook system with PreToolUse
- Higher-order hooks (DenyCommands, AllowPaths, etc.)

**Pending (Steps 7-15):**
- Extended options (Tools, Env, PermissionMode)
- Limits and sessions (MaxTurns, Resume, Fork)
- Structured output
- Audit system
- Additional lifecycle hooks (PostToolUse, Stop, etc.)
- Custom in-process tools
- MCP server configuration
- Subagents and skills

## Testing

```bash
go test ./...           # Run all tests
go test -v ./agent      # Verbose agent tests
go build ./...          # Verify compilation
```

## Hook System

Hooks provide deterministic enforcement. Chain evaluation:
1. First `Deny` wins - blocks execution immediately
2. `Allow` - approves and skips remaining hooks
3. `Continue` - passes to next hook

```go
agent.PreToolUse(
    agent.DenyCommands("rm -rf", "sudo"),
    agent.AllowPaths("/sandbox"),
    customHook,
)
```

## Common Patterns

### Basic Usage
```go
a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
if err != nil {
    log.Fatal(err)
}
defer a.Close()

result, err := a.Run(ctx, "What is 2+2?")
```

### Streaming
```go
for msg := range a.Stream(ctx, "Explain channels") {
    switch m := msg.(type) {
    case *agent.Text:
        fmt.Print(m.Text)
    case *agent.Result:
        fmt.Printf("\nCost: $%.4f\n", m.CostUSD)
    }
}
```

### With Hooks
```go
a, _ := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.PreToolUse(
        agent.DenyCommands("sudo", "rm -rf"),
        agent.AllowPaths(".", "/tmp"),
    ),
)
```

## Key Design Decisions

1. **Single package** - Everything in `agent/`, one import
2. **Functional options** - Composable configuration via `Option` type
3. **Lazy initialization** - Session ID captured on first message, not in New()
4. **Internal control handling** - Permission requests handled automatically, not exposed to user
5. **Channel-based streaming** - Stream() returns `<-chan Message`
