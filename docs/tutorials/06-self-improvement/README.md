# Tutorial 06: Self-Improving Agent System

This tutorial demonstrates how to build a self-improving agent system that learns from its outputs. The system captures
session data, extracts actionable lessons, and generates skills that enhance future agent sessions.

## What is a Self-Improving Agent?

A self-improving agent is a system that analyzes its own execution history to identify patterns, extract lessons, and
update its behavior for future tasks. The improvement cycle consists of:

1. **Capture** - Record all agent outputs during a session
2. **Analyze** - Process the session transcript to identify what worked and what failed
3. **Extract** - Convert observations into structured lessons
4. **Apply** - Generate skills that inject the lessons into future sessions

This creates a feedback loop where the agent accumulates domain knowledge over time. Each session contributes to a
growing skill library that makes subsequent sessions more effective.

```
+----------------+      +-----------------+      +------------------+
|   Run Session  | ---> | Extract Lessons | ---> |  Generate Skills |
+----------------+      +-----------------+      +------------------+
        ^                                                 |
        |                                                 |
        +-------------------------------------------------+
                    Load Skills in Next Session
```

The self-improvement pattern is particularly valuable when:

- Tasks share common patterns that benefit from accumulated knowledge
- Errors reveal important constraints that should be remembered
- Domain-specific conventions emerge through practice
- The agent encounters recurring problems with known solutions

## Prerequisites

Before starting this tutorial, ensure you have:

1. Completed the [Installation](../../getting-started/installation.md) guide
2. Go 1.21 or later installed
3. Claude Code CLI installed and authenticated
4. Basic familiarity with Go programming
5. Understanding of JSON marshaling and file I/O

## Data Structures

The self-improvement system uses two primary data structures to represent lessons.

### Lesson Schema

A lesson captures a single piece of knowledge extracted from a session:

```go
// Lesson represents a single piece of knowledge extracted from a session.
// Lessons are the atomic unit of learning in the self-improvement system.
type Lesson struct {
    // Context describes when this lesson applies.
    // Examples: "When writing Go tests", "When parsing JSON input"
    Context string `json:"context"`

    // Learning describes what was learned.
    // This should be actionable guidance, not just an observation.
    Learning string `json:"learning"`

    // Example provides a concrete illustration of the lesson.
    // Examples make lessons more memorable and easier to apply.
    Example string `json:"example"`

    // Tags categorize the lesson for filtering and organization.
    // Use consistent tags like "error-handling", "testing", "performance".
    Tags []string `json:"tags"`
}
```

Each field serves a specific purpose:

| Field      | Purpose                 | Example Value                                        |
|------------|-------------------------|------------------------------------------------------|
| `Context`  | When the lesson applies | "When validating patient records"                    |
| `Learning` | The actionable guidance | "Always check for nil date fields before comparison" |
| `Example`  | Concrete illustration   | "if admission.Date != nil { ... }"                   |
| `Tags`     | Categorization          | ["validation", "nil-safety", "healthcare"]           |

### Session Lessons Schema

Session lessons aggregate all lessons from a single session:

```go
// SessionLessons aggregates lessons extracted from a single agent session.
type SessionLessons struct {
    // SessionID uniquely identifies the source session.
    SessionID string `json:"session_id"`

    // Timestamp records when the lessons were extracted.
    Timestamp time.Time `json:"timestamp"`

    // TaskDescription summarizes what the session was doing.
    TaskDescription string `json:"task_description"`

    // Lessons contains all extracted lessons from this session.
    Lessons []Lesson `json:"lessons"`
}
```

## Session Output Capture

The first step in the improvement cycle is capturing session outputs. The SDK provides several mechanisms for recording
agent activity.

### Using Audit Handlers

The audit system provides the most comprehensive capture mechanism. Register an audit handler that stores events to a
file:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// SessionRecorder captures all messages from an agent session.
type SessionRecorder struct {
    SessionID string
    Messages  []RecordedMessage
    Started   time.Time
    Ended     time.Time
}

