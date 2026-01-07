# Sessions

A session represents a continuous conversation between your application and Claude. Each agent maintains a session that
persists across multiple `Run` or `Stream` calls, allowing Claude to remember context from previous interactions. This
document explains session management, lazy initialization, turn limits, and session persistence patterns.

## Session ID

Every agent session has a unique identifier assigned by the Claude Code CLI. This ID is used for:

- Resuming sessions after an agent is closed
- Correlating audit events with specific sessions
- Tracking conversation state in logs

### Lazy Initialization

The session ID is not available immediately after calling `agent.New`. Due to the stream-JSON protocol, the CLI waits
for the first user message before sending initialization data.

```go
a, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))

// Session ID is empty until first interaction
fmt.Println(a.SessionID()) // ""

// First interaction triggers initialization
_, _ = a.Run(ctx, "Hello")

// Now the session ID is available
fmt.Println(a.SessionID()) // "sess_abc123..."
```

This design choice enables:

1. Non-blocking agent creation
2. Concurrent agent initialization
3. Deferred resource allocation

If your application needs the session ID before the first interaction, you must send an initial prompt first:

```go
a, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
_, _ = a.Run(ctx, "Initialize") // Throwaway prompt to trigger init
sessionID := a.SessionID()
```

## Turn Tracking

The SDK tracks turns across all `Run` calls within a session. A turn represents a complete assistant response, which may
include multiple tool uses and text outputs.

### Cumulative Turns

The agent maintains a cumulative turn count:

```go
a, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
defer a.Close()

// Turn 1
result1, _ := a.Run(ctx, "What is 2+2?")
// result1.NumTurns = 1

// Turn 2 (cumulative from session start)
result2, _ := a.Run(ctx, "Multiply that by 3")
// result2.NumTurns = 1 (for this run)
// Total session turns = 2
```

Each `Result` reports the turns consumed by that specific run. The total across all runs is available through the
`OnStop` hook.

### Cost Accumulation

Similar to turns, the SDK accumulates API cost across the session:

```go
a, _ := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.OnStop(func(e *agent.StopEvent) {
        fmt.Printf("Session total: %d turns, $%.4f\n",
            e.NumTurns, e.CostUSD)
    }),
)
defer a.Close()

_, _ = a.Run(ctx, "Task 1") // $0.0050
_, _ = a.Run(ctx, "Task 2") // $0.0030
// OnStop reports: 2+ turns, $0.0080
```

## MaxTurns Option

The `MaxTurns` option limits how many turns an agent can execute. This prevents runaway sessions and controls costs.

### Agent-Level Limit

Set at agent creation to apply across all runs:

```go
a, _ := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.MaxTurns(10),
)
defer a.Close()

// Each Run consumes turns from the limit
_, _ = a.Run(ctx, "Task 1") // Uses 2 turns, 8 remaining
_, _ = a.Run(ctx, "Task 2") // Uses 3 turns, 5 remaining
_, _ = a.Run(ctx, "Task 3") // Uses 6 turns, exceeds limit
```

When the limit is exceeded, `Run` returns a `MaxTurnsError`:

```go
result, err := a.Run(ctx, "Another task")
if err != nil {
    var maxErr *agent.MaxTurnsError
    if errors.As(err, &maxErr) {
        fmt.Printf("Exceeded limit: %d/%d turns\n",
            maxErr.Turns, maxErr.MaxAllowed)
        // result may still contain partial data
    }
}
```

### Per-Run Override

Override the agent-level limit for a specific run:

```go
a, _ := agent.New(ctx, agent.MaxTurns(10)) // Default limit

// This run has a higher limit
result, err := a.Run(ctx, "Complex task",
    agent.MaxTurnsRun(20),
)
```

The per-run limit takes precedence over the agent-level limit for that specific invocation.

### Pre-Run vs Post-Run Checks

The SDK performs two turn checks:

1. **Pre-run**: Before sending the prompt, if cumulative turns already exceed the limit, `Run` returns `MaxTurnsError`
   immediately without sending anything to Claude.

2. **Post-run**: After receiving the result, if cumulative turns now exceed the limit, `Run` returns both the result and
   `MaxTurnsError`.

```go
a, _ := agent.New(ctx, agent.MaxTurns(5))

// Assume previous runs consumed 4 turns

// Pre-run check passes (4 < 5)
result, err := a.Run(ctx, "Task")
// This run used 2 turns, total now 6

// Post-run check fails (6 > 5)
// result is non-nil with the response
// err is *MaxTurnsError
```

