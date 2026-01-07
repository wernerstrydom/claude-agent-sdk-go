# Tutorial 3: Task Decomposition with Structured Output

This tutorial demonstrates how to use structured output for automated task decomposition and sequential execution with
the Claude Agent SDK for Go. You will learn to define schemas for project plans, execute tasks sequentially with
isolated agent contexts, and use the audit system for observability.

## Prerequisites

- Completed Tutorial 1 (Basic Agent) and Tutorial 2 (Streaming and Hooks)
- Go 1.21 or later
- Claude CLI installed with valid API credentials
- Familiarity with JSON Schema concepts

## Why Structured Output?

When building automation systems, free-form text responses create challenges. Parsing unstructured text to extract
actionable data is error-prone. Different response formats require different parsing strategies. There is no guarantee
that the response contains all required fields.

Structured output solves these problems by constraining Claude's responses to match a predefined JSON Schema. This
approach provides:

1. **Type Safety**: Responses map directly to Go structs
2. **Validation**: The schema enforces required fields and data types
3. **Reliability**: Consistent format enables deterministic processing
4. **Composability**: Output from one agent becomes input for another

## Implementation Status

> **Note**: The structured output features (`WithSchema`, `RunStructured`) are currently implemented in the SDK. The
`WithSchema` option configures the agent to request JSON output matching a schema derived from Go structs. The
`RunStructured` function provides a convenience wrapper for one-shot structured queries.

## Defining the Task Decomposition Schema

A project plan consists of tasks with dependencies. Each task has an identifier, description, list of affected files,
and references to prerequisite tasks.

```go
package main

// Task represents a single unit of work in the project plan.
type Task struct {
    // ID uniquely identifies this task within the plan.
    ID string `json:"id" desc:"Unique task identifier (e.g., task-001)"`

    // Description explains what this task accomplishes.
    Description string `json:"description" desc:"Clear explanation of the task objective"`

    // Files lists paths that this task will read or modify.
    Files []string `json:"files" desc:"File paths affected by this task"`

    // Dependencies lists task IDs that must complete before this task.
    Dependencies []string `json:"dependencies,omitempty" desc:"IDs of prerequisite tasks"`
}

// ProjectPlan represents a complete decomposition of work.
type ProjectPlan struct {
    // Title summarizes the overall objective.
    Title string `json:"title" desc:"Brief title for the project plan"`

    // Tasks contains the ordered list of work items.
    Tasks []Task `json:"tasks" desc:"Tasks to execute, ordered by dependency"`
}
```

The `desc` struct tag provides descriptions that become part of the JSON Schema. Claude uses these descriptions to
understand what each field should contain.

## Schema Generation

The SDK generates JSON Schema from Go structs using reflection. The above types produce this schema:

```json
{
  "type": "object",
  "properties": {
    "title": {
      "type": "string",
      "description": "Brief title for the project plan"
    },
    "tasks": {
      "type": "array",
      "description": "Tasks to execute, ordered by dependency",
      "items": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "description": "Unique task identifier (e.g., task-001)"
          },
          "description": {
            "type": "string",
            "description": "Clear explanation of the task objective"
          },
          "files": {
            "type": "array",
            "description": "File paths affected by this task",
            "items": {"type": "string"}
          },
          "dependencies": {
            "type": "array",
            "description": "IDs of prerequisite tasks",
            "items": {"type": "string"}
          }
        },
        "required": ["id", "description", "files"]
      }
    }
  },
  "required": ["title", "tasks"]
}
```

Fields with `omitempty` in the JSON tag are not marked as required. Pointer types are also treated as optional.

## Using Structured Output

### Method 1: Agent-Level Schema with WithSchema

Configure the agent once, then all responses follow the schema:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    // Create agent configured for structured output
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.WithSchema(ProjectPlan{}),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer a.Close()

    // Request a project plan - response will match ProjectPlan schema
    var plan ProjectPlan
    _, err = a.RunWithSchema(ctx, `
        Analyze the following requirements and create a project plan:

        Requirements:
        - Create a REST API for a patient records system
        - Include endpoints for creating, reading, and updating patient records
        - Add authentication middleware
        - Write unit tests for all handlers

        Break this into discrete tasks with clear dependencies.
    `, &plan)
    if err != nil {
        log.Fatal(err)
    }

    // plan is now populated with structured data
    fmt.Printf("Plan: %s\n", plan.Title)
    fmt.Printf("Tasks: %d\n", len(plan.Tasks))

    for _, task := range plan.Tasks {
        fmt.Printf("  [%s] %s\n", task.ID, task.Description)
        if len(task.Dependencies) > 0 {
            fmt.Printf("    Depends on: %v\n", task.Dependencies)
        }
    }
}
```

### Method 2: One-Shot Structured Query with RunStructured

For single queries where you want structured output without creating a persistent agent:

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
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    var plan ProjectPlan
    _, err := agent.RunStructured(ctx, `
        Create a simple project plan for adding a health check endpoint
        to an existing Go web service.
    `, &plan,
        agent.Model("claude-sonnet-4-5"),
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Plan: %s (%d tasks)\n", plan.Title, len(plan.Tasks))
}
```

