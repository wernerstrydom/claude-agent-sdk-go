# Claude Agent SDK for Go — Specification

## Overview

A Go library for programmatic automation of Claude Code. Designed for orchestrating AI agents in infrastructure, security, and release engineering workflows.

### Design Principles

1. **Clarity → Simplicity → Concision → Maintainability → Consistency**
2. **Agents as insider threats** — deterministic enforcement via hooks, not prompt-based compliance
3. **Functional options pattern** — composable, no maps, minimal failure modes
4. **Full fidelity** — rich message types for audit, observability, and progressive disclosure
5. **Single package** — one import, one thing to version

---

## Package Structure

```go
import "github.com/wernerstrydom/claude-agent-sdk-go/agent"
```

---

## Core Model

An `Agent` represents:
- One Claude Code process
- One session / context window
- Maintains conversation state across multiple `Run()` calls

```go
// Create agent
a, err := agent.New(ctx, opts...)

// Conversation builds in same context
a.Run(ctx, "analyze requirements.md")           // turn 1
a.Run(ctx, "create spec.md from that analysis") // turn 2, remembers turn 1
a.Run(ctx, "create plan.json from the spec")    // turn 3, has full context

// Cleanup
a.Close()
```

---

## Agent Lifecycle

### Creation

```go
func New(ctx context.Context, opts ...Option) (*Agent, error)
```

Context sets default timeout for all `Run()` calls when not overridden.

### Methods

```go
// Blocks, returns final result. Audit handler sees all messages.
func (a *Agent) Run(ctx context.Context, prompt string, opts ...RunOption) (*Result, error)

// Returns channel of messages for full control
func (a *Agent) Stream(ctx context.Context, prompt string, opts ...RunOption) <-chan Message

// Error from last Stream() after channel closes
func (a *Agent) Err() error

// Session ID for fork/resume
func (a *Agent) SessionID() string

// Terminate Claude Code process
func (a *Agent) Close() error
```

### Run vs Stream

```go
// Simple — for "fire and forget" tasks
result, err := a.Run(ctx, "build a hello world app")

// Full control — for interactive or observable tasks
for msg := range a.Stream(ctx, "interactive task") {
    switch m := msg.(type) {
    case *agent.Text:
        fmt.Print(m.Text)
    case *agent.ToolUse:
        log.Printf("tool: %s", m.Name)
    case *agent.Result:
        // done
    case *agent.Error:
        // handle
    }
}
if err := a.Err(); err != nil {
    // handle
}
```

`Run()` wraps `Stream()` internally. Audit handler fires for both.

---

## Options

All configuration via functional options pattern.

### Model

```go
agent.Model("claude-sonnet-4-5")    // "claude-opus-4-5", "claude-haiku-4-5"
```

### Working Directory

```go
agent.WorkDir("/path/to/project")
```

### Environment Variables

```go
agent.Env("TMPDIR", "/sandbox/tmp"),
agent.Env("HOME", "/sandbox/home"),
agent.Env("GOPATH", "/sandbox/go"),
```

### Tools

Built-in Claude Code tools:

```go
agent.Tools("Bash", "Write", "Edit", "Read", "Glob", "Grep", "Task")
```

`Task` enables subagent invocation.

### Permission Mode

```go
agent.PermissionMode(agent.Default)           // standard checks
agent.PermissionMode(agent.AcceptEdits)       // auto-accept file edits
agent.PermissionMode(agent.BypassPermissions) // bypass all (use with caution)
```

### Timeouts and Limits

```go
agent.MaxTurns(20)                    // safety limit on agentic loops
agent.Timeout(5 * time.Minute)        // per-Run override (context is default)
```

### Session Management

```go
agent.Resume(sessionID)    // continue previous session
agent.Fork(sessionID)      // branch from session, original unchanged
```

### Settings Sources

```go
agent.SettingSources("user")     // ~/.claude/settings.json
agent.SettingSources("project")  // .claude/settings.json, .claude/CLAUDE.md
agent.SettingSources("local")    // .claude/settings.local.json
```

---

## Hooks