This design ensures you receive any completed work even when hitting the limit.

## Session Persistence

The Claude Code CLI maintains session state in transcript files. The SDK provides options to resume or fork from
existing sessions.

### Resume (Coming Soon)

Continue a previous session by its ID:

```go
// First session
a1, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
_, _ = a1.Run(ctx, "Remember the number 42")
sessionID := a1.SessionID()
a1.Close()

// Resume later
a2, _ := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.Resume(sessionID),
)
result, _ := a2.Run(ctx, "What number did I mention?")
// Claude remembers the previous conversation
```

Resume preserves:

- Conversation history
- Accumulated context
- Tool execution results

Note: The session must have been properly closed, and the transcript must still exist.

### Fork (Coming Soon)

Create a new session branching from an existing one:

```go
// Original session
a1, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
_, _ = a1.Run(ctx, "Context setup")
sessionID := a1.SessionID()
a1.Close()

// Fork creates a new session with copied context
a2, _ := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.Fork(sessionID),
)
// a2 has a NEW session ID
// a2 has the conversation history from a1
// Changes to a2 do not affect the original session
```

Fork is useful for:

- Trying different approaches without losing the original conversation
- Creating checkpoints before risky operations
- A/B testing different prompts with identical context

## Long-Running Session Patterns

### Bounded Sessions

For batch processing, create fresh sessions for each unit of work:

```go
func processItems(items []string) {
    for _, item := range items {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

        a, _ := agent.New(ctx,
            agent.Model("claude-sonnet-4-5"),
            agent.MaxTurns(10),
        )

        _, err := a.Run(ctx, fmt.Sprintf("Process: %s", item))
        a.Close()
        cancel()

        if err != nil {
            log.Printf("Failed to process %s: %v", item, err)
        }
    }
}
```

Each item gets a fresh session with its own turn limit.

### Monitored Long Sessions

For interactive applications, monitor session health:

```go
a, _ := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.MaxTurns(100),
    agent.OnStop(func(e *agent.StopEvent) {
        if e.Reason == agent.StopMaxTurns {
            log.Printf("Session hit turn limit: %s", e.SessionID)
        }
    }),
    agent.PreCompact(func(e *agent.PreCompactEvent) agent.PreCompactResult {
        log.Printf("Context compacting at %d tokens", e.TokenCount)
        return agent.PreCompactResult{
            Archive:   true,
            ArchiveTo: fmt.Sprintf("archives/%s.json", e.SessionID),
        }
    }),
)
defer a.Close()

// Long-running interaction loop
for prompt := range prompts {
    result, err := a.Run(ctx, prompt)
    if err != nil {
        var maxErr *agent.MaxTurnsError
        if errors.As(err, &maxErr) {
            // Hit limit, need to start new session
            break
        }
    }
    // Process result
}
```

### Session Checkpointing

Save session IDs for later resumption:

```go
type Checkpoint struct {
    SessionID string
    Turns     int
    Cost      float64
    Timestamp time.Time
}

func withCheckpointing(ctx context.Context, save func(Checkpoint)) {
    var checkpoint Checkpoint

    a, _ := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.OnStop(func(e *agent.StopEvent) {
            checkpoint = Checkpoint{
                SessionID: e.SessionID,
                Turns:     e.NumTurns,
                Cost:      e.CostUSD,
                Timestamp: time.Now(),
            }
            save(checkpoint)
        }),
    )
    defer a.Close()

    // Use agent...
}
```

## Context Compaction

Claude Code automatically compacts the context window when approaching token limits. The SDK provides hooks to observe
and react to compaction events.

### PreCompact Hook

Called before compaction occurs:

```go
a, _ := agent.New(ctx,
    agent.PreCompact(func(e *agent.PreCompactEvent) agent.PreCompactResult {
        log.Printf("Compacting session %s", e.SessionID)
        log.Printf("Trigger: %s", e.Trigger)        // "auto" or "manual"
        log.Printf("Token count: %d", e.TokenCount)
        log.Printf("Transcript: %s", e.TranscriptPath)

        return agent.PreCompactResult{
            Archive:   true,
            ArchiveTo: "/archives/transcript.json",
            Extract:   map[string]any{"key_facts": "..."},
        }
    }),
)
```

Use `PreCompact` to:

- Archive transcripts before information is lost
- Extract important data from the conversation
- Log compaction events for debugging

## Audit Integration

Sessions integrate with the audit system for observability:

```go
a, _ := agent.New(ctx,
    agent.AuditToFile("audit.jsonl"),
    agent.Audit(func(e agent.AuditEvent) {
        if e.Type == "session.init" {
            log.Printf("Session initialized: %s", e.SessionID)
        }
        if e.Type == "session.end" {
            log.Printf("Session ended: %s", e.SessionID)
        }
    }),
)
```

Session-related audit events:

| Event Type      | When               | Data                                       |
|-----------------|--------------------|--------------------------------------------|
| `session.start` | Agent created      | -                                          |
| `session.init`  | First message sent | `transcript_path`, `tools`, `mcp_servers`  |
| `session.end`   | Agent closed       | `total_turns`, `total_cost`, `stop_reason` |

## Complete Example

The following example demonstrates session management with turn limits, monitoring, and checkpointing:

```go
package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

type SessionState struct {
    SessionID string    `json:"session_id"`
    Turns     int       `json:"turns"`
    Cost      float64   `json:"cost"`
    SavedAt   time.Time `json:"saved_at"`
}

func saveState(state SessionState) error {
    data, _ := json.Marshal(state)
    return os.WriteFile("session_state.json", data, 0644)
}

func loadState() (SessionState, error) {
    data, err := os.ReadFile("session_state.json")
    if err != nil {
        return SessionState{}, err
    }
    var state SessionState
    err = json.Unmarshal(data, &state)
    return state, err
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    // Try to resume previous session
    var opts []agent.Option
    opts = append(opts,
        agent.Model("claude-sonnet-4-5"),
        agent.MaxTurns(50),
    )

    if state, err := loadState(); err == nil {
        log.Printf("Resuming session %s (%d turns, $%.4f)",
            state.SessionID, state.Turns, state.Cost)
        opts = append(opts, agent.Resume(state.SessionID))
    }

    // Add monitoring
    opts = append(opts,
        agent.OnStop(func(e *agent.StopEvent) {
            state := SessionState{
                SessionID: e.SessionID,
                Turns:     e.NumTurns,
                Cost:      e.CostUSD,
                SavedAt:   time.Now(),
            }
            if err := saveState(state); err != nil {
                log.Printf("Failed to save state: %v", err)
            }
            log.Printf("Session %s: %s (%d turns, $%.4f)",
                e.SessionID, e.Reason, e.NumTurns, e.CostUSD)
        }),
        agent.PreCompact(func(e *agent.PreCompactEvent) agent.PreCompactResult {
            log.Printf("Context compacting: %d tokens", e.TokenCount)
            return agent.PreCompactResult{
                Archive:   true,
                ArchiveTo: fmt.Sprintf("archives/%s-%d.json",
                    e.SessionID, time.Now().Unix()),
            }
        }),
    )

    a, err := agent.New(ctx, opts...)
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }
    defer a.Close()

    // Interactive loop
    prompts := []string{
        "Create a new Go module called 'demo'",
        "Add a main.go with a hello world function",
        "Add unit tests for the hello function",
    }

    for _, prompt := range prompts {
        result, err := a.Run(ctx, prompt)
        if err != nil {
            var maxErr *agent.MaxTurnsError
            if errors.As(err, &maxErr) {
                log.Printf("Hit turn limit (%d/%d), saving state",
                    maxErr.Turns, maxErr.MaxAllowed)
                break
            }
            log.Printf("Run error: %v", err)
            continue
        }

        fmt.Printf("\n--- Response ---\n%s\n", result.ResultText)
        fmt.Printf("(turns: %d, cost: $%.4f)\n", result.NumTurns, result.CostUSD)
    }

    fmt.Printf("\nSession ID: %s\n", a.SessionID())
}
```

## Feature Status

| Feature       | Status      | Notes                             |
|---------------|-------------|-----------------------------------|
| Session ID    | Implemented | Lazy initialization               |
| Turn tracking | Implemented | Cumulative across runs            |
| Cost tracking | Implemented | Available via OnStop              |
| MaxTurns      | Implemented | Agent-level and per-run           |
| Resume        | Implemented | Requires valid session ID         |
| Fork          | Implemented | Creates new session from existing |
| PreCompact    | Implemented | Archive before compaction         |

## Related Documentation

- [Agents](agents.md) - Agent lifecycle and execution modes
- [Hooks](hooks.md) - Hook system for observability
- Source files:
    - `/Users/wstrydom/Developer/wernerstrydom/claude-agent-sdk-go/agent/agent.go`
    - `/Users/wstrydom/Developer/wernerstrydom/claude-agent-sdk-go/agent/options.go`
