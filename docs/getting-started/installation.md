# Installation

This guide covers the prerequisites, installation process, and verification steps for the Claude Agent SDK for Go.

## Prerequisites

Before installing the SDK, ensure your environment meets the following requirements.

### Go Version

The SDK requires Go 1.21 or later. Check your installed version:

```bash
go version
```

If you need to install or upgrade Go, refer to the official [Go installation documentation](https://go.dev/doc/install).

### Claude Code CLI

The SDK wraps the Claude Code CLI, which must be installed and authenticated before use.

Install the CLI using npm:

```bash
npm install -g @anthropic-ai/claude-code
```

Verify the installation:

```bash
claude --version
```

### Authentication

The CLI requires authentication with Anthropic. Run the following command and follow the prompts:

```bash
claude auth
```

This creates credentials that the SDK uses automatically. The SDK does not manage authentication directly; it relies on
the CLI's credential storage.

## Installation

Add the SDK to your Go project using `go get`:

```bash
go get github.com/wernerstrydom/claude-agent-sdk-go/agent
```

This downloads the SDK and adds it to your `go.mod` file.

## Verification

Create a simple program to verify the installation works correctly.

### Create Test File

Create a file named `verify.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }
    defer func() { _ = a.Close() }()

    result, err := a.Run(ctx, "Say hello in exactly three words.")
    if err != nil {
        log.Fatalf("Failed to run prompt: %v", err)
    }

    fmt.Printf("Response: %s\n", result.ResultText)
    fmt.Printf("Cost: $%.6f\n", result.CostUSD)
    fmt.Printf("Turns: %d\n", result.NumTurns)
}
```

### Run Verification

Execute the program:

```bash
go run verify.go
```

Expected output includes a three-word greeting, the API cost, and turn count. If you see this output, the SDK is
installed and working correctly.

### Troubleshooting

Common issues and their solutions:

**CLI not found**

If you see an error about the `claude` command not being found, ensure the CLI is installed and in your PATH:

```bash
which claude
```

If this returns nothing, reinstall the CLI or add its location to your PATH.

**Authentication error**

If authentication fails, re-run the authentication command:

```bash
claude auth
```

**Timeout error**

If the operation times out, the API may be slow or unreachable. Increase the timeout:

```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
```

**Module not found**

If Go cannot find the module, ensure you have initialized your project as a Go module:

```bash
go mod init myproject
go get github.com/wernerstrydom/claude-agent-sdk-go/agent
```

## Project Setup

For new projects, follow this structure:

```
myproject/
├── go.mod
├── go.sum
└── main.go
```

Initialize the module:

```bash
mkdir myproject
cd myproject
go mod init myproject
go get github.com/wernerstrydom/claude-agent-sdk-go/agent
```

Create your `main.go` and import the agent package:

```go
package main

import (
    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)
```

## Configuration Options

The SDK uses functional options for configuration. Common options include:

| Option          | Description                           | Default                 |
|-----------------|---------------------------------------|-------------------------|
| `Model(name)`   | Claude model to use                   | `claude-sonnet-4-5`     |
| `WorkDir(path)` | Working directory for file operations | `.` (current directory) |
| `CLIPath(path)` | Override CLI location                 | Auto-detected           |
| `MaxTurns(n)`   | Maximum turns allowed                 | Unlimited               |

Example with multiple options:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.WorkDir("/path/to/project"),
    agent.MaxTurns(10),
)
```

## Next Steps

With the SDK installed and verified, proceed to the [Tutorial Series](../tutorials/README.md) to learn how to build
applications with Claude agents.