Hooks provide deterministic enforcement. Evaluated in order; first `Deny` wins, `Continue` passes to next hook, `Allow` approves immediately.

### Hook Signatures

```go
// Tool hooks
type PreToolUseHook func(*ToolCall) HookResult
type PostToolUseHook func(*ToolCall, *ToolResult) HookResult

// Prompt hook
type UserPromptSubmitHook func(*PromptSubmit) PromptHookResult

// Lifecycle hooks
type StopHook func(*StopEvent)
type SubagentStopHook func(*SubagentStopEvent)
type PreCompactHook func(*PreCompactEvent) PreCompactResult
```

### Hook Types

```go
type ToolCall struct {
    Name  string
    Input map[string]any
}

type HookResult struct {
    Decision     Decision          // Allow, Deny, Continue
    Reason       string            // feedback shown to Claude
    UpdatedInput map[string]any    // optional: modify inputs
}

type Decision int

const (
    Continue Decision = iota  // pass to next hook
    Allow                     // approve, skip remaining hooks
    Deny                      // block
)

type PromptSubmit struct {
    Prompt    string
    SessionID string
    Turn      int
}

type PromptHookResult struct {
    UpdatedPrompt string    // optional: modify prompt
    Metadata      any       // optional: attach metadata for audit
}

type StopEvent struct {
    SessionID string
    Reason    string    // "completed", "max_turns", "interrupted", "error"
    NumTurns  int
    CostUSD   float64
}

type SubagentStopEvent struct {
    SessionID       string
    SubagentID      string
    SubagentType    string
    ParentToolUseID string
    NumTurns        int
    CostUSD         float64
}

type PreCompactEvent struct {
    SessionID      string
    Trigger        string    // "auto" or "manual"
    TranscriptPath string
    TokenCount     int
}

type PreCompactResult struct {
    Archive    bool      // save full transcript before compaction
    ArchiveTo  string    // optional: custom archive path
    Extract    any       // optional: structured data to preserve
}
```

### Hook Events

| Event | Description |
|-------|-------------|
| `PreToolUse` | Before tool execution — can allow, deny, or modify inputs |
| `PostToolUse` | After tool execution — observe results, add context |
| `UserPromptSubmit` | Before user prompt is sent — intercept or modify prompts |
| `Stop` | Agent execution stopping — cleanup, save state |
| `SubagentStop` | Subagent completed — aggregate results, track parallel work |
| `PreCompact` | Before context compaction — extract decisions, archive transcript |

### Registering Hooks

```go
agent.PreToolUse(hook1, hook2, hook3)   // evaluated in order
agent.PostToolUse(hook1, hook2)
agent.UserPromptSubmit(promptHook)
agent.Stop(stopHook)
agent.SubagentStop(subagentHook)
agent.PreCompact(compactHook)
```

### Higher-Order Hook Functions

#### Command Control

```go
// Block commands matching patterns
agent.DenyCommands("rm -rf /", "cat", "head", "tail", ":(){ :|:& };:")

// Require alternative command (deny originals, provide guidance)
agent.RequireCommand("make", "go build", "go test")
// Denies "go build" and "go test", tells Claude to use "make" instead
```

#### Path Control

```go
// Allow only these paths (deny all others)
agent.AllowPaths("/sandbox", "/usr/bin")

// Deny these paths (allow all others)
agent.DenyPaths("/etc", "/usr", "/var", "~/.ssh")

// Rewrite paths transparently
agent.RedirectPath("/tmp", "/sandbox/tmp")
agent.RedirectPath("/home/user", "/sandbox/home")
```

#### Custom Hooks

```go
agent.PreToolUse(
    agent.DenyCommands("sudo"),
    agent.AllowPaths("/sandbox"),
    
    // Custom logic inline
    func(t *agent.ToolCall) agent.HookResult {
        if t.Name == "Bash" && strings.Contains(t.Input["command"].(string), "curl") {
            if !isAllowedURL(t.Input["command"].(string)) {
                return agent.HookResult{
                    Decision: agent.Deny,
                    Reason:   "external network access not permitted",
                }
            }
        }
        return agent.HookResult{Decision: agent.Continue}
    },
)
```