// RecordedMessage stores a single message with metadata.
type RecordedMessage struct {
    Timestamp time.Time `json:"timestamp"`
    Type      string    `json:"type"`
    Content   any       `json:"content"`
}

// NewSessionRecorder creates a recorder that captures audit events.
func NewSessionRecorder() (*SessionRecorder, agent.AuditHandler) {
    rec := &SessionRecorder{
        Messages: make([]RecordedMessage, 0),
        Started:  time.Now(),
    }

    handler := func(e agent.AuditEvent) {
        if rec.SessionID == "" && e.SessionID != "" {
            rec.SessionID = e.SessionID
        }
        rec.Messages = append(rec.Messages, RecordedMessage{
            Timestamp: e.Time,
            Type:      e.Type,
            Content:   e.Data,
        })
    }

    return rec, handler
}

// SaveToFile writes the recorded session to a JSON file.
func (r *SessionRecorder) SaveToFile(dir string) error {
    r.Ended = time.Now()

    filename := fmt.Sprintf("session-%s.json", r.SessionID)
    path := filepath.Join(dir, filename)

    data, err := json.MarshalIndent(r, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal session: %w", err)
    }

    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }

    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("failed to write file: %w", err)
    }

    return nil
}
```

### Capturing Tool Calls and Results

Tool calls are particularly valuable for learning because they show what the agent attempted and whether it succeeded.
The audit handler captures these automatically:

```go
func runWithCapture(ctx context.Context, prompt string) (*SessionRecorder, error) {
    recorder, auditHandler := NewSessionRecorder()

    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Audit(auditHandler),
        agent.PostToolUse(func(tc *agent.ToolCall, tr *agent.ToolResultContext) agent.HookResult {
            // PostToolUse provides detailed tool execution data
            // The audit handler already captures this, but you can add
            // additional processing here if needed
            return agent.HookResult{Decision: agent.Continue}
        }),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create agent: %w", err)
    }
    defer a.Close()

    result, err := a.Run(ctx, prompt)
    if err != nil {
        return nil, fmt.Errorf("agent run failed: %w", err)
    }

    // Store result metadata
    recorder.Messages = append(recorder.Messages, RecordedMessage{
        Timestamp: time.Now(),
        Type:      "result",
        Content: map[string]any{
            "cost_usd":   result.CostUSD,
            "num_turns":  result.NumTurns,
            "is_error":   result.IsError,
            "duration":   result.DurationTotal.String(),
            "result":     result.ResultText,
        },
    })

    return recorder, nil
}
```

### Tracking Success and Failure

To extract meaningful lessons, the system must distinguish successful operations from failures. Add explicit tracking:

```go
// SessionOutcome summarizes the overall result of a session.
type SessionOutcome struct {
    Success       bool     `json:"success"`
    ErrorMessages []string `json:"error_messages,omitempty"`
    ToolFailures  []string `json:"tool_failures,omitempty"`
    Warnings      []string `json:"warnings,omitempty"`
}

