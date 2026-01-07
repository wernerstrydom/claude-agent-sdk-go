# Tutorial 2: Plan and Implement

This tutorial demonstrates a two-phase approach to code generation: first plan, then implement. By separating these
concerns, you gain predictable control over what the agent does in each phase. The planning phase produces a structured
specification without side effects, while the implementation phase executes changes within defined security boundaries.

## Prerequisites

- Go 1.21 or later
- Claude Code CLI installed and authenticated
- Completed Tutorial 1 (one-shot prompts)

## Why Separate Planning from Implementation

When an agent has unrestricted tool access, it may interleave planning and execution unpredictably. This creates several
problems:

1. **Unpredictable file modifications** - The agent might create files before the plan is reviewed
2. **Difficult to review** - Changes happen during the planning conversation
3. **No checkpoint for human approval** - There is no natural pause between thinking and doing

A two-phase approach addresses these issues:

- **Phase 1 (Planning)**: The agent analyzes requirements and produces a detailed plan. No file operations occur.
- **Phase 2 (Implementation)**: A new agent receives the plan and executes it with restricted permissions.

This separation creates a natural review checkpoint between phases, where a human or automated system can evaluate the
plan before any changes occur.

## Phase 1: The Planning Agent

The planning agent must produce output without modifying the filesystem. The SDK achieves this through a `PreToolUse`
hook that denies all tool calls.

### Denying All Tools

A hook that returns `Deny` for every tool call prevents the agent from executing any operations:

```go
func denyAllTools() agent.PreToolUseHook {
    return func(tc *agent.ToolCall) agent.HookResult {
        return agent.HookResult{
            Decision: agent.Deny,
            Reason:   "planning phase: no tool execution allowed",
        }
    }
}
```

When Claude attempts to use a tool, this hook intercepts the call and returns a denial with an explanation. Claude
receives this feedback and adjusts its approach to provide the plan as text output instead.

### Creating the Planning Agent

```go
planningAgent, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.WorkDir(projectDir),
    agent.PreToolUse(denyAllTools()),
)
if err != nil {
    return "", fmt.Errorf("failed to create planning agent: %w", err)
}
defer planningAgent.Close()
```

The planning agent operates with the same model and working directory as the implementation agent will use. The key
difference is the `PreToolUse` hook that blocks all tool execution.

### Capturing the Plan

The planning prompt asks Claude to produce a structured implementation plan:

```go
const planningPrompt = `Create a detailed implementation plan for a simple TODO web application in Go.

The application should have:
- A REST API with endpoints for CRUD operations on todos
- An in-memory data store
- JSON request/response handling
- Basic error handling

Output the plan as a structured list of files to create, with descriptions of each file's purpose and contents. Do not create any files - only describe what should be created.`
```

The result text contains the plan:

```go
result, err := planningAgent.Run(ctx, planningPrompt)
if err != nil {
    return "", fmt.Errorf("planning failed: %w", err)
}