#### PreCompact Hook

Extract decisions and important context before the context window is compacted:

```go
agent.PreCompact(func(e *agent.PreCompactEvent) agent.PreCompactResult {
    // Parse transcript for decisions made
    decisions := extractDecisions(e.TranscriptPath)
    
    return agent.PreCompactResult{
        Archive:   true,                                    // save full transcript
        ArchiveTo: fmt.Sprintf("/audit/%s.jsonl", e.SessionID),
        Extract:   decisions,                               // preserve in compacted context
    }
})
```

#### Stop Hook

Cleanup and state persistence when agent stops:

```go
agent.Stop(func(e *agent.StopEvent) {
    log.Printf("session %s stopped: %s (%d turns, $%.4f)", 
        e.SessionID, e.Reason, e.NumTurns, e.CostUSD)
    
    if e.Reason == "error" {
        alerting.Send("agent failed", e)
    }
    
    metrics.RecordSession(e)
})
```

#### SubagentStop Hook

Track parallel subagent work:

```go
agent.SubagentStop(func(e *agent.SubagentStopEvent) {
    log.Printf("subagent %s (%s) completed: %d turns, $%.4f",
        e.SubagentID, e.SubagentType, e.NumTurns, e.CostUSD)
    
    // Aggregate costs across subagents
    costTracker.Add(e.SessionID, e.CostUSD)
})
```

---

## Structured Outputs

Force Claude to return structured data matching a schema.

### From Go Struct (Recommended)

```go
type Plan struct {
    Title string `json:"title" desc:"Short title for the plan"`
    Steps []Step `json:"steps" desc:"Ordered implementation steps"`
}

type Step struct {
    ID        string   `json:"id" desc:"Unique step identifier"`
    Action    string   `json:"action" desc:"What to do"`
    DependsOn []string `json:"depends_on,omitempty" desc:"IDs of prerequisites"`
}

var plan Plan
err := a.Run(ctx, "create implementation plan",
    agent.Output(&plan),  // schema derived, response unmarshaled
)
// plan is populated and validated
```

### Schema Only

```go
result, err := a.Run(ctx, "create plan",
    agent.OutputSchema(Plan{}),  // validates, returns raw JSON in result
)
```

---

## Skills and System Prompts

### Skills from Filesystem

```go
agent.SkillsDir("/path/to/skills")
agent.SkillsDir("/another/skill/library")
```

### Inline Skills

```go
agent.Skill("go-conventions", `
# Go Conventions

## Error Handling
- Always wrap errors with context
- Use errors.Is/As for comparison
`)

agent.Skill("project-rules", projectRulesMarkdown)
```

### System Prompt

```go
// Use Claude Code's preset
agent.SystemPromptPreset("claude_code")

// Append custom instructions
agent.SystemPromptAppend(`
You are working on the Rooikat infrastructure project.
Always use make instead of direct go commands.
`)
```

---

## Subagents

Subagents are configured at agent creation. Claude decides when to invoke them to complete its task.

```go
a, _ := agent.New(ctx,
    agent.Tools("Bash", "Write", "Edit", "Read", "Task"),
    
    agent.Subagent("test-runner",
        agent.SubagentDescription("Runs and analyzes test suites"),
        agent.SubagentPrompt("You run tests and report failures clearly..."),
        agent.SubagentTools("Bash", "Read"),
        agent.SubagentModel("haiku"),  // cheaper for simple task
    ),
    
    agent.Subagent("security-reviewer",
        agent.SubagentDescription("Deep security analysis"),
        agent.SubagentPrompt("You review code for security vulnerabilities..."),
        agent.SubagentTools("Read", "Grep", "Glob"),
        agent.SubagentModel("opus"),  // more capable
    ),
)
```

---

## Custom Tools (In-Process)

Define Go functions that Claude can invoke.

### With Explicit Parameters

```go
agent.Tool("query_inventory",
    agent.ToolDescription("Check inventory levels by SKU"),
    agent.ToolParam("sku", agent.String, "Product SKU"),
    agent.ToolParam("warehouse", agent.String, "Warehouse code", agent.Optional),
    agent.ToolHandler(func(ctx context.Context, p agent.Params) (any, error) {
        return inventoryDB.Query(p.String("sku"), p.String("warehouse"))
    }),
)
```