// AnalyzeOutcome examines recorded messages to determine session outcome.
func (r *SessionRecorder) AnalyzeOutcome() SessionOutcome {
    outcome := SessionOutcome{
        Success:       true,
        ErrorMessages: make([]string, 0),
        ToolFailures:  make([]string, 0),
        Warnings:      make([]string, 0),
    }

    for _, msg := range r.Messages {
        switch msg.Type {
        case "error":
            outcome.Success = false
            if data, ok := msg.Content.(map[string]any); ok {
                if errMsg, ok := data["error"].(string); ok {
                    outcome.ErrorMessages = append(outcome.ErrorMessages, errMsg)
                }
            }
        case "message.tool_result":
            if data, ok := msg.Content.(map[string]any); ok {
                if isError, ok := data["is_error"].(bool); ok && isError {
                    outcome.ToolFailures = append(outcome.ToolFailures,
                        fmt.Sprintf("Tool execution failed"))
                }
            }
        }
    }

    return outcome
}
```

## Lesson Extraction Agent

The second step uses another agent to analyze session transcripts and extract lessons. This agent receives the raw
session data and returns structured lessons.

### Extraction Prompt

The extraction prompt instructs the agent on how to analyze sessions:

```go
const extractionPrompt = `Analyze the following agent session transcript and extract actionable lessons.

For each lesson, provide:
1. Context - When does this lesson apply?
2. Learning - What should be done differently or remembered?
3. Example - A concrete code or command example
4. Tags - Categories for organization

Focus on:
- Errors and how they were resolved
- Patterns that led to success
- Constraints or limitations discovered
- Domain-specific conventions used

Return lessons as JSON matching this schema:
{
  "lessons": [
    {
      "context": "string",
      "learning": "string",
      "example": "string",
      "tags": ["string"]
    }
  ]
}

Session transcript:
%s`
```

### Running the Extraction

Use structured output to ensure the agent returns properly formatted lessons:

```go
// LessonExtraction is the schema for extraction output.
type LessonExtraction struct {
    Lessons []Lesson `json:"lessons" desc:"Extracted lessons from the session"`
}

// ExtractLessons analyzes a session and returns structured lessons.
func ExtractLessons(ctx context.Context, recorder *SessionRecorder) (*SessionLessons, error) {
    // Serialize the session for analysis
    sessionData, err := json.MarshalIndent(recorder, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("failed to serialize session: %w", err)
    }

    prompt := fmt.Sprintf(extractionPrompt, string(sessionData))

    var extraction LessonExtraction
    result, err := agent.RunStructured(ctx, prompt, &extraction,
        agent.Model("claude-sonnet-4-5"),
    )
    if err != nil {
        return nil, fmt.Errorf("extraction failed: %w", err)
    }

    // Check that extraction was successful
    if result.IsError {
        return nil, fmt.Errorf("extraction returned error: %s", result.ResultText)
    }

    return &SessionLessons{
        SessionID:       recorder.SessionID,
        Timestamp:       time.Now(),
        TaskDescription: summarizeTask(recorder),
        Lessons:         extraction.Lessons,
    }, nil
}

// summarizeTask creates a brief description of what the session did.
func summarizeTask(r *SessionRecorder) string {
    // Look for the first prompt in the session
    for _, msg := range r.Messages {
        if msg.Type == "message.prompt" {
            if data, ok := msg.Content.(map[string]any); ok {
                if prompt, ok := data["prompt"].(string); ok {
                    // Truncate long prompts
                    if len(prompt) > 100 {
                        return prompt[:100] + "..."
                    }
                    return prompt
                }
            }
        }
    }
    return "Unknown task"
}
```

### Filtering and Validating Lessons

Not all extracted lessons are valuable. Add validation to filter low-quality lessons:

```go
// ValidateLesson checks whether a lesson meets quality standards.
func ValidateLesson(l Lesson) error {
    if l.Context == "" {
        return fmt.Errorf("lesson missing context")
    }
    if l.Learning == "" {
        return fmt.Errorf("lesson missing learning")
    }
    if len(l.Learning) < 20 {
        return fmt.Errorf("learning too brief to be actionable")
    }
    if len(l.Tags) == 0 {
        return fmt.Errorf("lesson missing tags")
    }
    return nil
}