return result.ResultText, nil
```

## Phase 2: The Implementation Agent

The implementation agent receives the plan and executes it with security constraints. Two types of constraints apply:

1. **Command restrictions** - Block dangerous shell commands
2. **Path restrictions** - Limit file operations to the project directory

### Security Hooks

The SDK provides built-in hooks for common security patterns.

#### DenyCommands

`DenyCommands` blocks shell commands containing specific patterns:

```go
agent.DenyCommands("rm -rf", "sudo", "curl", "wget")
```

This hook only applies to the `Bash` tool. When Claude attempts to run a command containing any of these patterns, the
hook returns a `Deny` result with an explanation.

Pattern matching uses substring containment. The pattern `"rm -rf"` blocks:

- `rm -rf /`
- `rm -rf ./temp`
- `echo test && rm -rf .`

#### AllowPaths

`AllowPaths` restricts file operations to specified directory prefixes:

```go
agent.AllowPaths("./todo-app")
```

This hook applies to file-operating tools: `Read`, `Write`, `Edit`, and `MultiEdit`. Only paths starting with
`./todo-app` are allowed. Attempts to access other paths result in denial.

### Hook Chain Evaluation

When multiple hooks are registered, they form a chain. The chain evaluates hooks in order with specific semantics:

1. **First `Deny` wins** - If any hook returns `Deny`, evaluation stops immediately
2. **`Allow` short-circuits** - If a hook returns `Allow`, remaining hooks are skipped
3. **`Continue` passes** - If a hook returns `Continue`, the next hook evaluates

If all hooks return `Continue`, the default result is `Allow`.

For the implementation agent:

```go
agent.PreToolUse(
    agent.DenyCommands("rm -rf", "sudo", "curl", "wget"),
    agent.AllowPaths("./todo-app"),
)
```

The evaluation order is:

1. `DenyCommands` checks if the tool is `Bash` with a blocked pattern
    - If matched: return `Deny` (evaluation stops)
    - Otherwise: return `Continue`

2. `AllowPaths` checks if the tool operates on files
    - If the path is outside allowed directories: return `Deny`
    - If the path is within allowed directories: return `Continue`
    - If the tool does not operate on files: return `Continue`

3. If both return `Continue`, the tool executes

### Creating the Implementation Agent

```go
implAgent, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.WorkDir(projectDir),
    agent.PreToolUse(
        agent.DenyCommands("rm -rf", "sudo", "curl", "wget"),
        agent.AllowPaths("./todo-app"),
    ),
)
if err != nil {
    return fmt.Errorf("failed to create implementation agent: %w", err)
}
defer implAgent.Close()
```

### Implementing with the Plan as Context

The implementation prompt includes the plan as context:

```go
implPrompt := fmt.Sprintf(`Implement the following plan for a TODO web application.

Create all files in the ./todo-app directory.

=== PLAN ===
%s
=== END PLAN ===

Implement each file described in the plan. Use proper Go idioms and error handling.`, plan)
```

### Streaming Progress

Using `Stream` instead of `Run` allows progress monitoring:

```go
for msg := range implAgent.Stream(ctx, implPrompt) {
    switch m := msg.(type) {
    case *agent.Text:
        fmt.Print(m.Text)
    case *agent.ToolUse:
        fmt.Printf("\n[Tool: %s]\n", m.Name)
    case *agent.ToolResult:
        if m.IsError {
            fmt.Printf("[Error in tool execution]\n")
        }
    case *agent.Result:
        fmt.Printf("\n\nImplementation complete.\n")
        fmt.Printf("Turns: %d, Cost: $%.4f\n", m.NumTurns, m.CostUSD)
    case *agent.Error:
        return fmt.Errorf("implementation error: %w", m.Err)
    }
}
```

## Complete Example

The following program implements the full plan-and-implement workflow:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
    ctx := context.Background()

    // Create project directory
    projectDir, err := os.MkdirTemp("", "todo-project-*")
    if err != nil {
        log.Fatalf("failed to create project directory: %v", err)
    }
    defer os.RemoveAll(projectDir)

    fmt.Printf("Project directory: %s\n\n", projectDir)

    // Phase 1: Planning
    fmt.Println("=== Phase 1: Planning ===")
    plan, err := runPlanningPhase(ctx, projectDir)
    if err != nil {
        log.Fatalf("planning phase failed: %v", err)
    }

    fmt.Println("\n--- Plan Output ---")
    fmt.Println(plan)
    fmt.Println("--- End Plan ---\n")

    // Phase 2: Implementation
    fmt.Println("=== Phase 2: Implementation ===")
    if err := runImplementationPhase(ctx, projectDir, plan); err != nil {
        log.Fatalf("implementation phase failed: %v", err)
    }

    // List created files
    fmt.Println("\n=== Created Files ===")
    listFiles(filepath.Join(projectDir, "todo-app"))
}

// denyAllTools returns a hook that blocks all tool execution.
func denyAllTools() agent.PreToolUseHook {
    return func(tc *agent.ToolCall) agent.HookResult {
        return agent.HookResult{
            Decision: agent.Deny,
            Reason:   "planning phase: no tool execution allowed",
        }
    }
}

func runPlanningPhase(ctx context.Context, projectDir string) (string, error) {
    planningAgent, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.WorkDir(projectDir),
        agent.PreToolUse(denyAllTools()),
    )
    if err != nil {
        return "", fmt.Errorf("failed to create planning agent: %w", err)
    }
    defer planningAgent.Close()

    const planningPrompt = `Create a detailed implementation plan for a simple TODO web application in Go.

The application should have:
- A REST API with endpoints for CRUD operations on todos
- An in-memory data store
- JSON request/response handling
- Basic error handling

Output the plan as a structured list of files to create, with descriptions of each file's purpose and contents. Do not create any files - only describe what should be created.`

    fmt.Println("Generating implementation plan...")
    result, err := planningAgent.Run(ctx, planningPrompt)
    if err != nil {
        return "", fmt.Errorf("planning failed: %w", err)
    }

    return result.ResultText, nil
}

