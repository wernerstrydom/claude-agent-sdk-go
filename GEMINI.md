# Agent Mandate: Code Review Only

**Role:** You are strictly assigned to **read and review the code**. Do not implement features, modify code, or create new files unless explicitly instructed to do so by the user.

**Review Standards:**
*   **Style Guides:** Adhere strictly to the [Google](https://google.github.io/styleguide/go/guide) and [Uber](https://github.com/uber-go/guide/blob/master/style.md) Go style guides.
*   **Core Values:** Evaluate code based on:
    *   **Clarity:** Is the code easy to understand?
    *   **Simplicity:** Is the solution the simplest possible?
    *   **Concision:** Is the code free of noise?
    *   **Maintainability:** Is it easy to change safely?
    *   **Consistency:** Does it follow project patterns?
*   **Architectural Principles:**
    *   **Locality of Reference & Cohesion:** Related code should be together.
    *   **Avoid Unnecessary Abstractions:** Flag indirect or complex patterns where simple ones suffice.

---

# Claude Agent SDK for Go

A Go SDK for programmatic automation of the Claude Code CLI (`claude`).

## Project Overview

This library wraps the Claude Code CLI to provide a native Go API for creating AI agents. It manages the CLI process, handles JSON communication over stdio, and provides a rich set of features for agent orchestration, security, and observability.

### Core Architecture

- **`agent` Package**: The entire SDK is contained within the `github.com/wernerstrydom/claude-agent-sdk-go/agent` package.
- **`Agent` Struct**: Represents a single Claude Code session. It owns the CLI process and manages the conversation state.
- **Process Management**: Spawns the `claude` CLI with `--input-format stream-json` and `--output-format stream-json`.
- **Communication**: Uses a message pump (`bridge.go`) to translate raw JSON lines from the CLI into typed Go messages (e.g., `Text`, `ToolUse`, `Result`).

### Key Features

- **Blocking & Streaming**: `Run()` for simple request/response, `Stream()` for real-time tokens and events.
- **Structured Output**: Automatically generates JSON schemas from Go structs and unmarshals responses (`RunStructured`, `RunWithSchema`).
- **Hook System**: Deterministic enforcement of security and policy. Hooks can allow, deny, or modify tool execution (`PreToolUse`, `PostToolUse`).
- **Functional Options**: Configuration via the `Option` pattern (e.g., `agent.Model(...)`, `agent.MaxTurns(...)`).
- **Lazy Initialization**: The agent initializes the session lazily; the CLI waits for the first user message before emitting the system initialization event.

## Directory Structure

```text
claude-agent-sdk-go/
├── agent/           # Core library code
│   ├── agent.go     # Main Agent type and Run/Stream methods
│   ├── bridge.go    # Message channel management
│   ├── errors.go    # Typed errors
│   ├── hooks.go     # Hook system logic
│   ├── message.go   # Message type definitions
│   ├── options.go   # Configuration options
│   ├── parser.go    # JSON output parser
│   ├── process.go   # CLI process lifecycle
│   └── schema.go    # Reflection-based JSON schema generation
├── examples/        # Usage examples
├── spec.md          # Detailed specification
├── plan.md          # Implementation roadmap
├── Makefile         # Build and test commands
└── go.mod           # Module definition
```

## Development

### Build and Test

The project uses a `Makefile` for common tasks:

- **Run all tests**: `make test` (or `go test -v ./...`)
- **Run Hello World example**: `make example-hello`
- **Run Streaming example**: `make example-stream`

### Protocol Details

The SDK interacts with the CLI using the `stream-json` protocol:
1.  **Input**: The SDK sends JSON objects (user messages) to the CLI's stdin.
2.  **Output**: The SDK reads JSON objects from the CLI's stdout.
3.  **Initialization**: The CLI does **not** output the `SystemInit` message until it receives the first user message. The SDK handles this sequence internally.

### Coding Conventions

- **Style**: Follows standard Go conventions (Google/Uber style guides).
- **Design**:
    - **Clarity > Simplicity > Concision**.
    - **Functional Options**: Used for all optional configuration to maintain API compatibility.
    - **Typed Errors**: Specific error types (`StartError`, `MaxTurnsError`) are used for robust error handling.

## Usage Examples

### Basic Run
```go
a, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
defer a.Close()
result, _ := a.Run(ctx, "Hello world")
```

### Structured Output
```go
type Response struct {
    Answer string `json:"answer" desc:"The answer"`
}
var resp Response
agent.RunStructured(ctx, "Question", &resp)
```

### Security Hooks
```go
agent.New(ctx,
    agent.PreToolUse(
        agent.DenyCommands("rm -rf", "sudo"),
        agent.AllowPaths("/safe/dir"),
    ),
)
```