`RunStructured` creates a temporary agent, executes the query, unmarshals the response, and closes the agent. This is
efficient for isolated structured queries.

## Fallback: Parsing JSON from Text Output

If working with an older SDK version or when structured output is unavailable, you can parse JSON from text responses.
This approach is less reliable but demonstrates the underlying mechanism:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "strings"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// parseJSONFromText extracts JSON from a text response.
// It looks for JSON between code fences or attempts to parse the entire response.
func parseJSONFromText(text string, target any) error {
    // Try to extract JSON from code fences
    if start := strings.Index(text, "```json"); start != -1 {
        start += 7 // Skip "```json"
        if end := strings.Index(text[start:], "```"); end != -1 {
            text = strings.TrimSpace(text[start : start+end])
        }
    } else if start := strings.Index(text, "```"); start != -1 {
        start += 3
        if end := strings.Index(text[start:], "```"); end != -1 {
            text = strings.TrimSpace(text[start : start+end])
        }
    }

    // Attempt to parse as JSON
    return json.Unmarshal([]byte(text), target)
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
    if err != nil {
        log.Fatal(err)
    }
    defer a.Close()

    result, err := a.Run(ctx, `
        Create a project plan for adding error logging to a Go service.

        Respond with ONLY a JSON object matching this structure:
        {
            "title": "string",
            "tasks": [
                {
                    "id": "string",
                    "description": "string",
                    "files": ["string"],
                    "dependencies": ["string"]
                }
            ]
        }
    `)
    if err != nil {
        log.Fatal(err)
    }

    var plan ProjectPlan
    if err := parseJSONFromText(result.ResultText, &plan); err != nil {
        log.Fatalf("Failed to parse JSON: %v\nResponse was: %s", err, result.ResultText)
    }

    fmt.Printf("Parsed plan: %s\n", plan.Title)
}
```

This fallback approach has limitations. Claude might include explanatory text around the JSON. The format might vary
between responses. There is no schema validation at the API level.

Use the native `WithSchema` or `RunStructured` approaches when available.

## Sequential Task Execution

With a project plan in hand, execute each task sequentially. Each task runs in a fresh agent context to maintain
isolation and prevent context pollution between tasks.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "strings"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// TaskResult captures the outcome of executing a single task.
type TaskResult struct {
    TaskID    string
    Success   bool
    Output    string
    CostUSD   float64
    Duration  time.Duration
    Error     error
}

// executeTask runs a single task in a fresh agent context.
func executeTask(ctx context.Context, task Task, completedTasks map[string]*TaskResult) *TaskResult {
    start := time.Now()
    result := &TaskResult{TaskID: task.ID}

    // Build context from completed dependencies
    var contextParts []string
    for _, depID := range task.Dependencies {
        if dep, ok := completedTasks[depID]; ok && dep.Success {
            contextParts = append(contextParts, fmt.Sprintf(
                "Completed task %s: %s", depID, dep.Output,
            ))
        }
    }

    // Create fresh agent for this task
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Tools("Bash", "Read", "Write", "Edit", "Glob", "Grep"),
        agent.PermissionPrompt(agent.PermissionAcceptEdits),
        agent.MaxTurns(15),
        agent.PreToolUse(
            agent.DenyCommands("rm -rf", "sudo"),
            agent.AllowPaths(".", "/tmp"),
        ),
    )
    if err != nil {
        result.Error = err
        result.Duration = time.Since(start)
        return result
    }
    defer a.Close()

    // Build the prompt with task details and context
    prompt := fmt.Sprintf(`Execute the following task:

Task ID: %s
Description: %s
Files to work with: %s

%s

