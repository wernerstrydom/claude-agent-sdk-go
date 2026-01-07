# Claude Agent SDK for Go

The Claude Agent SDK for Go provides programmatic automation of the Claude Code CLI. It enables Go applications to
interact with Claude as an AI agent that can execute tools, read and write files, and perform complex multi-turn tasks.

## Why This SDK Exists

The Claude Code CLI operates as a standalone command-line tool. While effective for interactive use, many applications
require programmatic control over Claude's capabilities. This SDK bridges that gap by wrapping the CLI in a Go-native
interface that handles:

- Process lifecycle management
- JSON streaming protocol parsing
- Permission request handling via hooks
- Session state and turn tracking
- Cost accumulation across interactions

The result is a single Go package that treats Claude as a controllable agent rather than an interactive CLI.

## Documentation

### Getting Started

- [Installation](getting-started/installation.md) - Prerequisites, installation, and verification steps

### Usage Examples

- [Plan Loop](usage/plan-loop.md) - Ralph-style iterate-until-done with JSON plans
- [Batch Generation](usage/batch-generation.md) - Generate programs across many languages concurrently
- [Driver Scaffolding](usage/driver-scaffolding.md) - Scaffold interface implementations for multiple backends
- [Repository Maintenance](usage/repository-maintenance.md) - Check repositories for updates and issues

### Concepts

- [Agents](concepts/agents.md) - Agent lifecycle, streaming, and error handling
- [Sessions](concepts/sessions.md) - Session management, resumption, and forking
- [Hooks](concepts/hooks.md) - Interception points for tool calls and lifecycle events
- [Structured Output](concepts/structured-output.md) - JSON schema-constrained responses for automation
- [Subagents](concepts/subagents.md) - Child agents for delegated task execution
- [MCP Servers](concepts/mcp-servers.md) - External tool providers via Model Context Protocol
- [Skills](concepts/skills.md) - Domain knowledge injection and system prompt customization
- [Audit System](concepts/audit.md) - Event logging and observability

### Tutorials

- [Tutorial Series Overview](tutorials/README.md) - Build a TODO web application progressively using the SDK

## Core Concepts

### Agent Lifecycle

An agent represents a Claude Code session. The lifecycle consists of:

1. **New** - Create an agent with configuration options
2. **Run/Stream** - Send prompts and receive responses
3. **Close** - Terminate the session and release resources

```go
a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
if err != nil {
log.Fatal(err)
}
defer a.Close()

result, err := a.Run(ctx, "What is 2+2?")
```

### Message Types

The SDK defines several message types that can appear during streaming:

| Type         | Description                      |
|--------------|----------------------------------|
| `Text`       | Assistant text output            |
| `Thinking`   | Extended thinking content        |
| `ToolUse`    | Tool invocation by Claude        |
| `ToolResult` | Result from tool execution       |
| `Result`     | Final result with cost and usage |
| `Error`      | Error during execution           |

### Hooks

Hooks intercept operations at defined points in the execution flow. The SDK supports several hook types:

| Hook               | Purpose                                                           |
|--------------------|-------------------------------------------------------------------|
| `PreToolUse`       | Intercept tool calls before execution; can allow, deny, or modify |
| `PostToolUse`      | Observe tool results after execution                              |
| `OnStop`           | React to session termination                                      |
| `PreCompact`       | Archive context before compaction                                 |
| `SubagentStop`     | Observe subagent completion                                       |
| `UserPromptSubmit` | Modify prompts before sending                                     |

Hook chains evaluate in order. For `PreToolUse`, the first `Deny` wins immediately, `Allow` short-circuits remaining
hooks, and `Continue` passes to the next hook.

### Structured Output

Configure an agent to return JSON matching a schema:

```go
type Answer struct {
Value int `json:"value" desc:"The numeric answer"`
}

var answer Answer
result, err := agent.RunStructured(ctx, "What is 2+2?", &answer)
```

## Feature Status

The following table indicates the implementation status of SDK features.

| Feature                                    | Status      | Description                             |
|--------------------------------------------|-------------|-----------------------------------------|
| `New()`, `Run()`, `Stream()`, `Close()`    | Implemented | Core agent lifecycle                    |
| `PreToolUse` hooks                         | Implemented | Intercept and control tool execution    |
| `DenyCommands`, `AllowPaths`, `DenyPaths`  | Implemented | Built-in security hooks                 |
| `RedirectPath`, `RequireCommand`           | Implemented | Path rewriting and command substitution |
| `PostToolUse`, `OnStop` hooks              | Implemented | Observation and cleanup hooks           |
| `WithSchema`, `RunStructured`              | Implemented | Structured JSON output                  |
| `Audit`, `AuditToFile`                     | Implemented | Event logging and observability         |
| `MaxTurns`, `Resume`, `Fork`               | Implemented | Session limits and management           |
| `CustomTool`                               | Implemented | In-process tool execution               |
| `MCPServer`, `StrictMCPConfig`             | Implemented | MCP server configuration                |
| `Subagent`                                 | Implemented | Subagent configuration                  |
| `Skill`, `SkillsDir`                       | Implemented | Skills and context injection            |
| `SystemPromptPreset`, `SystemPromptAppend` | Implemented | System prompt customization             |

## Package Structure

The SDK consists of a single package:

```
github.com/wernerstrydom/claude-agent-sdk-go/agent
```

This design choice simplifies imports and reduces cognitive overhead. All types, functions, and options are accessed
through one import.

## Quick Reference

### Creating an Agent

```go
import "github.com/wernerstrydom/claude-agent-sdk-go/agent"

a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.WorkDir("/path/to/project"),
    agent.MaxTurns(10),
)
```

### Running Prompts

```go
// Blocking call
result, err := a.Run(ctx, "Explain channels in Go")

// Streaming
for msg := range a.Stream(ctx, "Write a function") {
    switch m := msg.(type) {
    case *agent.Text:
        fmt.Print(m.Text)
    case *agent.Result:
        fmt.Printf("Cost: $%.4f\n", m.CostUSD)
    }
}
```

### Security Hooks

```go
a, _ := agent.New(ctx,
    agent.PreToolUse(
        agent.DenyCommands("sudo", "rm -rf"),
        agent.AllowPaths("/sandbox", "/tmp"),
    ),
)
```

### Audit Logging

```go
a, _ := agent.New(ctx,
    agent.AuditToFile("audit.jsonl"),
)
```

## Next Steps

1. Follow the [Installation](getting-started/installation.md) guide to set up your environment
2. Work through the [Tutorial Series](tutorials/README.md) to build a complete application
3. Explore the source code in the `agent/` directory for implementation details
