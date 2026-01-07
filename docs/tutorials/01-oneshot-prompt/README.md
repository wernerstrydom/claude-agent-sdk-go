# Tutorial 01: Oneshot Prompt

This tutorial demonstrates the simplest way to use the Claude Agent SDK for Go: sending a single prompt and receiving a
complete response. You will build a command-line tool that asks Claude to generate a TODO web application.

## What is a Oneshot Prompt?

A oneshot prompt is a single interaction with Claude where you send one prompt and receive one complete response. The
term "oneshot" comes from the fact that you send the prompt once and Claude completes the entire task without further
input from your program.

Use oneshot prompts when:

- The task can be fully described in a single prompt
- No intermediate decisions or human approval is needed
- The task scope is well-defined

Oneshot prompts are unsuitable when:

- The task requires iterative refinement based on intermediate results
- You need to approve individual steps (such as file writes or command execution)
- The task scope is ambiguous or requires clarification

## Prerequisites

Before starting this tutorial, ensure you have:

1. **Go 1.21 or later** installed
2. **Claude CLI** installed and configured with a valid API key
3. The **Claude Agent SDK** available in your project

Verify your setup:

```bash
go version          # Should show Go 1.21+
claude --version    # Should show the Claude CLI version
```

## Project Setup

Create a new directory for this tutorial and initialize a Go module:

```bash
mkdir oneshot-example
cd oneshot-example
go mod init oneshot-example
```

Add the SDK dependency. Replace the path with the actual module path of the SDK:

```bash
go get github.com/wernerstrydom/claude-agent-sdk-go/agent
```

## The Code

Create a file named `main.go` with the following content:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Create a context that cancels on interrupt signals.
	// This allows graceful shutdown when the user presses Ctrl+C.
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	// Create a new agent with default settings.
	// The Model option specifies which Claude model to use.
	a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}
	defer a.Close()

	// Define the prompt for Claude.
	// A well-structured prompt improves the quality of the output.
	prompt := `Create a simple TODO web application with the following requirements:
1. HTML file with inline CSS and JavaScript
2. Add, complete, and delete tasks
3. Tasks persist in localStorage
4. Clean, minimal design

Create the file as todo.html in the current directory.`

	fmt.Println("Generating TODO application...")
	fmt.Println()

	// Stream responses from Claude.
	// Streaming provides real-time feedback as Claude generates its response.
	for msg := range a.Stream(ctx, prompt) {
		switch m := msg.(type) {
		case *agent.Text:
			// Text messages contain Claude's explanations and commentary.
			fmt.Print(m.Text)

		case *agent.ToolUse:
			// ToolUse messages indicate Claude is invoking a tool.
			// The Name field identifies the tool (e.g., "Write", "Bash").
			// The Input field contains the tool's parameters.
			fmt.Printf("\n[Using tool: %s]\n", m.Name)

		case *agent.ToolResult:
			// ToolResult messages contain the output from a tool execution.
			// IsError indicates whether the tool execution failed.
			if m.IsError {
				fmt.Printf("[Tool error: %v]\n", m.Content)
			}

		case *agent.Result:
			// Result is the final message, containing summary statistics.
			// CostUSD is the total API cost for this interaction.
			// NumTurns is the number of conversation turns.
			// DurationTotal is the wall-clock time for the entire operation.
			fmt.Println()
			fmt.Println("---")
			fmt.Printf("Completed in %v\n", m.DurationTotal)
			fmt.Printf("Turns: %d\n", m.NumTurns)
			fmt.Printf("Cost: $%.4f\n", m.CostUSD)

		case *agent.Error:
			// Error messages indicate a problem during execution.
			return fmt.Errorf("agent error: %w", m.Err)
		}
	}

	// Check for any streaming errors after the channel closes.
	if err := a.Err(); err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	return nil
}
```

## Key Concepts

### Creating an Agent

The `agent.New` function creates a new agent instance:

```go
a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
```

The first argument is a `context.Context` that controls the agent's lifecycle. Canceling this context will interrupt any
ongoing operations.

The remaining arguments are functional options that configure the agent's behavior. The SDK uses the functional options
pattern, where each option is a function that modifies the agent's configuration. This pattern provides several
benefits:

- Options are self-documenting through their function names
- Default values work without any options
- New options can be added without breaking existing code

Common options include:

| Option          | Purpose                                        |
|-----------------|------------------------------------------------|
| `Model(name)`   | Specifies the Claude model to use              |
| `WorkDir(path)` | Sets the working directory for file operations |
| `MaxTurns(n)`   | Limits the number of conversation turns        |

### Streaming vs Blocking

The SDK provides two methods for sending prompts:

**`Stream(ctx, prompt)`** returns a channel that yields messages as Claude generates them. Use streaming when you want
real-time feedback:

```go
for msg := range a.Stream(ctx, prompt) {
    // Handle each message as it arrives
}
```

**`Run(ctx, prompt)`** blocks until Claude completes the entire response. Use this when you only need the final result:

```go
result, err := a.Run(ctx, prompt)
if err != nil {
    return err
}
fmt.Printf("Cost: $%.4f\n", result.CostUSD)
```

This tutorial uses `Stream` to provide visibility into what Claude is doing. For simpler use cases where you only need
the final result, `Run` is more concise.

### Message Types

The SDK defines several message types that represent different kinds of output from Claude:

| Type                | Description                                                  |
|---------------------|--------------------------------------------------------------|
| `*agent.Text`       | Textual output from Claude                                   |
| `*agent.ToolUse`    | Claude is invoking a tool (e.g., writing a file)             |
| `*agent.ToolResult` | The result of a tool execution                               |
| `*agent.Result`     | Final summary with cost, duration, and turn count            |
| `*agent.Error`      | An error occurred during execution                           |
| `*agent.Thinking`   | Claude's reasoning process (if extended thinking is enabled) |

The type switch pattern allows you to handle each message type appropriately:

```go
switch m := msg.(type) {
case *agent.Text:
    // Handle text
case *agent.ToolUse:
    // Handle tool invocation
}
```

### Error Handling

Errors can occur at several points:

1. **Agent creation**: `agent.New` returns an error if the CLI cannot be started
2. **During streaming**: Messages of type `*agent.Error` indicate runtime errors
3. **After streaming**: `a.Err()` returns any error that occurred after the channel closed

Always check all three error sources:

```go
a, err := agent.New(ctx, ...)
if err != nil {
    return err
}
defer a.Close()