Complete this task. Be thorough but concise in your response.
Summarize what you accomplished when done.`,
        task.ID,
        task.Description,
        strings.Join(task.Files, ", "),
        func() string {
            if len(contextParts) > 0 {
                return "Context from completed tasks:\n" + strings.Join(contextParts, "\n")
            }
            return ""
        }(),
    )

    // Execute the task
    runResult, err := a.Run(ctx, prompt)
    if err != nil {
        result.Error = err
        result.Duration = time.Since(start)
        return result
    }

    result.Success = true
    result.Output = runResult.ResultText
    result.CostUSD = runResult.CostUSD
    result.Duration = time.Since(start)

    return result
}

// topologicalSort orders tasks respecting dependencies.
// Returns an error if circular dependencies are detected.
func topologicalSort(tasks []Task) ([]Task, error) {
    // Build dependency graph
    taskMap := make(map[string]*Task)
    inDegree := make(map[string]int)

    for i := range tasks {
        task := &tasks[i]
        taskMap[task.ID] = task
        inDegree[task.ID] = len(task.Dependencies)
    }

    // Find tasks with no dependencies
    var queue []string
    for id, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, id)
        }
    }

    var sorted []Task
    for len(queue) > 0 {
        // Pop from queue
        id := queue[0]
        queue = queue[1:]

        task := taskMap[id]
        sorted = append(sorted, *task)

        // Reduce in-degree for dependent tasks
        for _, other := range tasks {
            for _, dep := range other.Dependencies {
                if dep == id {
                    inDegree[other.ID]--
                    if inDegree[other.ID] == 0 {
                        queue = append(queue, other.ID)
                    }
                }
            }
        }
    }

    if len(sorted) != len(tasks) {
        return nil, fmt.Errorf("circular dependency detected")
    }

    return sorted, nil
}
```

## Cost Aggregation Across Agents

When running multiple agents, track costs to understand total spend:

```go
package main

import (
    "fmt"
    "sync"
)

// CostTracker accumulates costs across multiple agent executions.
type CostTracker struct {
    mu        sync.Mutex
    taskCosts map[string]float64
    totalCost float64
}

// NewCostTracker creates a new cost tracker.
func NewCostTracker() *CostTracker {
    return &CostTracker{
        taskCosts: make(map[string]float64),
    }
}

// Add records cost for a task.
func (ct *CostTracker) Add(taskID string, cost float64) {
    ct.mu.Lock()
    defer ct.mu.Unlock()
    ct.taskCosts[taskID] = cost
    ct.totalCost += cost
}

// Total returns the accumulated cost.
func (ct *CostTracker) Total() float64 {
    ct.mu.Lock()
    defer ct.mu.Unlock()
    return ct.totalCost
}

// Summary returns a formatted cost summary.
func (ct *CostTracker) Summary() string {
    ct.mu.Lock()
    defer ct.mu.Unlock()

    summary := fmt.Sprintf("Total Cost: $%.4f\n", ct.totalCost)
    summary += "Breakdown:\n"
    for taskID, cost := range ct.taskCosts {
        summary += fmt.Sprintf("  %s: $%.4f\n", taskID, cost)
    }
    return summary
}
```

## The Audit System

The audit system provides observability into agent execution. Events are emitted at key points: session start/end,
messages, tool usage, and hook evaluations.

### Audit Event Types

| Event Type            | Description                 |
|-----------------------|-----------------------------|
| `session.start`       | Agent session begins        |
| `session.init`        | SystemInit message received |
| `session.end`         | Agent session closes        |
| `message.prompt`      | User prompt submitted       |
| `message.text`        | Text response from Claude   |
| `message.thinking`    | Thinking/reasoning content  |
| `message.tool_use`    | Tool invocation begins      |
| `message.tool_result` | Tool execution completes    |
| `message.result`      | Final result received       |
| `hook.pre_tool_use`   | PreToolUse hook evaluated   |
| `hook.post_tool_use`  | PostToolUse hook evaluated  |
| `hook.stop`           | Stop hook called            |
| `error`               | Error occurred              |

### Using the Audit System

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    // Create agent with audit handlers
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),

        // Write audit log to file in JSONL format
        agent.AuditToFile("task-audit.jsonl"),

        // Also log key events to console
        agent.Audit(func(e agent.AuditEvent) {
            switch e.Type {
            case "session.start":
                fmt.Println("[AUDIT] Session started")
            case "message.tool_use":
                data := e.Data.(map[string]any)
                fmt.Printf("[AUDIT] Tool: %s\n", data["name"])
            case "message.result":
                data := e.Data.(map[string]any)
                fmt.Printf("[AUDIT] Cost: $%.4f\n", data["cost_usd"])
            case "error":
                data := e.Data.(map[string]any)
                fmt.Printf("[AUDIT] Error: %s\n", data["error"])
            }
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer a.Close()

    // Execute a task
    _, err = a.Run(ctx, "List the Go files in the current directory")
    if err != nil {
        log.Fatal(err)
    }

    // Read and display audit log
    fmt.Println("\n--- Audit Log ---")
    data, err := os.ReadFile("task-audit.jsonl")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(data))

    // Clean up
    os.Remove("task-audit.jsonl")
}
```

### Analyzing Audit Data

The JSONL format allows easy processing with standard tools:

```go
package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
    "time"
)

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
    Time      time.Time      `json:"time"`
    SessionID string         `json:"session_id"`
    Type      string         `json:"type"`
    Data      map[string]any `json:"data"`
}