// FilterValidLessons removes lessons that fail validation.
func FilterValidLessons(lessons []Lesson) []Lesson {
    valid := make([]Lesson, 0, len(lessons))
    for _, l := range lessons {
        if err := ValidateLesson(l); err == nil {
            valid = append(valid, l)
        }
    }
    return valid
}
```

## Skill Generation

Lessons become useful when they are injected into future sessions as skills. A skill is a markdown document that Claude
loads into its context.

### Skill File Format

The SDK loads skills from files matching the pattern `*.skill.md` or from directories containing `SKILL.md`:

```go
// GenerateSkillContent converts lessons into skill markdown.
func GenerateSkillContent(lessons *SessionLessons) string {
    var sb strings.Builder

    sb.WriteString("# Lessons Learned\n\n")
    sb.WriteString(fmt.Sprintf("Extracted from session: %s\n", lessons.SessionID))
    sb.WriteString(fmt.Sprintf("Task: %s\n\n", lessons.TaskDescription))

    for i, lesson := range lessons.Lessons {
        sb.WriteString(fmt.Sprintf("## Lesson %d: %s\n\n", i+1, lesson.Context))
        sb.WriteString(fmt.Sprintf("**When**: %s\n\n", lesson.Context))
        sb.WriteString(fmt.Sprintf("**Guidance**: %s\n\n", lesson.Learning))

        if lesson.Example != "" {
            sb.WriteString("**Example**:\n```\n")
            sb.WriteString(lesson.Example)
            sb.WriteString("\n```\n\n")
        }

        if len(lesson.Tags) > 0 {
            sb.WriteString(fmt.Sprintf("*Tags: %s*\n\n", strings.Join(lesson.Tags, ", ")))
        }
    }

    return sb.String()
}
```

### Saving Skills

Write skills to a directory that the SDK can load:

```go
// SaveAsSkill writes lessons to a skill file.
func SaveAsSkill(lessons *SessionLessons, skillsDir string) error {
    if err := os.MkdirAll(skillsDir, 0755); err != nil {
        return fmt.Errorf("failed to create skills directory: %w", err)
    }

    // Generate a unique skill name from session ID
    skillName := fmt.Sprintf("learned-%s.skill.md", lessons.SessionID[:8])
    skillPath := filepath.Join(skillsDir, skillName)

    content := GenerateSkillContent(lessons)

    if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
        return fmt.Errorf("failed to write skill file: %w", err)
    }

    return nil
}
```

### Loading Skills in Future Sessions

When creating an agent, load the skills directory to inject accumulated knowledge:

```go
func createAgentWithSkills(ctx context.Context, skillsDir string) (*agent.Agent, error) {
    return agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.SkillsDir(skillsDir),
    )
}
```

The SDK automatically loads all `*.skill.md` files from the specified directory and injects their content into Claude's
context.

## Long Sessions and Context Management

Long-running tasks may exceed Claude's context window. The SDK provides mechanisms to handle this, though some are still
in development.

### Session Limits with MaxTurns

Use `MaxTurns` to limit session length and force periodic checkpoints:

```go
func runWithLimits(ctx context.Context, prompt string) (*agent.Result, error) {
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.MaxTurns(10), // Limit to 10 turns
    )
    if err != nil {
        return nil, err
    }
    defer a.Close()

    result, err := a.Run(ctx, prompt)

    // Check if we hit the turn limit
    var maxTurnsErr *agent.MaxTurnsError
    if errors.As(err, &maxTurnsErr) {
        // Session ended due to turn limit, not failure
        // The result still contains useful data
        return result, nil
    }

    return result, err
}
```

### Resuming Sessions

The SDK supports resuming previous sessions using the session ID:

```go
func resumeSession(ctx context.Context, sessionID string, prompt string) (*agent.Result, error) {
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Resume(sessionID), // Continue from previous session
    )
    if err != nil {
        return nil, err
    }
    defer a.Close()

    return a.Run(ctx, prompt)
}
```

### Forking Sessions

Use `Fork` to branch from an existing session without modifying it:

```go
func forkSession(ctx context.Context, sessionID string, prompt string) (*agent.Result, error) {
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Fork(sessionID), // Create a new branch
    )
    if err != nil {
        return nil, err
    }
    defer a.Close()

    return a.Run(ctx, prompt)
}
```

### Context Compaction Hooks

The `PreCompact` hook allows you to archive context before compaction:

```go
func createAgentWithCompaction(ctx context.Context, archiveDir string) (*agent.Agent, error) {
    return agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.PreCompact(func(e *agent.PreCompactEvent) agent.PreCompactResult {
            // Archive the transcript before compaction
            archivePath := filepath.Join(archiveDir,
                fmt.Sprintf("compact-%d.json", time.Now().Unix()))

            return agent.PreCompactResult{
                Archive:   true,
                ArchiveTo: archivePath,
            }
        }),
    )
}
```

## Complete Working Example

This section provides a complete, runnable implementation of the self-improvement system.

### Project Structure

```
self-improve/
├── go.mod
├── main.go
├── lesson.go
├── recorder.go
├── extractor.go
├── skill.go
└── skills/
    └── .gitkeep