### With Struct Handler

```go
type InventoryQuery struct {
    SKU       string `json:"sku" desc:"Product SKU"`
    Warehouse string `json:"warehouse,omitempty" desc:"Warehouse code"`
}

agent.Tool("query_inventory",
    agent.ToolDescription("Check inventory levels by SKU"),
    agent.ToolHandle(func(ctx context.Context, q InventoryQuery) (*InventoryResult, error) {
        return inventoryDB.Query(q.SKU, q.Warehouse)
    }),
)
```

---

## MCP Servers (External)

For vendor-provided MCP servers when they add value.

```go
agent.MCPServer("github",
    agent.MCPCommand("npx"),
    agent.MCPArgs("@modelcontextprotocol/server-github"),
    agent.MCPEnv("GITHUB_TOKEN", os.Getenv("GITHUB_TOKEN")),
)

agent.MCPServer("database",
    agent.MCPSSE("https://db.example.com/mcp"),
    agent.MCPHeader("Authorization", "Bearer "+token),
)
```

---

## Audit

Transparent observability via options. Handler pattern like `slog.Handler`.

### Built-in Handlers

```go
agent.AuditToFile("/var/log/agent/audit.jsonl")
agent.AuditToWriter(os.Stderr)
```

### Custom Handler

```go
agent.AuditHandler(func(e agent.AuditEvent) {
    switch e.Type {
    case "hook.pre_tool_use":
        d := e.Data.(*agent.HookEvaluation)
        metrics.RecordHookLatency(d.Hook, d.Duration)
        if d.Decision == agent.Deny {
            alerting.Send("tool denied", d)
        }
    case "hook.pre_compact":
        d := e.Data.(*agent.PreCompactEvent)
        log.Printf("compacting session %s at %d tokens", d.SessionID, d.TokenCount)
    case "message.result":
        billing.RecordUsage(e.Data.(*agent.Result))
    }
    splunk.Write(e)
})
```

### Audit Event Types

- `session.start`, `session.end`
- `message.text`, `message.thinking`, `message.tool_use`, `message.tool_result`
- `message.result`
- `hook.pre_tool_use`, `hook.post_tool_use`
- `hook.user_prompt_submit`
- `hook.stop`
- `hook.subagent_stop`
- `hook.pre_compact`
- `subagent.start`, `subagent.stop`
- `compact` — context window compaction occurred
- `error`

### AuditEvent Structure

```go
type AuditEvent struct {
    Time      time.Time
    SessionID string
    Type      string
    Data      any  // typed payload
}
```

---

## Message Types

Full fidelity with metadata for correlation and display.

### Common Metadata

```go
type MessageMeta struct {
    Timestamp  time.Time
    SessionID  string
    Turn       int
    Sequence   int     // order within turn
    ParentID   string  // links tool results to tool use
    SubagentID string  // if from subagent
}
```

### Message Types

```go
type Message interface {
    message()
}

type SystemInit struct {
    MessageMeta
    TranscriptPath string
    Tools          []ToolDef
    MCPServers     []MCPServerStatus
}

type Text struct {
    MessageMeta
    Text string
}

type Thinking struct {
    MessageMeta
    Thinking  string
    Signature string
}

type ToolUse struct {
    MessageMeta
    ID    string
    Name  string
    Input map[string]any
}

type ToolResult struct {
    MessageMeta
    ToolUseID string
    Content   any
    IsError   bool
    Duration  time.Duration
}

type Result struct {
    MessageMeta
    DurationTotal time.Duration
    DurationAPI   time.Duration
    NumTurns      int
    CostUSD       float64
    Usage         Usage
    Result        string
    IsError       bool
}

type Error struct {
    MessageMeta
    Err error
}

type Usage struct {
    InputTokens  int
    OutputTokens int
    CacheRead    int
    CacheWrite   int
}
```

---

## Error Handling

Typed errors with rich data. Use `errors.As` for inspection.

### Error Types

