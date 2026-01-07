# Agents

An agent represents a Claude Code session. It manages the underlying CLI process, handles message parsing, and tracks
state such as session ID, turns, and accumulated cost. This document explains the agent lifecycle, the differences
between execution modes, and how to handle errors and context cancellation.

## Creating an Agent

The `agent.New` function creates an agent with the specified configuration options. It starts the Claude Code CLI
process and prepares the communication bridge.

```go
import (
    "context"
    "log"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
    ctx := context.Background()

    a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }
    defer a.Close()
}
```

The first argument is a `context.Context` that governs the lifetime of the agent creation process. The remaining
arguments are functional options that configure the agent's behavior.

### Common Options

| Option                 | Description                                    | Default                 |
|------------------------|------------------------------------------------|-------------------------|
| `Model(name)`          | Claude model to use                            | `claude-sonnet-4-5`     |
| `WorkDir(path)`        | Working directory for file operations          | `.` (current directory) |
| `CLIPath(path)`        | Override CLI executable location               | Auto-detected           |
| `MaxTurns(n)`          | Maximum turns allowed across all Run calls     | Unlimited               |
| `PreToolUse(hooks...)` | Hooks to intercept tool calls before execution | None                    |

Example with multiple options:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.WorkDir("/path/to/project"),
    agent.MaxTurns(10),
    agent.PreToolUse(
        agent.DenyCommands("sudo"),
        agent.AllowPaths("/sandbox"),
    ),
)
```

For the complete list of options, see `/Users/wstrydom/Developer/wernerstrydom/claude-agent-sdk-go/agent/options.go`.

## Lazy Session Initialization

The Claude Code CLI uses a stream-JSON input format. With this protocol, the CLI waits for the first user message before
outputting anything, including the session initialization message.

This means:

1. `agent.New` starts the CLI process but does not block waiting for initialization
2. The session ID is captured lazily when the first message arrives
3. `SessionID()` returns an empty string until after the first `Run` or `Stream` call

```go
a, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
fmt.Println(a.SessionID()) // Empty string

result, _ := a.Run(ctx, "Hello")
fmt.Println(a.SessionID()) // Now contains the session ID
```

This design avoids blocking during agent creation and allows the SDK to start multiple agents concurrently without
waiting for each to initialize.

## Running Prompts

There are two ways to send prompts to an agent: `Run` for blocking execution and `Stream` for real-time message
handling.

### Run (Blocking)

The `Run` method sends a prompt and blocks until Claude completes its response. It returns a `*Result` containing the
final output, cost, and usage information.

```go
result, err := a.Run(ctx, "What is the capital of France?")
if err != nil {
    log.Fatalf("Run failed: %v", err)
}

fmt.Printf("Response: %s\n", result.ResultText)
fmt.Printf("Cost: $%.6f\n", result.CostUSD)
fmt.Printf("Turns: %d\n", result.NumTurns)
```

The `Result` type contains:

| Field           | Type            | Description                            |
|-----------------|-----------------|----------------------------------------|
| `ResultText`    | `string`        | The final text response from Claude    |
| `CostUSD`       | `float64`       | API cost for this run in USD           |
| `NumTurns`      | `int`           | Number of turns in this run            |
| `DurationTotal` | `time.Duration` | Total elapsed time                     |
| `DurationAPI`   | `time.Duration` | Time spent in API calls                |
| `IsError`       | `bool`          | Whether the result represents an error |

Use `Run` when you only need the final result and do not need to observe intermediate messages.

### Stream (Real-time)

The `Stream` method sends a prompt and returns a channel of messages. This enables real-time observation of Claude's
response as it arrives.

```go
for msg := range a.Stream(ctx, "Explain goroutines") {
    switch m := msg.(type) {
    case *agent.Text:
        fmt.Print(m.Text)
    case *agent.Thinking:
        fmt.Printf("[Thinking: %s]\n", m.Thinking)
    case *agent.ToolUse:
        fmt.Printf("[Tool: %s]\n", m.Name)
    case *agent.ToolResult:
        fmt.Printf("[Result for %s]\n", m.ToolUseID)
    case *agent.Result:
        fmt.Printf("\n\nCost: $%.6f\n", m.CostUSD)
    case *agent.Error:
        log.Printf("Error: %v\n", m.Err)
    }
}