```

### Core Types (lesson.go)

```go
package main

import (
    "time"
)

// Lesson represents a single piece of knowledge extracted from a session.
type Lesson struct {
    Context  string   `json:"context"`
    Learning string   `json:"learning"`
    Example  string   `json:"example"`
    Tags     []string `json:"tags"`
}

// SessionLessons aggregates lessons from a single session.
type SessionLessons struct {
    SessionID       string    `json:"session_id"`
    Timestamp       time.Time `json:"timestamp"`
    TaskDescription string    `json:"task_description"`
    Lessons         []Lesson  `json:"lessons"`
}

// LessonExtraction is the structured output schema.
type LessonExtraction struct {
    Lessons []Lesson `json:"lessons" desc:"Extracted lessons from the session"`
}
```

### Session Recorder (recorder.go)

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// SessionRecorder captures agent session data.
type SessionRecorder struct {
    SessionID string            `json:"session_id"`
    Messages  []RecordedMessage `json:"messages"`
    Started   time.Time         `json:"started"`
    Ended     time.Time         `json:"ended,omitempty"`
}

// RecordedMessage stores a message with metadata.
type RecordedMessage struct {
    Timestamp time.Time `json:"timestamp"`
    Type      string    `json:"type"`
    Content   any       `json:"content"`
}

// NewSessionRecorder creates a recorder with an audit handler.
func NewSessionRecorder() (*SessionRecorder, agent.AuditHandler) {
    rec := &SessionRecorder{
        Messages: make([]RecordedMessage, 0),
        Started:  time.Now(),
    }

    handler := func(e agent.AuditEvent) {
        if rec.SessionID == "" && e.SessionID != "" {
            rec.SessionID = e.SessionID
        }
        rec.Messages = append(rec.Messages, RecordedMessage{
            Timestamp: e.Time,
            Type:      e.Type,
            Content:   e.Data,
        })
    }

    return rec, handler
}

// Finalize marks the session as complete.
func (r *SessionRecorder) Finalize() {
    r.Ended = time.Now()
}

// SaveToFile writes the session to JSON.
func (r *SessionRecorder) SaveToFile(dir string) error {
    filename := fmt.Sprintf("session-%s.json", r.SessionID)
    if r.SessionID == "" {
        filename = fmt.Sprintf("session-%d.json", time.Now().Unix())
    }
    path := filepath.Join(dir, filename)

    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }

    data, err := json.MarshalIndent(r, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal session: %w", err)
    }

    return os.WriteFile(path, data, 0644)
}

// GetTaskDescription extracts the task from the first prompt.
func (r *SessionRecorder) GetTaskDescription() string {
    for _, msg := range r.Messages {
        if msg.Type == "message.prompt" {
            if data, ok := msg.Content.(map[string]any); ok {
                if prompt, ok := data["prompt"].(string); ok {
                    if len(prompt) > 100 {
                        return prompt[:100] + "..."
                    }
                    return prompt
                }
            }
        }
    }
    return "Unknown task"
}
```

### Lesson Extractor (extractor.go)

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

const extractionPromptTemplate = `Analyze this agent session transcript and extract actionable lessons.

For each lesson:
- Context: When does this apply? Be specific.
- Learning: What should be done? Be actionable.
- Example: Show concrete code or commands.
- Tags: Categorize with 2-4 tags.

Focus on:
- Errors encountered and their solutions
- Successful patterns worth repeating
- Constraints or limitations discovered
- Domain conventions used

Return JSON:
{
  "lessons": [
    {
      "context": "When doing X",
      "learning": "Always do Y because Z",
      "example": "code or command here",
      "tags": ["tag1", "tag2"]
    }
  ]
}

Session transcript:
%s`