```go
type StartError struct {
    Reason string
    Cause  error
}

type ProcessError struct {
    ExitCode int
    Stderr   string
}

type MaxTurnsError struct {
    Turns      int
    MaxAllowed int
    SessionID  string
}

type HookInterruptError struct {
    Hook   string
    Tool   string
    Reason string
}

type TaskError struct {
    SessionID string
    Message   string
}
```

### Usage

```go
result, err := a.Run(ctx, prompt)
if err != nil {
    var maxErr *agent.MaxTurnsError
    if errors.As(err, &maxErr) {
        log.Printf("hit %d turns (max %d)", maxErr.Turns, maxErr.MaxAllowed)
        return
    }
    
    var procErr *agent.ProcessError
    if errors.As(err, &procErr) {
        log.Printf("process died: exit %d: %s", procErr.ExitCode, procErr.Stderr)
        return
    }
    
    // generic error handling
    return err
}
```

---

## Complete Example

### Orchestration: Spec → Plan → Fan-out

```go
package main

import (
    "context"
    "log"
    "sync"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
    defer cancel()

    baseOpts := []agent.Option{
        agent.Model("claude-sonnet-4-5"),
        agent.WorkDir("/project"),
        agent.Tools("Bash", "Write", "Edit", "Read", "Glob", "Grep", "Task"),
        agent.PermissionMode(agent.AcceptEdits),
        
        // Environment
        agent.Env("TMPDIR", "/sandbox/tmp"),
        agent.Env("HOME", "/sandbox/home"),
        
        // Security hooks
        agent.PreToolUse(
            agent.AllowPaths("/project", "/sandbox"),
            agent.RedirectPath("/tmp", "/sandbox/tmp"),
            agent.DenyCommands("rm -rf /", "sudo"),
            agent.RequireCommand("make", "go build", "go test"),
        ),
        
        // Skills
        agent.SkillsDir("/skills/go"),
        agent.Skill("project-rules", projectRules),
        
        // Subagents for internal use
        agent.Subagent("test-runner",
            agent.SubagentDescription("Runs tests and reports results"),
            agent.SubagentTools("Bash", "Read"),
            agent.SubagentModel("haiku"),
        ),
        
        // Audit
        agent.AuditToFile("/var/log/agent/audit.jsonl"),
    }

    // Phase 1: Generate spec
    planner, err := agent.New(ctx, baseOpts...)
    if err != nil {
        log.Fatal(err)
    }
    
    _, err = planner.Run(ctx, "Analyze requirements.md and create spec.md")
    if err != nil {
        log.Fatal(err)
    }

    // Phase 2: Generate plan with structured output
    var plan Plan
    err = planner.Run(ctx, "Create implementation plan from spec.md",
        agent.Output(&plan),
    )
    if err != nil {
        log.Fatal(err)
    }
    planner.Close()

    // Phase 3: Fan-out workers
    var wg sync.WaitGroup
    results := make(chan StepResult, len(plan.Steps))

    for _, step := range plan.Steps {
        wg.Add(1)
        go func(s Step) {
            defer wg.Done()
            
            worker, err := agent.New(ctx, baseOpts...)
            if err != nil {
                results <- StepResult{Step: s, Err: err}
                return
            }
            defer worker.Close()

            result, err := worker.Run(ctx, s.Action,
                agent.Timeout(10*time.Minute),
                agent.MaxTurns(15),
            )
            results <- StepResult{Step: s, Result: result, Err: err}
        }(step)
    }

    wg.Wait()
    close(results)

    // Collect results
    for r := range results {
        if r.Err != nil {
            log.Printf("step %s failed: %v", r.Step.ID, r.Err)
        } else {
            log.Printf("step %s complete: cost $%.4f", r.Step.ID, r.Result.CostUSD)
        }
    }
}

type Plan struct {
    Title string `json:"title" desc:"Plan title"`
    Steps []Step `json:"steps" desc:"Implementation steps"`
}

type Step struct {
    ID        string   `json:"id" desc:"Step identifier"`
    Action    string   `json:"action" desc:"What to do"`
    DependsOn []string `json:"depends_on,omitempty" desc:"Prerequisites"`
}

type StepResult struct {
    Step   Step
    Result *agent.Result
    Err    error
}

var projectRules = `
# Project Rules