if err := a.Err(); err != nil {
    log.Fatalf("Stream error: %v", err)
}
```

The channel closes when:

- A `Result` message is received (normal completion)
- An error occurs during parsing
- The context is cancelled

After the channel closes, call `a.Err()` to check for any errors that occurred during streaming.

### Message Types

The following message types can appear during streaming:

| Type                | Description                              |
|---------------------|------------------------------------------|
| `*agent.Text`       | Assistant text output                    |
| `*agent.Thinking`   | Extended thinking content (when enabled) |
| `*agent.ToolUse`    | Claude is invoking a tool                |
| `*agent.ToolResult` | Result from a tool execution             |
| `*agent.Result`     | Final result with cost and usage         |
| `*agent.Error`      | Error during execution                   |

Note that `*agent.SystemInit` is handled internally and not exposed to the caller.

### Per-Run Options

Both `Run` and `Stream` accept optional `RunOption` arguments for per-call configuration:

```go
// Set a timeout for this specific run
result, err := a.Run(ctx, "Process this data",
    agent.Timeout(30*time.Second),
)

// Override max turns for this run
result, err := a.Run(ctx, "Complete this task",
    agent.MaxTurnsRun(5),
)
```

## Closing an Agent

The `Close` method terminates the agent session and releases all resources. It should be called when the agent is no
longer needed.

```go
a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
if err != nil {
    log.Fatal(err)
}
defer a.Close()

// Use the agent...
```

Using `defer` ensures the agent is closed even if an error occurs. Close performs the following:

1. Calls any registered `OnStop` hooks
2. Emits a `session.end` audit event
3. Closes the message bridge
4. Terminates the CLI process
5. Runs audit cleanup functions

Close is idempotent. Calling it multiple times has no effect after the first call.

After Close is called, subsequent calls to `Run` or `Stream` return immediately with an empty channel.

## Context Cancellation

The SDK respects Go's context cancellation. Passing a cancelled context or cancelling a context during execution causes
the operation to stop.

### Agent Creation

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
if err != nil {
    // Handle timeout or cancellation
    log.Fatalf("Failed to create agent: %v", err)
}
```

### During Run

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := a.Run(ctx, "Perform a long-running task")
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        log.Println("Operation timed out")
    } else if ctx.Err() == context.Canceled {
        log.Println("Operation cancelled")
    }
}
```

### During Stream

When the context is cancelled during streaming, the channel closes and the agent's stop reason is set to
`StopInterrupted`:

```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    time.Sleep(5 * time.Second)
    cancel() // Cancel after 5 seconds
}()

for msg := range a.Stream(ctx, "Long task") {
    // Process messages...
}
// Channel closed due to cancellation
```

## Error Handling

The SDK defines several typed errors for different failure modes.

### StartError

Returned from `agent.New` when the agent fails to start. This typically indicates the CLI is not installed or not
accessible.

```go
a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
if err != nil {
    var startErr *agent.StartError
    if errors.As(err, &startErr) {
        log.Printf("Start failed: %s", startErr.Reason)
        if startErr.Cause != nil {
            log.Printf("Cause: %v", startErr.Cause)
        }
    }
}
```

### ProcessError

Returned when the CLI process exits with a non-zero exit code.

```go
result, err := a.Run(ctx, prompt)
if err != nil {
    var procErr *agent.ProcessError
    if errors.As(err, &procErr) {
        log.Printf("Process exited with code %d: %s",
            procErr.ExitCode, procErr.Stderr)
    }
}
```

### MaxTurnsError

Returned when the agent exceeds the configured maximum turns. This can occur before or after a run completes.

```go
a, _ := agent.New(ctx, agent.MaxTurns(5))

