# API Reference

This document provides a complete reference for the public API of the Claude Agent SDK for Go.

## Package Overview

```go
import "github.com/wernerstrydom/claude-agent-sdk-go/agent"
```

The `agent` package provides programmatic automation of the Claude Code CLI. It wraps the CLI process, parses the JSON
streaming protocol, handles permission requests through hooks, and tracks session state.

## Types

### Agent

`Agent` represents a Claude Code session. It manages the lifecycle of a CLI process and provides methods to send prompts
and receive responses.

```go
type Agent struct {
    // contains filtered or unexported fields
}
```

#### Methods

##### New

```go
func New(ctx context.Context, opts ...Option) (*Agent, error)
```

Creates a new Agent with the given options.

**Parameters:**

- `ctx` - Context for the operation. Cancellation stops the agent process.
- `opts` - Configuration options (see [Options](#options) section).

**Returns:**

- `*Agent` - The created agent.
- `error` - A `*StartError` if the agent fails to start.

**Notes:**

- With stream-json input format, the CLI waits for the first message before outputting anything. The session ID is
  captured lazily when the first message is sent.
- The agent must be closed with `Close()` when no longer needed.

**Example:**

```go
a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
if err != nil {
    log.Fatal(err)
}
defer a.Close()
```

##### Run

```go
func (a *Agent) Run(ctx context.Context, prompt string, opts ...RunOption) (*Result, error)
```

Sends a prompt and waits for the result.

**Parameters:**

- `ctx` - Context for the operation.
- `prompt` - The text prompt to send to Claude.
- `opts` - Per-run options such as `Timeout` or `MaxTurnsRun`.

**Returns:**

- `*Result` - The final result containing response text, cost, and usage.
- `error` - An error if the operation fails, including `*MaxTurnsError` if turn limit is exceeded.

**Example:**

```go
result, err := a.Run(ctx, "What is 2+2?")
if err != nil {
    log.Fatal(err)
}
fmt.Println(result.ResultText)
```

##### Stream

```go
func (a *Agent) Stream(ctx context.Context, prompt string, opts ...RunOption) <-chan Message
```

Sends a prompt and returns a channel of messages. The channel closes when the result is received or an error occurs.

**Parameters:**

- `ctx` - Context for the operation. Cancellation closes the channel.
- `prompt` - The text prompt to send to Claude.
- `opts` - Per-run options.

**Returns:**

- `<-chan Message` - A read-only channel that emits messages as they arrive.

**Notes:**

- Call `Err()` after the channel closes to check for errors.
- Messages are emitted in order: `Text`, `Thinking`, `ToolUse`, `ToolResult`, and finally `Result`.

**Example:**

```go
for msg := range a.Stream(ctx, "Explain Go channels") {
    switch m := msg.(type) {
    case *agent.Text:
        fmt.Print(m.Text)
    case *agent.ToolUse:
        fmt.Printf("Tool: %s\n", m.Name)
    case *agent.Result:
        fmt.Printf("\nCost: $%.4f\n", m.CostUSD)
    }
}
if err := a.Err(); err != nil {
    log.Fatal(err)
}
```

##### RunWithSchema

```go
func (a *Agent) RunWithSchema(ctx context.Context, prompt string, ptr any, opts ...RunOption) (*Result, error)
```

Runs a prompt and unmarshals the structured response into the provided pointer. The agent must have been created with
`WithSchema` or `WithSchemaRaw` option.

**Parameters:**

- `ctx` - Context for the operation.
- `prompt` - The text prompt to send to Claude.
- `ptr` - A pointer to the struct to unmarshal the response into.
- `opts` - Per-run options.

**Returns:**

- `*Result` - The result containing the raw response text.
- `error` - An error if the operation or unmarshaling fails.

**Example:**

```go
type Answer struct {
    Value int `json:"value" desc:"The numeric answer"`
}

a, _ := agent.New(ctx, agent.WithSchema(Answer{}))
defer a.Close()

var answer Answer
result, err := a.RunWithSchema(ctx, "What is 2+2?", &answer)
if err != nil {
    log.Fatal(err)
}
fmt.Println(answer.Value) // 4
```

##### SessionID

```go
func (a *Agent) SessionID() string
```

Returns the session identifier. The session ID is captured lazily when the first message is processed.

**Returns:**

- `string` - The session ID, or empty string if no message has been processed yet.

##### Err

```go
func (a *Agent) Err() error
```

Returns any error that occurred during streaming. Call this after the `Stream()` channel closes.

**Returns:**

- `error` - The error, or nil if no error occurred.

##### Close

```go
func (a *Agent) Close() error
```

Terminates the agent and releases resources. This method should always be called when the agent is no longer needed.

**Returns:**

- `error` - An error if the process fails to terminate.

**Notes:**

- Safe to call multiple times; subsequent calls are no-ops.
- Calls `OnStop` hooks before releasing resources.

### RunStructured

```go
func RunStructured(ctx context.Context, prompt string, ptr any, opts ...Option) (*Result, error)
```

A convenience function that creates a one-shot agent for structured output. It generates a schema from the pointer's
type, sends the prompt, unmarshals the response, and closes the agent.

**Parameters:**

- `ctx` - Context for the operation.
- `prompt` - The text prompt to send to Claude.
- `ptr` - A pointer to the struct to unmarshal the response into.
- `opts` - Agent configuration options.

**Returns:**

- `*Result` - The result containing the raw response text.
- `error` - An error if any step fails.

**Notes:**

- Use this for single structured queries. For multiple queries with the same schema, create an agent with `WithSchema`
  for better performance.

**Example:**

```go
type Answer struct {
    Value int `json:"value" desc:"The numeric answer"`
}

var answer Answer
result, err := agent.RunStructured(ctx, "What is 2+2?", &answer)
if err != nil {
    log.Fatal(err)
}
fmt.Println(answer.Value)
```

---

## Options

Options configure an agent using the functional options pattern. Pass them to `New()`.

```go
type Option func(*config)
```

### Model

```go
func Model(name string) Option
```

Sets the Claude model to use.

**Parameters:**

- `name` - The model identifier (e.g., `"claude-sonnet-4-5"`, `"claude-opus-4-5"`).

**Default:** `"claude-sonnet-4-5"`

### WorkDir

```go
func WorkDir(path string) Option
```

Sets the working directory for the agent. File operations are relative to this directory.

**Parameters:**

- `path` - The directory path.

**Default:** `"."`

### CLIPath

```go
func CLIPath(path string) Option
```

Overrides the default Claude CLI location.

**Parameters:**

- `path` - The path to the Claude CLI executable.

### Tools

```go
func Tools(names ...string) Option
```

Sets the available tools for the agent.

**Parameters:**

- `names` - Tool names such as `"Bash"`, `"Read"`, `"Write"`, `"Edit"`, `"Glob"`, `"Grep"`, `"Task"`.

**Notes:**

- An empty slice disables all tools.

### AllowedTools

```go
func AllowedTools(patterns ...string) Option
```

Sets tool permission patterns. Patterns can include globs for fine-grained control.

**Parameters:**

- `patterns` - Permission patterns, e.g., `"Bash(git:*)"` allows only git commands in Bash.

### DisallowedTools

```go
func DisallowedTools(patterns ...string) Option
```

Sets tool denial patterns. Tools matching these patterns will be blocked.

**Parameters:**

- `patterns` - Denial patterns.

### PreToolUse

```go
func PreToolUse(hooks ...PreToolUseHook) Option
```

Adds hooks that are called before tool execution. Hooks are evaluated in order: first `Deny` wins, `Allow`
short-circuits.

**Parameters:**

- `hooks` - One or more `PreToolUseHook` functions.

**Example:**

```go
agent.PreToolUse(
    agent.DenyCommands("sudo", "rm -rf"),
    agent.AllowPaths("/sandbox"),
)
```

### PostToolUse

```go
func PostToolUse(hooks ...PostToolUseHook) Option
```

Adds hooks that are called after tool execution completes. These hooks receive the original tool call and the result.

**Parameters:**

- `hooks` - One or more `PostToolUseHook` functions.

**Notes:**

- PostToolUse hooks cannot modify or block the result. All hooks are called in order.

**Example:**

```go
agent.PostToolUse(func(tc *agent.ToolCall, tr *agent.ToolResultContext) agent.HookResult {
    log.Printf("Tool %s completed in %v", tc.Name, tr.Duration)
    return agent.HookResult{Decision: agent.Continue}
})
```

### OnStop

```go
func OnStop(hooks ...StopHook) Option
```

Adds hooks that are called when the agent session ends. Stop hooks receive information about the session including total
turns, cost, and the reason for stopping.

**Parameters:**

- `hooks` - One or more `StopHook` functions.

**Example:**

```go
agent.OnStop(func(e *agent.StopEvent) {
    log.Printf("Session %s ended: %s (%d turns, $%.4f)",
        e.SessionID, e.Reason, e.NumTurns, e.CostUSD)
})
```

### PreCompact

```go
func PreCompact(hooks ...PreCompactHook) Option
```

Adds hooks that are called before context window compaction. These hooks can archive the current transcript or extract
important data.

**Parameters:**

- `hooks` - One or more `PreCompactHook` functions.

### SubagentStop

```go
func SubagentStop(hooks ...SubagentStopHook) Option
```

Adds hooks that are called when a subagent completes execution.

**Parameters:**

- `hooks` - One or more `SubagentStopHook` functions.

### UserPromptSubmit

```go
func UserPromptSubmit(hooks ...UserPromptSubmitHook) Option
```

Adds hooks that are called before a prompt is sent to Claude. These hooks can modify the prompt or attach metadata.

**Parameters:**

- `hooks` - One or more `UserPromptSubmitHook` functions.

**Example:**

```go
agent.UserPromptSubmit(func(e *agent.PromptSubmitEvent) agent.PromptSubmitResult {
    return agent.PromptSubmitResult{
        UpdatedPrompt: e.Prompt + "\n[Context: Production environment]",
    }
})
```

### PermissionPrompt

```go
func PermissionPrompt(mode PermissionMode) Option
```

Sets how tool permissions are handled.

**Parameters:**

- `mode` - One of the `PermissionMode` constants.

### Env

```go
func Env(key, value string) Option
```

Sets an environment variable for the agent process. Multiple calls accumulate environment variables.

**Parameters:**

- `key` - The environment variable name.
- `value` - The environment variable value.

### AddDir

```go
func AddDir(paths ...string) Option
```

Adds directories the agent is allowed to access.

**Parameters:**

- `paths` - Directory paths.

### SettingSources

```go
func SettingSources(sources ...string) Option
```

Controls which settings files are loaded.

**Parameters:**

- `sources` - Valid values: `"user"`, `"project"`, `"local"`.

### MaxTurns

```go
func MaxTurns(n int) Option
```

Sets the maximum number of turns allowed. A turn is a complete assistant response. When exceeded, `Run()` returns
`*MaxTurnsError`.

**Parameters:**

- `n` - Maximum turns. A value of 0 means unlimited (default).

### Resume

```go
func Resume(sessionID string) Option
```

Continues a previous session by its ID.

**Parameters:**

- `sessionID` - The session ID from `SessionID()` or a previous result.

### Fork

```go
func Fork(sessionID string) Option
```

Branches from an existing session, creating a new session ID. The original session remains unchanged.

**Parameters:**

- `sessionID` - The session ID to branch from.

### WithSchema

```go
func WithSchema(example any) Option
```

Configures the agent for structured output using the provided type as a template. All responses will be formatted as
JSON matching the generated schema.

**Parameters:**

- `example` - A struct value (not pointer) used to generate the JSON Schema.

**Notes:**

- Use the `desc` struct tag to add descriptions to fields.
- Fields without `omitempty` and not pointers are marked as required.

**Example:**

```go
type Response struct {
    Answer string `json:"answer" desc:"The answer to the question"`
    Score  int    `json:"score" desc:"Confidence score 0-100"`
}

a, _ := agent.New(ctx, agent.WithSchema(Response{}))
```

### WithSchemaRaw

```go
func WithSchemaRaw(schema map[string]any) Option
```

Configures the agent with a custom JSON Schema. Use this for schemas that cannot be derived from Go types.

**Parameters:**

- `schema` - A map representing a JSON Schema.

**Example:**

```go
schema := map[string]any{
    "type": "object",
    "properties": map[string]any{
        "name": map[string]any{"type": "string"},
    },
}
a, _ := agent.New(ctx, agent.WithSchemaRaw(schema))
```

### Audit

```go
func Audit(h AuditHandler) Option
```

Adds a handler that receives audit events during agent execution. Multiple handlers can be added.

**Parameters:**

- `h` - An `AuditHandler` function.

**Example:**

```go
agent.Audit(func(e agent.AuditEvent) {
    log.Printf("[%s] %s: %v", e.Type, e.SessionID, e.Data)
})
```

### AuditToFile

```go
func AuditToFile(path string) Option
```

Configures the agent to write audit events to a file in JSONL format.

**Parameters:**

- `path` - The file path. The file is created or appended to.

**Example:**

```go
a, _ := agent.New(ctx, agent.AuditToFile("audit.jsonl"))
```

### CustomTool

```go
func CustomTool(tools ...Tool) Option
```

Registers custom in-process tools. Custom tools are executed directly by the SDK without going through the CLI.

**Parameters:**

- `tools` - One or more `Tool` implementations.

**Example:**

```go
calculator := agent.NewFuncTool(
    "calculator",
    "Evaluates arithmetic expressions",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "expression": map[string]any{"type": "string"},
        },
        "required": []string{"expression"},
    },
    func(ctx context.Context, input map[string]any) (any, error) {
        // Implementation
        return result, nil
    },
)
a, _ := agent.New(ctx, agent.CustomTool(calculator))
```

### MCPServer

```go
func MCPServer(name string, opts ...MCPOption) Option
```

Configures an MCP (Model Context Protocol) server.

**Parameters:**

- `name` - The server name used as a key.
- `opts` - MCP configuration options.

**Example:**

```go
agent.MCPServer("github",
    agent.MCPHTTP("https://api.githubcopilot.com/mcp"),
    agent.MCPHeader("Authorization", "Bearer token"),
)
```

### StrictMCPConfig

```go
func StrictMCPConfig(strict bool) Option
```

Ensures only SDK-configured MCP servers are used. When enabled, user and project MCP configurations are ignored.

**Parameters:**

- `strict` - Whether to enforce strict MCP configuration.

### Subagent

```go
func Subagent(name string, opts ...SubagentOption) Option
```

Configures a subagent that can be spawned by the Task tool.

**Parameters:**

- `name` - The subagent name.
- `opts` - Subagent configuration options.

**Example:**

```go
agent.Subagent("tester",
    agent.SubagentDescription("Runs tests and reports results"),
    agent.SubagentTools("Bash", "Read"),
    agent.SubagentModel("haiku"),
)
```

### Skill

```go
func Skill(name, content string) Option
```

Adds an inline skill with the given name and content. Skills are markdown instructions loaded into Claude's context.

**Parameters:**

- `name` - The skill name.
- `content` - The markdown content of the skill.

**Example:**

```go
goSkill := `# Go Development
- Use gofmt for formatting
- Handle all errors`
a, _ := agent.New(ctx, agent.Skill("go", goSkill))
```

### SkillsDir

```go
func SkillsDir(path string) Option
```

Loads skills from a directory. Skill files can be named `SKILL.md` (in a named directory) or `*.skill.md`.

**Parameters:**

- `path` - The directory path.

### SystemPromptPreset

```go
func SystemPromptPreset(name string) Option
```

Sets a preset system prompt by name.

**Parameters:**

- `name` - The preset name.

### SystemPromptAppend

```go
func SystemPromptAppend(text string) Option
```

Adds text to the end of the system prompt.

**Parameters:**

- `text` - The text to append.

**Example:**

```go
agent.SystemPromptAppend("Always explain your reasoning.")
```

---

## Run Options

Run options configure a single `Run()` or `Stream()` call.

```go
type RunOption func(*runConfig)
```

### Timeout

```go
func Timeout(d time.Duration) RunOption
```

Sets a timeout for the Run() call.

**Parameters:**

- `d` - The timeout duration.

### MaxTurnsRun

```go
func MaxTurnsRun(n int) RunOption
```

Overrides the agent-level MaxTurns for this Run() call.

**Parameters:**

- `n` - Maximum turns for this run.

---

## Message Types

All message types implement the `Message` interface.

```go
type Message interface {
    message() // unexported marker method
}
```

### MessageMeta

Common metadata embedded in all message types.

```go
type MessageMeta struct {
    Timestamp  time.Time
    SessionID  string
    Turn       int
    Sequence   int
    ParentID   string
    SubagentID string
}
```

### Text

Contains assistant text output.

```go
type Text struct {
    MessageMeta
    Text string
}
```

### Thinking

Contains the assistant's thinking process (extended thinking).

```go
type Thinking struct {
    MessageMeta
    Thinking  string
    Signature string
}
```

### ToolUse

Represents a tool invocation by the assistant.

```go
type ToolUse struct {
    MessageMeta
    ID    string
    Name  string
    Input map[string]any
}
```

**Fields:**

- `ID` - Unique identifier for this tool invocation.
- `Name` - The name of the tool being invoked.
- `Input` - The input parameters for the tool.

### ToolResult

Contains the result of a tool execution.

```go
type ToolResult struct {
    MessageMeta
    ToolUseID string
    Content   any
    IsError   bool
    Duration  time.Duration
}
```

**Fields:**

- `ToolUseID` - Links to the corresponding `ToolUse.ID`.
- `Content` - The result returned by the tool.
- `IsError` - Whether the tool execution resulted in an error.
- `Duration` - How long the tool took to execute.

### Result

The final result of an agent run.

```go
type Result struct {
    MessageMeta
    DurationTotal time.Duration
    DurationAPI   time.Duration
    NumTurns      int
    CostUSD       float64
    Usage         Usage
    ResultText    string
    IsError       bool
}
```

**Fields:**

- `DurationTotal` - Total time for the run.
- `DurationAPI` - Time spent in API calls.
- `NumTurns` - Number of turns in this run.
- `CostUSD` - Cost of this run in USD.
- `Usage` - Token usage details.
- `ResultText` - The final text response.
- `IsError` - Whether the result represents an error.

### Error

Represents an error during agent execution.

```go
type Error struct {
    MessageMeta
    Err error
}
```

### Usage

Contains token usage information.

```go
type Usage struct {
    InputTokens  int
    OutputTokens int
    CacheRead    int
    CacheWrite   int
}
```

### ToolInfo

Describes a tool available to the agent.

```go
type ToolInfo struct {
    Name        string
    Description string
}
```

### MCPStatus

Describes the status of an MCP server.

```go
type MCPStatus struct {
    Name   string
    Status string
}
```

---

## Hooks

Hooks intercept operations at defined points in the execution flow.

### Decision

```go
type Decision int

const (
    Continue Decision = iota
    Allow
    Deny
)
```

**Values:**

- `Continue` - Passes evaluation to the next hook in the chain.
- `Allow` - Approves the operation and skips remaining hooks.
- `Deny` - Blocks the operation.

### ToolCall

```go
type ToolCall struct {
    Name  string
    Input map[string]any
}
```

Represents a tool invocation that can be intercepted by hooks.

### HookResult

```go
type HookResult struct {
    Decision     Decision
    Reason       string
    UpdatedInput map[string]any
}
```

**Fields:**

- `Decision` - Whether to allow, deny, or continue to next hook.
- `Reason` - Feedback to Claude when denying.
- `UpdatedInput` - Optionally modifies the tool inputs.

### PreToolUseHook

```go
type PreToolUseHook func(*ToolCall) HookResult
```

Called before a tool is executed. It can allow, deny, or modify the tool call.

### PostToolUseHook

```go
type PostToolUseHook func(*ToolCall, *ToolResultContext) HookResult
```

Called after a tool has executed. It receives the original tool call and the result context.

### ToolResultContext

```go
type ToolResultContext struct {
    ToolUseID string
    Content   any
    IsError   bool
    Duration  time.Duration
}
```

Provides context about a completed tool execution.

### StopHook

```go
type StopHook func(*StopEvent)
```

Called when an agent session ends.

### StopEvent

```go
type StopEvent struct {
    SessionID string
    Reason    StopReason
    NumTurns  int
    CostUSD   float64
}
```

### StopReason

```go
type StopReason string

const (
    StopCompleted   StopReason = "completed"
    StopMaxTurns    StopReason = "max_turns"
    StopInterrupted StopReason = "interrupted"
    StopError       StopReason = "error"
)
```

### PreCompactHook

```go
type PreCompactHook func(*PreCompactEvent) PreCompactResult
```

Called before context window compaction.

### PreCompactEvent

```go
type PreCompactEvent struct {
    SessionID      string
    Trigger        string
    TranscriptPath string
    TokenCount     int
}
```

### PreCompactResult

```go
type PreCompactResult struct {
    Archive   bool
    ArchiveTo string
    Extract   any
}
```

### SubagentStopHook

```go
type SubagentStopHook func(*SubagentStopEvent)
```

Called when a subagent completes execution.

### SubagentStopEvent

```go
type SubagentStopEvent struct {
    SessionID       string
    SubagentID      string
    SubagentType    string
    ParentToolUseID string
    NumTurns        int
    CostUSD         float64
}
```

### UserPromptSubmitHook

```go
type UserPromptSubmitHook func(*PromptSubmitEvent) PromptSubmitResult
```

Called before a prompt is sent to Claude.

### PromptSubmitEvent

```go
type PromptSubmitEvent struct {
    Prompt    string
    SessionID string
    Turn      int
}
```

### PromptSubmitResult

```go
type PromptSubmitResult struct {
    UpdatedPrompt string
    Metadata      any
}
```

---

## Built-in Hooks

The SDK provides several pre-built hooks for common use cases.

### DenyCommands

```go
func DenyCommands(patterns ...string) PreToolUseHook
```

Returns a hook that blocks Bash commands matching any pattern using substring containment.

**Parameters:**

- `patterns` - Command patterns to block.

**Example:**

```go
agent.PreToolUse(agent.DenyCommands("sudo", "curl", "wget"))
```

### RequireCommand

```go
func RequireCommand(use string, insteadOf ...string) PreToolUseHook
```

Returns a hook that blocks commands matching any of the `insteadOf` patterns and suggests using the preferred command
instead.

**Parameters:**

- `use` - The preferred command.
- `insteadOf` - Commands to block.

**Example:**

```go
agent.PreToolUse(agent.RequireCommand("make", "go build", "go test"))
```

### AllowPaths

```go
func AllowPaths(paths ...string) PreToolUseHook
```

Returns a hook that only allows file operations on paths that start with one of the allowed prefixes. All other paths
are denied.

**Parameters:**

- `paths` - Allowed path prefixes.

**Example:**

```go
agent.PreToolUse(agent.AllowPaths("/sandbox", "/tmp"))
```

### DenyPaths

```go
func DenyPaths(paths ...string) PreToolUseHook
```

Returns a hook that blocks file operations on paths that start with any of the denied prefixes.

**Parameters:**

- `paths` - Denied path prefixes.

**Example:**

```go
agent.PreToolUse(agent.DenyPaths("/etc", "/usr", "~/.ssh"))
```

### RedirectPath

```go
func RedirectPath(from, to string) PreToolUseHook
```

Returns a hook that rewrites file paths. If a path starts with `from`, it is rewritten to start with `to`.

**Parameters:**

- `from` - The path prefix to match.
- `to` - The replacement prefix.

**Example:**

```go
agent.PreToolUse(agent.RedirectPath("/tmp", "/sandbox/tmp"))
```

A path like `/tmp/foo.txt` becomes `/sandbox/tmp/foo.txt`.

---

## Custom Tools

### Tool Interface

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(ctx context.Context, input map[string]any) (any, error)
}
```

**Methods:**

- `Name()` - Returns the unique identifier for this tool.
- `Description()` - Returns a human-readable description.
- `InputSchema()` - Returns the JSON Schema for input parameters.
- `Execute()` - Runs the tool with the given input.

### FuncTool

```go
type FuncTool struct {
    // contains filtered or unexported fields
}
```

A function-based implementation of `Tool`.

### NewFuncTool

```go
func NewFuncTool(
    name, description string,
    inputSchema map[string]any,
    fn func(context.Context, map[string]any) (any, error),
) *FuncTool
```

Creates a new Tool from a function.

**Parameters:**

- `name` - The tool name.
- `description` - A description of what the tool does.
- `inputSchema` - JSON Schema for the input parameters.
- `fn` - The function that implements the tool.

**Example:**

```go
tool := agent.NewFuncTool(
    "calculator",
    "Performs arithmetic calculations",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "expression": map[string]any{
                "type":        "string",
                "description": "The arithmetic expression to evaluate",
            },
        },
        "required": []string{"expression"},
    },
    func(ctx context.Context, input map[string]any) (any, error) {
        expr := input["expression"].(string)
        // Evaluate expression
        return result, nil
    },
)
```

---

## MCP Configuration

### MCPConfig

```go
type MCPConfig struct {
    Name      string
    Transport string            // "stdio", "sse", or "http"
    Command   string            // Executable (stdio only)
    Args      []string          // Command arguments (stdio only)
    URL       string            // Server URL (sse/http only)
    Headers   map[string]string // Request headers (sse/http only)
    Env       map[string]string // Environment variables (stdio only)
}
```

### MCPOption

```go
type MCPOption func(*MCPConfig)
```

### MCPCommand

```go
func MCPCommand(cmd string) MCPOption
```

Sets the transport to "stdio" and specifies the command to run.

### MCPArgs

```go
func MCPArgs(args ...string) MCPOption
```

Sets the command arguments for a stdio transport.

### MCPSSE

```go
func MCPSSE(url string) MCPOption
```

Sets the transport to "sse" (Server-Sent Events) and specifies the URL.

### MCPHTTP

```go
func MCPHTTP(url string) MCPOption
```

Sets the transport to "http" and specifies the URL.

### MCPHeader

```go
func MCPHeader(key, val string) MCPOption
```

Adds a header to the MCP server configuration. Used for sse/http transports.

### MCPEnv

```go
func MCPEnv(key, val string) MCPOption
```

Adds an environment variable to the MCP server configuration. Used for stdio transport.

---

## Subagent Configuration

### SubagentConfig

```go
type SubagentConfig struct {
    Name        string
    Description string
    Prompt      string
    Tools       []string
    Model       string
}
```

### SubagentOption

```go
type SubagentOption func(*SubagentConfig)
```

### SubagentDescription

```go
func SubagentDescription(desc string) SubagentOption
```

Sets the description for the subagent.

### SubagentPrompt

```go
func SubagentPrompt(prompt string) SubagentOption
```

Sets the system prompt or instructions for the subagent.

### SubagentTools

```go
func SubagentTools(tools ...string) SubagentOption
```

Sets the tools available to the subagent.

### SubagentModel

```go
func SubagentModel(model string) SubagentOption
```

Sets the model for the subagent. Common values: `"haiku"` for fast/cheap tasks, `"sonnet"` for balanced tasks.

---

## Audit System

### AuditEvent

```go
type AuditEvent struct {
    Time      time.Time `json:"time"`
    SessionID string    `json:"session_id"`
    Type      string    `json:"type"`
    Data      any       `json:"data,omitempty"`
}
```

**Event Types:**

- `session.start` - Session begins
- `session.init` - Session initialized with tools
- `session.end` - Session terminates
- `message.prompt` - Prompt submitted
- `message.text` - Text response
- `message.thinking` - Thinking content
- `message.tool_use` - Tool invocation
- `message.tool_result` - Tool result
- `message.result` - Final result
- `hook.pre_tool_use` - PreToolUse hook evaluated
- `hook.post_tool_use` - PostToolUse hook evaluated
- `hook.stop` - Stop hook called
- `hook.pre_compact` - PreCompact hook called
- `hook.subagent_stop` - SubagentStop hook called
- `hook.user_prompt_submit` - UserPromptSubmit hook called
- `error` - Error occurred

### AuditHandler

```go
type AuditHandler func(AuditEvent)
```

A function that receives audit events.

### AuditWriterHandler

```go
func AuditWriterHandler(w io.Writer) AuditHandler
```

Creates an AuditHandler that writes JSONL to the given writer.

### AuditFileHandler

```go
func AuditFileHandler(path string) (AuditHandler, func() error, error)
```

Creates an AuditHandler that writes JSONL to a file. Returns the handler and a cleanup function.

---

## Permission Modes

```go
type PermissionMode string