// ExtractLessons analyzes a session and returns lessons.
func ExtractLessons(ctx context.Context, recorder *SessionRecorder) (*SessionLessons, error) {
    sessionData, err := json.MarshalIndent(recorder, "", "  ")
    if err != nil {
        return nil, fmt.Errorf("failed to serialize session: %w", err)
    }

    prompt := fmt.Sprintf(extractionPromptTemplate, string(sessionData))

    var extraction LessonExtraction
    result, err := agent.RunStructured(ctx, prompt, &extraction,
        agent.Model("claude-sonnet-4-5"),
    )
    if err != nil {
        return nil, fmt.Errorf("extraction failed: %w", err)
    }

    if result.IsError {
        return nil, fmt.Errorf("extraction error: %s", result.ResultText)
    }

    // Filter invalid lessons
    valid := make([]Lesson, 0, len(extraction.Lessons))
    for _, l := range extraction.Lessons {
        if l.Context != "" && l.Learning != "" && len(l.Tags) > 0 {
            valid = append(valid, l)
        }
    }

    return &SessionLessons{
        SessionID:       recorder.SessionID,
        Timestamp:       time.Now(),
        TaskDescription: recorder.GetTaskDescription(),
        Lessons:         valid,
    }, nil
}
```

### Skill Generator (skill.go)

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// GenerateSkillContent creates markdown from lessons.
func GenerateSkillContent(lessons *SessionLessons) string {
    var sb strings.Builder

    sb.WriteString("# Learned Patterns\n\n")
    sb.WriteString(fmt.Sprintf("Source: Session %s\n\n", lessons.SessionID))
    sb.WriteString(fmt.Sprintf("Task: %s\n\n", lessons.TaskDescription))
    sb.WriteString("---\n\n")

    for i, lesson := range lessons.Lessons {
        sb.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, lesson.Context))
        sb.WriteString(fmt.Sprintf("**Guidance**: %s\n\n", lesson.Learning))

        if lesson.Example != "" {
            sb.WriteString("**Example**:\n")
            sb.WriteString("```\n")
            sb.WriteString(lesson.Example)
            sb.WriteString("\n```\n\n")
        }

        sb.WriteString(fmt.Sprintf("Tags: `%s`\n\n", strings.Join(lesson.Tags, "`, `")))
    }

    return sb.String()
}

// SaveAsSkill writes lessons to a skill file.
func SaveAsSkill(lessons *SessionLessons, skillsDir string) (string, error) {
    if err := os.MkdirAll(skillsDir, 0755); err != nil {
        return "", fmt.Errorf("failed to create skills dir: %w", err)
    }

    // Use first 8 chars of session ID for filename
    sessionPrefix := lessons.SessionID
    if len(sessionPrefix) > 8 {
        sessionPrefix = sessionPrefix[:8]
    }
    if sessionPrefix == "" {
        sessionPrefix = "unknown"
    }

    skillName := fmt.Sprintf("learned-%s.skill.md", sessionPrefix)
    skillPath := filepath.Join(skillsDir, skillName)

    content := GenerateSkillContent(lessons)
    if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
        return "", fmt.Errorf("failed to write skill: %w", err)
    }

    return skillPath, nil
}
```

### Main Program (main.go)

```go
package main