// AnalyzeAuditLog reads an audit log and computes statistics.
func AnalyzeAuditLog(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    var (
        totalCost   float64
        toolCounts  = make(map[string]int)
        errorCount  int
        sessionTime time.Duration
        startTime   time.Time
        endTime     time.Time
    )

    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        var entry AuditEntry
        if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
            continue
        }

        switch entry.Type {
        case "session.start":
            startTime = entry.Time
        case "session.end":
            endTime = entry.Time
            if data, ok := entry.Data["total_cost"].(float64); ok {
                totalCost = data
            }
        case "message.tool_use":
            if name, ok := entry.Data["name"].(string); ok {
                toolCounts[name]++
            }
        case "error":
            errorCount++
        }
    }

    if !startTime.IsZero() && !endTime.IsZero() {
        sessionTime = endTime.Sub(startTime)
    }

    fmt.Printf("Session Duration: %v\n", sessionTime)
    fmt.Printf("Total Cost: $%.4f\n", totalCost)
    fmt.Printf("Errors: %d\n", errorCount)
    fmt.Println("Tool Usage:")
    for tool, count := range toolCounts {
        fmt.Printf("  %s: %d\n", tool, count)
    }

    return scanner.Err()
}
```

## Complete Working Example

This example brings together all concepts: structured output for planning, sequential task execution, cost tracking, and
audit logging.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "strings"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// Task represents a single unit of work.
type Task struct {
    ID           string   `json:"id" desc:"Unique task identifier"`
    Description  string   `json:"description" desc:"Task objective"`
    Files        []string `json:"files" desc:"Affected files"`
    Dependencies []string `json:"dependencies,omitempty" desc:"Prerequisite task IDs"`
}

// ProjectPlan represents a complete work breakdown.
type ProjectPlan struct {
    Title string `json:"title" desc:"Plan title"`
    Tasks []Task `json:"tasks" desc:"Ordered tasks"`
}

// TaskResult captures execution outcome.
type TaskResult struct {
    TaskID   string
    Success  bool
    Output   string
    CostUSD  float64
    Duration time.Duration
    Error    error
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    // Phase 1: Generate project plan with structured output
    fmt.Println("=== Phase 1: Generating Project Plan ===")

    var plan ProjectPlan
    _, err := agent.RunStructured(ctx, `
        Create a project plan for adding a health check endpoint to a Go web service.

        The service uses the standard library net/http package.
        The health check should verify database connectivity and return JSON status.

        Include tasks for:
        1. Creating the handler
        2. Adding the route
        3. Writing tests

        Keep each task focused and small.
    `, &plan,
        agent.Model("claude-sonnet-4-5"),
    )
    if err != nil {
        log.Fatalf("Failed to generate plan: %v", err)
    }

    fmt.Printf("Plan: %s\n", plan.Title)
    fmt.Printf("Tasks: %d\n\n", len(plan.Tasks))

    for i, task := range plan.Tasks {
        fmt.Printf("%d. [%s] %s\n", i+1, task.ID, task.Description)
        if len(task.Dependencies) > 0 {
            fmt.Printf("   Depends on: %s\n", strings.Join(task.Dependencies, ", "))
        }
    }

    // Phase 2: Execute tasks sequentially
    fmt.Println("\n=== Phase 2: Executing Tasks ===")

    completedTasks := make(map[string]*TaskResult)
    var totalCost float64

    for _, task := range plan.Tasks {
        fmt.Printf("\nExecuting: [%s] %s\n", task.ID, task.Description)

        // Check dependencies are met
        for _, depID := range task.Dependencies {
            dep, exists := completedTasks[depID]
            if !exists || !dep.Success {
                fmt.Printf("  Skipping: dependency %s not satisfied\n", depID)
                completedTasks[task.ID] = &TaskResult{
                    TaskID: task.ID,
                    Error:  fmt.Errorf("dependency %s not satisfied", depID),
                }
                continue
            }
        }

        result := executeTask(ctx, task, completedTasks)
        completedTasks[task.ID] = result
        totalCost += result.CostUSD

        if result.Success {
            fmt.Printf("  Completed in %v (cost: $%.4f)\n", result.Duration, result.CostUSD)
        } else {
            fmt.Printf("  Failed: %v\n", result.Error)
        }
    }

    // Phase 3: Summary
    fmt.Println("\n=== Summary ===")

    var succeeded, failed int
    for _, result := range completedTasks {
        if result.Success {
            succeeded++
        } else {
            failed++
        }
    }

    fmt.Printf("Tasks: %d succeeded, %d failed\n", succeeded, failed)
    fmt.Printf("Total Cost: $%.4f\n", totalCost)
}

// executeTask runs a single task with a fresh agent context.
func executeTask(ctx context.Context, task Task, completed map[string]*TaskResult) *TaskResult {
    start := time.Now()
    result := &TaskResult{TaskID: task.ID}

    // Build context from dependencies
    var contextParts []string
    for _, depID := range task.Dependencies {
        if dep, ok := completed[depID]; ok && dep.Success {
            // Summarize what the dependency accomplished
            summary := dep.Output
            if len(summary) > 200 {
                summary = summary[:200] + "..."
            }
            contextParts = append(contextParts,
                fmt.Sprintf("Task %s completed: %s", depID, summary))
        }
    }

    // Create isolated agent for this task
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Tools("Bash", "Read", "Write", "Edit", "Glob", "Grep"),
        agent.PermissionPrompt(agent.PermissionAcceptEdits),
        agent.MaxTurns(10),

        // Security: restrict file access
        agent.PreToolUse(
            agent.DenyCommands("rm -rf", "sudo", "chmod"),
            agent.AllowPaths(".", "/tmp"),
        ),

        // Observability: log tool usage
        agent.PostToolUse(func(tc *agent.ToolCall, tr *agent.ToolResultContext) agent.HookResult {
            fmt.Printf("    Tool: %s (%v)\n", tc.Name, tr.Duration.Round(time.Millisecond))
            return agent.HookResult{Decision: agent.Continue}
        }),
    )
    if err != nil {
        result.Error = fmt.Errorf("failed to create agent: %w", err)
        result.Duration = time.Since(start)
        return result
    }
    defer a.Close()

    // Build comprehensive prompt
    prompt := fmt.Sprintf(`Execute this task:

Task: %s
Description: %s
Files: %s

%s

Complete the task and provide a brief summary of what you accomplished.`,
        task.ID,
        task.Description,
        strings.Join(task.Files, ", "),
        func() string {
            if len(contextParts) > 0 {
                return "Context from completed tasks:\n" + strings.Join(contextParts, "\n")
            }
            return ""
        }(),
    )

    runResult, err := a.Run(ctx, prompt)
    if err != nil {
        result.Error = fmt.Errorf("task execution failed: %w", err)
        result.Duration = time.Since(start)
        return result
    }

    result.Success = true
    result.Output = runResult.ResultText
    result.CostUSD = runResult.CostUSD
    result.Duration = time.Since(start)

    return result
}
```

## Key Concepts Summary

### Fresh Context Per Task

Creating a new agent for each task provides:

- **Isolation**: Tasks cannot interfere with each other
- **Clean State**: No accumulated context window pollution
- **Clear Boundaries**: Each task starts with exactly the information it needs
- **Cost Attribution**: Costs are tracked per task

### Dependency Ordering

Tasks declare dependencies explicitly. Before executing a task:

1. Verify all dependencies completed successfully
2. Extract relevant context from dependency outputs
3. Pass this context to the new task

This creates a directed acyclic graph (DAG) of work that can be executed in order.

### Cost Aggregation

When running multiple agents:

- Track cost per task for granular billing
- Aggregate total cost for overall spend
- Use audit logs for detailed cost analysis
- Consider cost when deciding task granularity

### Audit for Debugging

The audit system provides:

- Complete history of all agent interactions
- Tool usage patterns and timings
- Error tracking and diagnosis
- Cost breakdown by event

Use audit logs to debug failed tasks, optimize prompts, and understand agent behavior.

## Exercises

1. **Parallel Execution**: Modify the example to execute independent tasks (no dependencies between them) in parallel
   using goroutines.

2. **Retry Logic**: Add retry logic for failed tasks with exponential backoff.

3. **Progress Reporting**: Create a real-time progress display that shows task completion percentage and estimated time
   remaining.

4. **Audit Analysis**: Write a tool that reads audit logs and generates a summary report with cost breakdown by task.

5. **Schema Evolution**: Extend the `Task` struct to include estimated duration and priority, then update the planning
   prompt to populate these fields.

## Next Steps

- **Tutorial 4**: Custom Tools - Define Go functions that Claude can invoke
- **Tutorial 5**: MCP Servers - Integrate external tool providers
- **Tutorial 6**: Subagents - Delegate specialized work to child agents