const (
    PermissionDefault     PermissionMode = "default"
    PermissionAcceptEdits PermissionMode = "acceptEdits"
    PermissionBypass      PermissionMode = "bypassPermissions"
    PermissionDontAsk     PermissionMode = "dontAsk"
    PermissionPlan        PermissionMode = "plan"
)
```

**Values:**

- `PermissionDefault` - Uses standard permission checks.
- `PermissionAcceptEdits` - Automatically accepts file edit operations.
- `PermissionBypass` - Bypasses all permission checks (use with caution).
- `PermissionDontAsk` - Skips permission prompts.
- `PermissionPlan` - Uses plan mode for permissions.

---

## Error Types

### StartError

```go
type StartError struct {
    Reason string
    Cause  error
}
```

Indicates the agent failed to start.

### ProcessError

```go
type ProcessError struct {
    ExitCode int
    Stderr   string
}
```

Indicates the Claude Code process exited with an error.

### MaxTurnsError

```go
type MaxTurnsError struct {
    Turns      int
    MaxAllowed int
    SessionID  string
}
```

Indicates the agent exceeded the maximum number of turns.

### HookInterruptError

```go
type HookInterruptError struct {
    Hook   string
    Tool   string
    Reason string
}
```

Indicates a hook blocked execution.

### TaskError

```go
type TaskError struct {
    SessionID string
    Message   string
}
```

Indicates a task-level error.

### SchemaError

```go
type SchemaError struct {
    Type   string
    Reason string
    Cause  error
}
```

Indicates a JSON Schema generation or unmarshaling error.

### ToolError

```go
type ToolError struct {
    ToolName string
    Message  string
    Cause    error
}
```

Represents an error during custom tool execution.