import (
    "context"
    "flag"
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
    // Parse command line flags
    skillsDir := flag.String("skills", "./skills", "Directory for skill files")
    sessionsDir := flag.String("sessions", "./sessions", "Directory for session logs")
    extractOnly := flag.String("extract", "", "Extract lessons from existing session file")
    flag.Parse()

    ctx, cancel := signal.NotifyContext(
        context.Background(),
        syscall.SIGINT,
        syscall.SIGTERM,
    )
    defer cancel()

    // Mode 1: Extract lessons from existing session
    if *extractOnly != "" {
        return extractFromFile(ctx, *extractOnly, *skillsDir)
    }

    // Mode 2: Run a task with lesson extraction
    if flag.NArg() == 0 {
        fmt.Println("Usage: self-improve [flags] <prompt>")
        fmt.Println("       self-improve -extract <session.json>")
        flag.PrintDefaults()
        return nil
    }

    prompt := flag.Arg(0)
    return runAndLearn(ctx, prompt, *skillsDir, *sessionsDir)
}

func runAndLearn(ctx context.Context, prompt, skillsDir, sessionsDir string) error {
    // Step 1: Create recorder and agent
    recorder, auditHandler := NewSessionRecorder()

    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Audit(auditHandler),
        agent.SkillsDir(skillsDir), // Load existing skills
    )
    if err != nil {
        return fmt.Errorf("failed to create agent: %w", err)
    }
    defer a.Close()

    // Step 2: Run the task with streaming output
    fmt.Println("Running task...")
    fmt.Println()

    for msg := range a.Stream(ctx, prompt) {
        switch m := msg.(type) {
        case *agent.Text:
            fmt.Print(m.Text)
        case *agent.ToolUse:
            fmt.Printf("\n[Tool: %s]\n", m.Name)
        case *agent.ToolResult:
            if m.IsError {
                fmt.Printf("[Tool error]\n")
            }
        case *agent.Result:
            fmt.Println()
            fmt.Printf("\n--- Task Complete ---\n")
            fmt.Printf("Turns: %d, Cost: $%.4f\n", m.NumTurns, m.CostUSD)
        case *agent.Error:
            fmt.Printf("[Error: %v]\n", m.Err)
        }
    }

    if err := a.Err(); err != nil {
        fmt.Printf("Stream error: %v\n", err)
    }

    recorder.Finalize()

    // Step 3: Save session log
    if err := recorder.SaveToFile(sessionsDir); err != nil {
        fmt.Printf("Warning: failed to save session: %v\n", err)
    } else {
        fmt.Printf("Session saved to %s\n", sessionsDir)
    }

    // Step 4: Extract lessons
    fmt.Println("\nExtracting lessons...")
    lessons, err := ExtractLessons(ctx, recorder)
    if err != nil {
        return fmt.Errorf("lesson extraction failed: %w", err)
    }

    if len(lessons.Lessons) == 0 {
        fmt.Println("No lessons extracted from this session.")
        return nil
    }

    fmt.Printf("Extracted %d lessons:\n", len(lessons.Lessons))
    for i, l := range lessons.Lessons {
        fmt.Printf("  %d. [%s] %s\n", i+1, l.Context, l.Learning)
    }

    // Step 5: Generate skill file
    skillPath, err := SaveAsSkill(lessons, skillsDir)
    if err != nil {
        return fmt.Errorf("failed to save skill: %w", err)
    }

    fmt.Printf("\nSkill saved to: %s\n", skillPath)
    fmt.Println("This skill will be loaded in future sessions.")

    return nil
}

func extractFromFile(ctx context.Context, sessionFile, skillsDir string) error {
    // Load session from file
    data, err := os.ReadFile(sessionFile)
    if err != nil {
        return fmt.Errorf("failed to read session file: %w", err)
    }

    var recorder SessionRecorder
    if err := json.Unmarshal(data, &recorder); err != nil {
        return fmt.Errorf("failed to parse session file: %w", err)
    }

    fmt.Printf("Loaded session %s with %d messages\n",
        recorder.SessionID, len(recorder.Messages))

    // Extract lessons
    lessons, err := ExtractLessons(ctx, &recorder)
    if err != nil {
        return fmt.Errorf("extraction failed: %w", err)
    }

    if len(lessons.Lessons) == 0 {
        fmt.Println("No lessons extracted.")
        return nil
    }

    fmt.Printf("Extracted %d lessons:\n", len(lessons.Lessons))
    for i, l := range lessons.Lessons {
        fmt.Printf("  %d. [%s] %s\n", i+1, l.Context, l.Learning)
    }

    // Save skill
    skillPath, err := SaveAsSkill(lessons, skillsDir)
    if err != nil {
        return fmt.Errorf("failed to save skill: %w", err)
    }

    fmt.Printf("\nSkill saved to: %s\n", skillPath)
    return nil
}
```

### Missing Import

Add this import to main.go:

```go
import (
    "encoding/json"
    // ... other imports
)
```

## Running the Example

Build and run:

```bash
# Initialize the module
go mod init self-improve
go get github.com/wernerstrydom/claude-agent-sdk-go/agent