- Use make for all build operations
- Tests must pass before completion
- Follow Go conventions
`
```

### Interactive Agent with Stream

```go
func interactiveSession(ctx context.Context) error {
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Tools("Bash", "Write", "Edit", "Read"),
        agent.PermissionMode(agent.AcceptEdits),
    )
    if err != nil {
        return err
    }
    defer a.Close()

    reader := bufio.NewReader(os.Stdin)
    
    for {
        fmt.Print("\n> ")
        input, _ := reader.ReadString('\n')
        input = strings.TrimSpace(input)
        
        if input == "exit" {
            break
        }

        for msg := range a.Stream(ctx, input) {
            switch m := msg.(type) {
            case *agent.Text:
                fmt.Print(m.Text)
            case *agent.ToolUse:
                fmt.Printf("\n[using %s]\n", m.Name)
            case *agent.ToolResult:
                if m.IsError {
                    fmt.Printf("[tool error: %v]\n", m.Content)
                }
            case *agent.Result:
                fmt.Printf("\n[done: %d turns, $%.4f]\n", m.NumTurns, m.CostUSD)
            case *agent.Error:
                fmt.Printf("\n[error: %v]\n", m.Err)
            }
        }
        
        if err := a.Err(); err != nil {
            fmt.Printf("error: %v\n", err)
        }
    }
    
    return nil
}
```

---

## Run Options

Options that can be passed to `Run()` or `Stream()`:

```go
agent.Timeout(duration)      // override context timeout
agent.MaxTurns(n)            // override agent MaxTurns
agent.Model(name)            // override model for this run
agent.Output(&structPtr)     // structured output, unmarshal into struct
agent.OutputSchema(Type{})   // structured output, validate only
```

---

## Summary of Options

### Agent Creation Options

| Option | Description |
|--------|-------------|
| `Model(name)` | Claude model to use |
| `WorkDir(path)` | Working directory |
| `Env(key, value)` | Environment variable |
| `Tools(names...)` | Enabled tools |
| `PermissionMode(mode)` | Permission handling mode |
| `MaxTurns(n)` | Default max turns |
| `Resume(sessionID)` | Resume previous session |
| `Fork(sessionID)` | Fork from session |
| `SettingSources(sources...)` | Filesystem settings to load |
| `PreToolUse(hooks...)` | Pre-tool execution hooks |
| `PostToolUse(hooks...)` | Post-tool execution hooks |
| `UserPromptSubmit(hooks...)` | Prompt interception hooks |
| `Stop(hooks...)` | Agent stop hooks |
| `SubagentStop(hooks...)` | Subagent completion hooks |
| `PreCompact(hooks...)` | Pre-compaction hooks |
| `Skill(name, content)` | Inline skill |
| `SkillsDir(path)` | Skills directory |
| `SystemPromptPreset(name)` | Use preset system prompt |
| `SystemPromptAppend(text)` | Append to system prompt |
| `Subagent(name, opts...)` | Define subagent |
| `Tool(name, opts...)` | In-process custom tool |
| `MCPServer(name, opts...)` | External MCP server |
| `AuditToFile(path)` | Audit to JSON lines file |
| `AuditToWriter(w)` | Audit to writer |
| `AuditHandler(fn)` | Custom audit handler |

### Hook Higher-Order Functions

| Function | Description |
|----------|-------------|
| `DenyCommands(patterns...)` | Block matching bash commands |
| `RequireCommand(use, instead...)` | Require alternative command |
| `AllowPaths(paths...)` | Allow only these paths |
| `DenyPaths(paths...)` | Deny these paths |
| `RedirectPath(from, to)` | Rewrite paths |

### Run/Stream Options

| Option | Description |
|--------|-------------|
| `Timeout(duration)` | Override timeout |
| `MaxTurns(n)` | Override max turns |
| `Model(name)` | Override model |
| `Output(&ptr)` | Structured output with unmarshal |
| `OutputSchema(type)` | Structured output validation only |