for msg := range a.Stream(ctx, prompt) {
    if e, ok := msg.(*agent.Error); ok {
        return e.Err
    }
    // ...
}

if err := a.Err(); err != nil {
    return err
}
```

### Context Usage

The `context.Context` parameter controls cancellation and timeouts. In the example, we create a context that cancels
when the user presses Ctrl+C:

```go
ctx, cancel := signal.NotifyContext(
    context.Background(),
    syscall.SIGINT,
    syscall.SIGTERM,
)
defer cancel()
```

You can also use `context.WithTimeout` to set a deadline:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
```

When the context is canceled, the agent stops processing and the stream channel closes.

### Resource Cleanup

Always close the agent when you are done:

```go
defer a.Close()
```

The `Close` method terminates the underlying CLI process and releases resources. Failing to close the agent may leave
orphaned processes.

## Running the Example

Build and run the example:

```bash
go run main.go
```

You should see output similar to:

```
Generating TODO application...

I'll create a simple TODO web application with the requirements you specified.

[Using tool: Write]

I've created a clean, minimal TODO application. Here's what I included:

**Features:**
- Add new tasks by typing and pressing Enter or clicking Add
- Mark tasks as complete by clicking the checkbox
- Delete tasks with the X button
- All tasks persist in localStorage
- Clean, minimal design with a modern look

The file has been saved as `todo.html`. You can open it directly in any web browser.

---
Completed in 12.345s
Turns: 1
Cost: $0.0123
```

After running, you should find a `todo.html` file in your current directory. Open it in a web browser to see the
generated application.

## Cost Tracking

The `Result` message includes the cost of the API call in US dollars:

```go
case *agent.Result:
    fmt.Printf("Cost: $%.4f\n", m.CostUSD)
```

This allows you to track spending when building applications that make many API calls. The cost includes all input and
output tokens, including any tool usage during the interaction.

## Limitations

While oneshot prompts are the simplest way to use the SDK, they have limitations:

1. **No intermediate control**: You cannot review or approve Claude's actions before they execute. If Claude decides to
   write a file or run a command, it happens without your input.

2. **No error recovery**: If something fails partway through, you must start over from the beginning.

3. **No iterative refinement**: You cannot adjust the prompt based on partial results.

4. **Context limitations**: Very complex tasks may exceed Claude's context window or require multiple interactions.

The next tutorial covers hooks, which allow you to intercept and control tool execution, addressing the first
limitation.

## Summary

This tutorial covered:

- Creating an agent with `agent.New` and functional options
- Using `Stream` for real-time feedback during generation
- Handling different message types with type switches
- Proper error handling and resource cleanup
- Cost tracking with the `Result` message

The complete code demonstrates the minimal pattern for using the Claude Agent SDK. Subsequent tutorials build on this
foundation to add control, safety, and more complex interaction patterns.