func runImplementationPhase(ctx context.Context, projectDir, plan string) error {
    implAgent, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.WorkDir(projectDir),
        agent.PreToolUse(
            agent.DenyCommands("rm -rf", "sudo", "curl", "wget"),
            agent.AllowPaths("./todo-app"),
        ),
    )
    if err != nil {
        return fmt.Errorf("failed to create implementation agent: %w", err)
    }
    defer implAgent.Close()

    implPrompt := fmt.Sprintf(`Implement the following plan for a TODO web application.

Create all files in the ./todo-app directory.

=== PLAN ===
%s
=== END PLAN ===

Implement each file described in the plan. Use proper Go idioms and error handling.`, plan)

    fmt.Println("Implementing plan...")
    for msg := range implAgent.Stream(ctx, implPrompt) {
        switch m := msg.(type) {
        case *agent.Text:
            fmt.Print(m.Text)
        case *agent.ToolUse:
            fmt.Printf("\n[Tool: %s]\n", m.Name)
        case *agent.ToolResult:
            if m.IsError {
                fmt.Printf("[Error in tool execution]\n")
            }
        case *agent.Result:
            fmt.Printf("\n\nImplementation complete.\n")
            fmt.Printf("Turns: %d, Cost: $%.4f\n", m.NumTurns, m.CostUSD)
        case *agent.Error:
            return fmt.Errorf("implementation error: %w", m.Err)
        }
    }

    if err := implAgent.Err(); err != nil {
        return fmt.Errorf("stream error: %w", err)
    }

    return nil
}

func listFiles(dir string) {
    err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() {
            relPath, _ := filepath.Rel(dir, path)
            fmt.Printf("  %s (%d bytes)\n", relPath, info.Size())
        }
        return nil
    })
    if err != nil {
        fmt.Printf("Error listing files: %v\n", err)
    }
}
```

## Running the Example

1. Create the example directory and file:

```bash
mkdir -p examples/plan-implement
```

2. Save the code above as `examples/plan-implement/main.go`

3. Run the example:

```bash
go run ./examples/plan-implement
```

Expected output structure:

```
Project directory: /tmp/todo-project-123456

=== Phase 1: Planning ===
Generating implementation plan...

--- Plan Output ---
[Structured plan describing files to create]
--- End Plan ---

=== Phase 2: Implementation ===
Implementing plan...
[Progress output as files are created]

Implementation complete.
Turns: 8, Cost: $0.0234

=== Created Files ===
  main.go (1234 bytes)
  todo.go (567 bytes)
  handlers.go (890 bytes)
  ...
```

## Security Benefits

The hook system provides deterministic enforcement of security policies. Unlike prompting-based approaches that rely on
the model following instructions, hooks operate at the tool execution layer.

### What the Hooks Protect Against

| Threat                      | Hook                           | Protection                                |
|-----------------------------|--------------------------------|-------------------------------------------|
| Arbitrary command execution | `DenyCommands`                 | Blocks patterns like `sudo`, `curl`       |
| File system escape          | `AllowPaths`                   | Restricts operations to project directory |
| Accidental deletion         | `DenyCommands("rm -rf")`       | Prevents recursive deletion               |
| Network exfiltration        | `DenyCommands("curl", "wget")` | Blocks HTTP clients                       |

### Deterministic vs Probabilistic Security

Prompt-based security relies on the model's interpretation:

```
"Do not delete any files outside the project directory"
```

The model might:

- Misinterpret the boundary
- Forget the instruction over long conversations
- Be manipulated by adversarial prompts

Hook-based security operates deterministically:

```go
agent.AllowPaths("./todo-app")
```

The hook evaluates each tool call against exact criteria. There is no interpretation or forgetting. The behavior is the
same regardless of conversation length or prompt content.

## Key Concepts Summary

### Decision Types

| Decision   | Effect                                            |
|------------|---------------------------------------------------|
| `Continue` | Pass to next hook; default allows if all continue |
| `Allow`    | Approve immediately; skip remaining hooks         |
| `Deny`     | Block immediately; return reason to Claude        |

### PreToolUseHook Signature

```go
type PreToolUseHook func(*ToolCall) HookResult
```

The hook receives a `ToolCall` containing the tool name and input parameters. It returns a `HookResult` with a decision
and optional reason or input modifications.

### Built-in Hooks

| Hook                              | Purpose                                    |
|-----------------------------------|--------------------------------------------|
| `DenyCommands(patterns...)`       | Block Bash commands matching patterns      |
| `AllowPaths(prefixes...)`         | Allow only file operations within prefixes |
| `DenyPaths(prefixes...)`          | Block file operations within prefixes      |
| `RedirectPath(from, to)`          | Rewrite file paths                         |
| `RequireCommand(use, instead...)` | Suggest alternative commands               |

### Composing Hooks

Multiple hooks combine through the `PreToolUse` option:

```go
agent.PreToolUse(
    hook1,
    hook2,
    hook3,
)
```

Evaluation proceeds left-to-right until a `Deny` or `Allow` occurs, or all hooks return `Continue`.

## Next Steps

The next tutorial explores structured output, where Claude returns responses matching a JSON schema. This enables
programmatic processing of agent results.