# Create the skills directory
mkdir -p skills sessions

# Run a task with lesson extraction
go run . "Create a Go function that validates email addresses"

# Later, extract lessons from an existing session
go run . -extract sessions/session-abc123.json
```

## Key Concepts

### Session Persistence

Session data is preserved in JSON files containing:

- All audit events with timestamps
- Tool calls and their inputs
- Tool results including errors
- Final result with cost and duration

This data enables offline analysis and lesson extraction without requiring a live agent.

### Skill File Format

Skills are markdown files that Claude loads into context. The format is flexible, but effective skills include:

- Clear section headings
- Specific guidance (not vague suggestions)
- Concrete examples
- Tags for mental organization

### Feedback Loops

The self-improvement pattern creates a feedback loop:

1. Agent runs task, makes mistakes
2. Mistakes are captured in session log
3. Extractor agent identifies what went wrong
4. Lessons become skills for future sessions
5. Future sessions avoid the same mistakes

This loop accumulates domain knowledge over time.

### Guardrails for Self-Modification

Self-improvement systems require safeguards:

1. **Human review** - Examine generated skills before deployment
2. **Version control** - Track skill changes over time
3. **Validation** - Filter low-quality lessons automatically
4. **Boundaries** - Skills cannot modify system prompts or safety hooks

## Cautions and Best Practices

### Review Generated Skills

Always review skills before adding them to production:

```bash
# Review pending skills
cat skills/learned-*.skill.md

# Remove problematic skills
rm skills/learned-bad123.skill.md
```

Skills can contain incorrect or harmful guidance. Human review is essential.

### Version Control

Track skills in version control:

```bash
git add skills/
git commit -m "Add lessons from session abc123"
```

This creates an audit trail and enables rollback if a skill causes problems.

### Monitor for Drift

Over time, accumulated skills can cause unexpected behavior. Monitor for:

- Increased API costs (verbose skills increase context size)
- Decreased task quality (conflicting guidance)
- Unexpected patterns (skills reinforcing bad habits)

Periodically review and prune the skills directory.

### Separate Skill Domains

Organize skills by domain to avoid cross-contamination:

```
skills/
├── go/
│   └── error-handling.skill.md
├── testing/
│   └── table-tests.skill.md
└── healthcare/
    └── patient-validation.skill.md
```

Load only relevant skill directories for each task:

```go
a, _ := agent.New(ctx,
    agent.SkillsDir("./skills/go"),
    agent.SkillsDir("./skills/testing"),
)
```

### Limit Skill Size

Large skills consume context window space. Keep skills focused:

- One skill per topic
- Maximum 500 words per skill
- Remove redundant examples
- Archive old skills periodically

## Summary

This tutorial covered:

- **Session capture** using audit handlers and PostToolUse hooks
- **Lesson extraction** using structured output agents
- **Skill generation** from extracted lessons
- **Skill loading** via SkillsDir option
- **Session management** with MaxTurns, Resume, and Fork
- **Context management** with PreCompact hooks
- **Complete implementation** of a self-improvement loop
- **Best practices** for safe self-modification

The self-improvement pattern enables agents to accumulate domain knowledge over time, making them more effective at
recurring tasks. The key is maintaining human oversight through skill review and version control.