result, err := a.Run(ctx, "Complete multiple tasks")
if err != nil {
    var maxErr *agent.MaxTurnsError
    if errors.As(err, &maxErr) {
        log.Printf("Max turns exceeded: %d/%d",
            maxErr.Turns, maxErr.MaxAllowed)
        // result may still contain partial data
    }
}
```

### TaskError

Indicates a task-level error, such as no result being received.

```go
result, err := a.Run(ctx, prompt)
if err != nil {
    var taskErr *agent.TaskError
    if errors.As(err, &taskErr) {
        log.Printf("Task error in session %s: %s",
            taskErr.SessionID, taskErr.Message)
    }
}
```

### Error Handling Pattern

A comprehensive error handling pattern:

```go
result, err := a.Run(ctx, prompt)
if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        log.Println("Timeout")
    case errors.Is(err, context.Canceled):
        log.Println("Cancelled")
    default:
        var startErr *agent.StartError
        var procErr *agent.ProcessError
        var maxErr *agent.MaxTurnsError
        var taskErr *agent.TaskError

        switch {
        case errors.As(err, &startErr):
            log.Printf("Start error: %s", startErr.Reason)
        case errors.As(err, &procErr):
            log.Printf("Process error: code %d", procErr.ExitCode)
        case errors.As(err, &maxErr):
            log.Printf("Max turns: %d/%d", maxErr.Turns, maxErr.MaxAllowed)
        case errors.As(err, &taskErr):
            log.Printf("Task error: %s", taskErr.Message)
        default:
            log.Printf("Unknown error: %v", err)
        }
    }
    return
}
```

## Multi-Turn Conversations

An agent maintains conversation state across multiple `Run` or `Stream` calls. Each call continues the conversation from
where the previous one ended.

```go
a, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
defer a.Close()

// First turn
_, _ = a.Run(ctx, "Remember the number 42")

// Second turn - Claude remembers context from the first
result, _ := a.Run(ctx, "What number did I ask you to remember?")
fmt.Println(result.ResultText) // Contains reference to 42
```

The SDK tracks cumulative state:

```go
// Total turns across all Run calls
// (accessed internally, reflected in OnStop hooks)

// Total cost is also accumulated and available via OnStop hooks
```

For long-running sessions, consider using `MaxTurns` to set boundaries:

```go
a, _ := agent.New(ctx, agent.MaxTurns(20))
```

## Complete Example

The following example demonstrates the agent lifecycle with error handling and cleanup:

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()

    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.MaxTurns(10),
        agent.OnStop(func(e *agent.StopEvent) {
            log.Printf("Session %s ended: %s (%d turns, $%.4f)",
                e.SessionID, e.Reason, e.NumTurns, e.CostUSD)
        }),
    )
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }
    defer a.Close()

    // Stream first prompt
    for msg := range a.Stream(ctx, "Explain Go interfaces briefly") {
        if text, ok := msg.(*agent.Text); ok {
            fmt.Print(text.Text)
        }
    }
    fmt.Println()

    // Run follow-up
    result, err := a.Run(ctx, "Give an example")
    if err != nil {
        var maxErr *agent.MaxTurnsError
        if errors.As(err, &maxErr) {
            log.Printf("Hit max turns, partial result may be available")
        } else {
            log.Fatalf("Run failed: %v", err)
        }
    }

    if result != nil {
        fmt.Printf("\nExample:\n%s\n", result.ResultText)
        fmt.Printf("Cost: $%.6f\n", result.CostUSD)
    }
}
```

## Related Documentation

- [Hooks](hooks.md) - Intercepting and controlling tool execution
- [Sessions](sessions.md) - Session management, resume, and fork
- [Installation](../getting-started/installation.md) - Setup instructions
