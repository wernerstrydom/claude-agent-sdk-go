# Claude Agent SDK for Go — Implementation Blueprint

## Design Philosophy

### Value-First Development
Every step must demonstrate tangible progress. We don't build infrastructure for infrastructure's sake—we build working software that does something useful as early as possible.

### OODA Loop Per Step
Each prompt instructs the implementing agent to:
1. **Observe** — Read the requirements, examine existing code
2. **Orient** — Understand current state, identify gaps
3. **Decide** — Plan specific changes
4. **Act** — Implement, test, validate

### Step Sizing Criteria
- **Small enough**: Single responsibility, testable in isolation
- **Large enough**: Delivers measurable progress
- **Integrated**: No orphaned code—everything wires into the whole

### Testing Strategy
- Tests run at every step
- Regressions caught immediately
- Examples serve as integration tests

---

## Implementation Roadmap

| Step | Deliverable | Value Demonstration |
|------|-------------|---------------------|
| 1 | Project skeleton + types | Compiles, tests pass |
| 2 | Process management + parser | Can spawn CLI, parse output |
| 3 | Agent with Run() | **First working example** |
| 4 | Stream() method | Streaming example |
| 5 | Hook foundation + PreToolUse | Hooks integrated |
| 6 | Higher-order hook functions | **Hooks example** |
| 7 | Extended options | Tools, Env, Permissions |
| 8 | Limits and sessions | MaxTurns, Resume, Fork |
| 9 | Structured output | **Structured output example** |
| 10 | Audit system | **Audit example** |
| 11 | Lifecycle hooks | Stop, PostToolUse, PreCompact |
| 12 | UserPromptSubmit | Prompt interception |
| 13 | Custom tools | **Tools example** |
| 14 | MCP servers | External tool support |
| 15 | Subagents + Skills | **Complete example** |

---

## Step 1: Project Skeleton and Core Types

### Context
Starting from nothing. Establish the foundation that everything builds upon.

### Deliverables
- `go.mod` initialized
- Core message types with `MessageMeta`
- Error types with `errors.As` support
- Option pattern foundation
- All tests passing

### Prompt

```text
You are implementing a Go SDK for automating Claude Code CLI. 

Module: github.com/wernerstrydom/claude-agent-sdk-go
Package: agent

## OODA

### Observe
Read this specification carefully. You're building:
- A wrapper around Claude Code CLI
- Process spawns, JSON over stdio
- Functional options pattern throughout

### Orient  
This is step 1 of 15. You're creating the type foundation. Later steps add:
- Process management (step 2)
- Agent implementation (step 3)
- Hooks, options, features (steps 4-15)

### Decide
Create these files:
1. go.mod
2. agent/message.go - message types
3. agent/errors.go - error types  
4. agent/options.go - option pattern
5. Tests for each

### Act

Create the project:

1. **go.mod** for github.com/wernerstrydom/claude-agent-sdk-go, Go 1.21+

2. **agent/message.go**:

   MessageMeta struct:
   - Timestamp time.Time
   - SessionID string
   - Turn int
   - Sequence int
   - ParentID string
   - SubagentID string

   Message interface with unexported marker method.

   Concrete types (all embed MessageMeta):
   - SystemInit: TranscriptPath, Tools []ToolInfo, MCPServers []MCPStatus
   - Text: Text string
   - Thinking: Thinking, Signature string
   - ToolUse: ID, Name string, Input map[string]any
   - ToolResult: ToolUseID string, Content any, IsError bool, Duration time.Duration
   - Result: DurationTotal, DurationAPI time.Duration, NumTurns int, CostUSD float64, Usage Usage, ResultText string, IsError bool
   - Error: Err error

   Supporting types:
   - ToolInfo: Name, Description string
   - MCPStatus: Name, Status string
   - Usage: InputTokens, OutputTokens, CacheRead, CacheWrite int

3. **agent/errors.go**:

   All implement error interface. Use "agent: " prefix in messages.

   - StartError: Reason string, Cause error (implements Unwrap)
   - ProcessError: ExitCode int, Stderr string
   - MaxTurnsError: Turns, MaxAllowed int, SessionID string
   - HookInterruptError: Hook, Tool, Reason string
   - TaskError: SessionID, Message string

4. **agent/options.go**:

   config struct (unexported):
   - model string
   - workDir string

   Option type: func(*config)

   Functions:
   - Model(name string) Option
   - WorkDir(path string) Option
   - newConfig(opts ...Option) *config (applies defaults, then options)

5. **Tests**:
   - agent/message_test.go: All types implement Message, MessageMeta accessible
   - agent/errors_test.go: All implement error, errors.As works, Unwrap chains
   - agent/options_test.go: Options compose, defaults applied

Run: go test ./...

All tests must pass. No compilation errors.
```

---

## Step 2: Process Management and JSON Parser

### Context
Step 1 complete. Types exist. Now add the ability to spawn Claude Code CLI and parse its JSON output.

### Deliverables
- Process spawning with stdin/stdout/stderr pipes
- JSON line parser that produces typed messages
- Integration between process and parser
- Tests for both components

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/message.go — message types with MessageMeta
- agent/errors.go — StartError, ProcessError, etc.
- agent/options.go — Option pattern with Model, WorkDir

Run: go test ./... (should pass)

### Orient
This is step 2 of 15. You're adding:
- Process management to spawn Claude Code CLI
- JSON parser to convert CLI output to typed messages

Next step (3) will create the Agent that uses these.

### Decide
Create:
1. agent/process.go — process lifecycle
2. agent/parser.go — JSON line parsing
3. Tests for both

### Act

1. **agent/process.go**:

   process struct (unexported):
   - cmd *exec.Cmd
   - stdin io.WriteCloser
   - stdout io.ReadCloser
   - stderr bytes.Buffer
   - done chan struct{}
   - exitErr error
   - mu sync.Mutex

   Functions:

   findCLI() (string, error):
   - Check PATH for "claude"
   - Check common locations: ~/.npm-global/bin/claude, /usr/local/bin/claude
   - Return StartError if not found

   startProcess(ctx context.Context, cfg *config) (*process, error):
   - Find CLI
   - Build args: --output-format json, --model (from config)
   - Set working directory from config
   - Create stdin/stdout/stderr pipes
   - Start process
   - Launch goroutine to wait for exit, store result
   - Return process or StartError

   (p *process) write(data []byte) error — write to stdin
   (p *process) reader() io.Reader — return stdout
   (p *process) close() error:
   - Close stdin
   - Wait with 5s timeout
   - Kill if needed
   - Return ProcessError if non-zero exit

   (p *process) wait() error — block until done

2. **agent/parser.go**:

   parser struct (unexported):
   - scanner *bufio.Scanner
   - sessionID string
   - turn int
   - sequence int

   newParser(r io.Reader) *parser

   (p *parser) next() (Message, error):
   - Read line from scanner
   - Return io.EOF when done
   - Parse JSON into rawMessage struct
   - Discriminate on "type" field:
     - "system" + subtype "init" → SystemInit
     - "assistant" → parse content blocks for Text, Thinking, ToolUse
     - "result" → Result
   - Populate MessageMeta (use time.Now(), sessionID, turn, sequence)
   - Increment sequence; detect new turn on certain message types
   - Return typed Message or error

   rawMessage struct for initial parse:
   - Type, Subtype string
   - Content json.RawMessage
   - (add fields as needed for Claude Code's actual format)

3. **agent/options.go** addition:
   - CLIPath(path string) Option — override CLI location

4. **Tests**:

   agent/process_test.go:
   - Test findCLI returns error when not found
   - Test startProcess builds correct command args
   - Test write/close lifecycle
   - Use exec.Command with "echo" or similar for unit tests

   agent/parser_test.go:
   - Create test fixtures as JSON strings representing Claude Code output
   - Test parsing SystemInit message
   - Test parsing Text message
   - Test parsing ToolUse message
   - Test parsing Result message
   - Test malformed JSON returns error (not panic)
   - Test unknown message types handled gracefully
   - Test MessageMeta population

Run: go test ./...

All tests must pass. Parser tests are critical—use realistic fixtures.
```

---

## Step 3: Basic Agent with Run()

### Context
Steps 1-2 complete. Types, process management, and parser exist. Now create the Agent that ties it all together.

### Deliverables
- Agent struct with New(), Run(), Close()
- Message channel bridge
- **First executable example**
- Integration tests

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/message.go — all message types
- agent/errors.go — error types
- agent/options.go — Option pattern
- agent/process.go — process spawning
- agent/parser.go — JSON parser

Run: go test ./... (should pass)

### Orient
This is step 3 of 15. This is a MAJOR MILESTONE—first working agent.

You're creating:
- Agent struct that owns a process
- Bridge that pumps messages from process to channel
- Run() method for blocking execution
- First executable example

### Decide
Create:
1. agent/bridge.go — message pump
2. agent/agent.go — Agent type
3. agent/agent_test.go — tests
4. examples/hello/main.go — FIRST EXAMPLE

### Act

1. **agent/bridge.go**:

   bridge struct (unexported):
   - parser *parser
   - messages chan Message
   - err error
   - done chan struct{}
   - closeOnce sync.Once

   newBridge(r io.Reader) *bridge:
   - Create parser
   - Create buffered channel (size 32)
   - Create done channel
   - Start pump goroutine
   - Return bridge

   (b *bridge) pump():
   - Loop: call parser.next()
   - On message: send to channel (select with done)
   - On io.EOF: break loop
   - On error: store in b.err, break loop
   - Close channel when done

   (b *bridge) recv() <-chan Message — return read-only channel
   (b *bridge) error() error — return stored error
   (b *bridge) close():
   - Use closeOnce
   - Close done channel (signals pump to stop)

2. **agent/agent.go**:

   Agent struct:
   - cfg *config
   - proc *process
   - bridge *bridge
   - sessionID string
   - mu sync.Mutex
   - closed bool

   New(ctx context.Context, opts ...Option) (*Agent, error):
   - Create config from options
   - Start process
   - Create bridge from process.reader()
   - Wait for first message (should be SystemInit)
   - Extract sessionID from SystemInit
   - Return Agent or error (with cleanup on failure)

   (a *Agent) Run(ctx context.Context, prompt string, opts ...RunOption) (*Result, error):
   - Lock mutex
   - Write prompt to process as JSON: {"type": "user", "content": prompt}
   - Collect messages until Result received
   - Handle context cancellation
   - Return Result or error

   (a *Agent) SessionID() string
   
   (a *Agent) Close() error:
   - Set closed flag
   - Close bridge
   - Close process
   - Return any error

   RunOption type (empty for now):
   type RunOption func(*runConfig)
   type runConfig struct{}

3. **agent/agent_test.go**:
   - Test New() returns error with invalid CLI path
   - Test Close() is idempotent
   - Test Run() returns Result (may need integration test flag)
   - Test context cancellation stops Run()

4. **examples/hello/main.go**:

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
       ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
       defer cancel()

       a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
       if err != nil {
           log.Fatal(err)
       }
       defer a.Close()

       result, err := a.Run(ctx, "What is 2 + 2? Reply with just the number.")
       if err != nil {
           log.Fatal(err)
       }

       fmt.Printf("Response: %s\n", result.ResultText)
       fmt.Printf("Cost: $%.4f\n", result.CostUSD)
       fmt.Printf("Session: %s\n", a.SessionID())
   }
   ```

5. **Makefile**:
   ```makefile
   .PHONY: test example-hello

   test:
   	go test -v ./...

   example-hello:
   	go run ./examples/hello
   ```

Run: go test ./...
Run: make example-hello (requires ANTHROPIC_API_KEY)

This is your first working agent. Celebrate, then continue.
```

---

## Step 4: Stream() Method

### Context
Step 3 complete. Agent works with blocking Run(). Now add streaming for real-time message observation.

### Deliverables
- Stream() method returning message channel
- Err() method for post-stream errors
- Run() refactored to use Stream()
- Streaming example

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/agent.go — Agent with New(), Run(), Close()
- agent/bridge.go — message channel bridge
- examples/hello/main.go — working example

Run: go test ./... (should pass)
Run: make example-hello (should work)

### Orient
This is step 4 of 15. Adding streaming capability.

The pattern: Stream() returns a channel, Run() wraps Stream().
This lets users choose blocking or streaming based on their needs.

### Decide
Modify:
1. agent/agent.go — add Stream(), Err(), refactor Run()
2. agent/agent_test.go — add streaming tests
3. Create examples/stream/main.go

### Act

1. **agent/agent.go** modifications:

   (a *Agent) Stream(ctx context.Context, prompt string, opts ...RunOption) <-chan Message:
   - Lock mutex (prevent concurrent streams)
   - Write prompt to process
   - Return bridge.recv() channel
   - Handle context cancellation (close on cancel)

   (a *Agent) Err() error:
   - Return bridge.error()
   - Call after channel closes to check for errors

   Refactor Run() to use Stream():
   ```go
   func (a *Agent) Run(ctx context.Context, prompt string, opts ...RunOption) (*Result, error) {
       var result *Result
       for msg := range a.Stream(ctx, prompt, opts...) {
           switch m := msg.(type) {
           case *Result:
               result = m
           case *Error:
               return nil, m.Err
           }
       }
       if err := a.Err(); err != nil {
           return nil, err
       }
       if result == nil {
           return nil, &TaskError{SessionID: a.sessionID, Message: "no result received"}
       }
       return result, nil
   }
   ```

2. **agent/agent_test.go** additions:
   - Test Stream() returns messages
   - Test all message types appear in stream
   - Test Err() returns nil on success
   - Test Err() returns error on failure
   - Test Run() still works (regression)

3. **examples/stream/main.go**:

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
       ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
       defer cancel()

       a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
       if err != nil {
           log.Fatal(err)
       }
       defer a.Close()

       fmt.Println("Streaming response:\n")
       
       for msg := range a.Stream(ctx, "Explain what Go channels are in 3 sentences.") {
           switch m := msg.(type) {
           case *agent.Text:
               fmt.Print(m.Text)
           case *agent.Thinking:
               fmt.Printf("[thinking...]\n")
           case *agent.ToolUse:
               fmt.Printf("\n[tool: %s]\n", m.Name)
           case *agent.Result:
               fmt.Printf("\n\n---\nDone: %d turns, $%.4f\n", m.NumTurns, m.CostUSD)
           case *agent.Error:
               log.Printf("Error: %v\n", m.Err)
           }
       }

       if err := a.Err(); err != nil {
           log.Fatal(err)
       }
   }
   ```

4. **Makefile** addition:
   ```makefile
   example-stream:
   	go run ./examples/stream
   ```

Run: go test ./...
Run: make example-hello (regression check)
Run: make example-stream
```

---

## Step 5: Hook Foundation and PreToolUse

### Context
Steps 1-4 complete. Working agent with Run() and Stream(). Now add the hook system for tool control.

### Deliverables
- Hook types (Decision, HookResult, ToolCall)
- Hook chain evaluation
- PreToolUse option and integration
- Control request/response handling

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/agent.go — Agent with Run(), Stream(), Err()
- agent/bridge.go — message pump
- agent/process.go — process management
- Two working examples

Run: go test ./... (should pass)

### Orient
This is step 5 of 15. Adding the hook system.

Philosophy: Hooks provide deterministic enforcement. PreToolUse hooks can Allow, Deny, or modify tool calls before execution.

Chain evaluation: First Deny wins, Allow short-circuits, Continue passes to next.

### Decide
Create:
1. agent/hooks.go — types and chain evaluation
2. agent/control.go — control request handling
3. Modify agent/options.go — PreToolUse option
4. Modify agent/parser.go — detect control requests
5. Modify agent/agent.go — integrate hooks
6. Tests for hooks

### Act

1. **agent/hooks.go**:

   ```go
   type Decision int

   const (
       Continue Decision = iota  // pass to next hook
       Allow                     // approve, skip remaining
       Deny                      // block execution
   )

   type ToolCall struct {
       Name  string
       Input map[string]any
   }

   type HookResult struct {
       Decision     Decision
       Reason       string            // feedback to Claude
       UpdatedInput map[string]any    // optional: modify inputs
   }

   type PreToolUseHook func(*ToolCall) HookResult
   ```

   hookChain struct (unexported):
   - hooks []PreToolUseHook

   (c *hookChain) evaluate(tc *ToolCall) HookResult:
   - Iterate through hooks
   - First Deny: return immediately with reason
   - Allow: return immediately
   - Continue: proceed to next hook
   - If all Continue: return HookResult{Decision: Allow}
   - Accumulate UpdatedInput changes through chain

2. **agent/control.go**:

   ControlRequest struct:
   - RequestID string
   - Type string
   - Tool *ToolCall

   controlResponse struct:
   - RequestID string
   - Decision string ("allow", "deny")
   - Reason string
   - UpdatedInput map[string]any

   (a *Agent) handleControlRequest(req *ControlRequest) error:
   - Evaluate hook chain
   - Build response based on HookResult
   - Write response to process as JSON

3. **agent/options.go** additions:

   Add to config:
   - preToolUseHooks []PreToolUseHook

   PreToolUse(hooks ...PreToolUseHook) Option:
   - Appends hooks to config

4. **agent/parser.go** modifications:

   Detect control requests in message stream:
   - Claude Code sends permission requests as special JSON
   - Parse into ControlRequest struct
   - Add ControlRequest message type (or handle inline)

5. **agent/agent.go** modifications:

   In message processing:
   - Detect ControlRequest messages
   - Call handleControlRequest
   - Don't pass to user channel (internal handling)

   Create hook chain in New():
   - Build chain from config.preToolUseHooks

6. **agent/hooks_test.go**:
   - Empty chain returns Allow
   - Single Deny returns Deny with reason
   - Single Allow returns Allow
   - Single Continue falls through to Allow
   - Deny short-circuits (later hooks not called)
   - Allow short-circuits
   - UpdatedInput passed through chain
   - Multiple hooks compose correctly

Run: go test ./...
Run: make example-hello (regression)
Run: make example-stream (regression)

Hooks are now integrated but we have no convenient hook functions yet. Next step adds those.
```

---

## Step 6: Higher-Order Hook Functions

### Context
Step 5 complete. Hook system works with raw functions. Now add ergonomic higher-order functions.

### Deliverables
- DenyCommands()
- RequireCommand()
- AllowPaths()
- DenyPaths()
- RedirectPath()
- **Hooks example**

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/hooks.go — PreToolUseHook, HookResult, hookChain
- agent/control.go — control request handling
- agent/options.go — PreToolUse() option

Run: go test ./... (should pass)

### Orient
This is step 6 of 15. Adding higher-order hook functions for common patterns.

These functions return PreToolUseHook, making composition easy:
```go
agent.PreToolUse(
    agent.DenyCommands("rm -rf", "sudo"),
    agent.AllowPaths("/sandbox"),
    customHook,
)
```

### Decide
Create:
1. agent/hooks_commands.go — command control functions
2. agent/hooks_paths.go — path control functions
3. agent/hooks_commands_test.go
4. agent/hooks_paths_test.go
5. examples/hooks/main.go

### Act

1. **agent/hooks_commands.go**:

   DenyCommands(patterns ...string) PreToolUseHook:
   - Return hook that checks if tool is "Bash"
   - Extract "command" from Input
   - If command contains any pattern: Deny with reason
   - Otherwise: Continue

   RequireCommand(use string, insteadOf ...string) PreToolUseHook:
   - Return hook that checks if tool is "Bash"
   - If command matches any insteadOf pattern:
     - Deny with reason: "use <use> instead of <matched>"
   - Otherwise: Continue

2. **agent/hooks_paths.go**:

   pathTools variable: []string{"Read", "Write", "Edit", "MultiEdit"}

   AllowPaths(paths ...string) PreToolUseHook:
   - Return hook that checks if tool is in pathTools
   - Extract "file_path" or "path" from Input
   - If path doesn't start with any allowed path: Deny
   - Otherwise: Continue

   DenyPaths(paths ...string) PreToolUseHook:
   - Return hook that checks if tool is in pathTools
   - If path starts with any denied path: Deny
   - Otherwise: Continue

   RedirectPath(from, to string) PreToolUseHook:
   - Return hook that checks if tool is in pathTools
   - If path starts with 'from':
     - Rewrite path (replace prefix)
     - Return Allow with UpdatedInput containing new path
   - Otherwise: Continue

3. **agent/hooks_commands_test.go**:
   - DenyCommands("rm -rf") blocks "rm -rf /"
   - DenyCommands("rm -rf") allows "ls -la"
   - DenyCommands with multiple patterns
   - RequireCommand("make", "go build") blocks "go build"
   - RequireCommand reason message is helpful
   - Both return Continue for non-Bash tools

4. **agent/hooks_paths_test.go**:
   - AllowPaths("/sandbox") allows "/sandbox/file.txt"
   - AllowPaths("/sandbox") denies "/etc/passwd"
   - AllowPaths with multiple paths
   - DenyPaths("/etc") denies "/etc/passwd"
   - DenyPaths allows non-matching paths
   - RedirectPath("/tmp", "/sandbox/tmp") rewrites correctly
   - RedirectPath returns Allow (not Continue)
   - All return Continue for non-path tools

5. **examples/hooks/main.go**:

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
       ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
       defer cancel()

       a, err := agent.New(ctx,
           agent.Model("claude-sonnet-4-5"),
           agent.PreToolUse(
               // Block dangerous commands
               agent.DenyCommands("rm -rf", "sudo", "chmod 777"),
               
               // Suggest make over direct go commands
               agent.RequireCommand("make", "go build", "go test"),
               
               // Restrict file access
               agent.AllowPaths(".", "/tmp"),
               
               // Custom hook for logging
               func(tc *agent.ToolCall) agent.HookResult {
                   fmt.Printf("[Hook] Tool: %s\n", tc.Name)
                   return agent.HookResult{Decision: agent.Continue}
               },
           ),
       )
       if err != nil {
           log.Fatal(err)
       }
       defer a.Close()

       fmt.Println("Running with hooks enabled...\n")

       result, err := a.Run(ctx, "List the files in the current directory using ls -la")
       if err != nil {
           log.Fatal(err)
       }

       fmt.Printf("\nResult: %s\n", result.ResultText)
   }
   ```

6. **Makefile** addition:
   ```makefile
   example-hooks:
   	go run ./examples/hooks
   ```

Run: go test ./...
Run: make example-hello (regression)
Run: make example-stream (regression)
Run: make example-hooks
```

---

## Step 7: Extended Options

### Context
Steps 1-6 complete. Working agent with hooks. Now add configuration options for tools, environment, and permissions.

### Deliverables
- Tools() option
- Env() option
- PermissionMode type and option
- Process spawning uses these options

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/options.go — Model, WorkDir, CLIPath, PreToolUse
- agent/process.go — startProcess builds CLI args

Run: go test ./... (should pass)

### Orient
This is step 7 of 15. Adding configuration options.

These options configure Claude Code CLI behavior:
- Tools: which tools Claude can use
- Env: environment variables for the process
- PermissionMode: how tool permissions are handled

### Decide
Modify:
1. agent/options.go — add new options
2. agent/process.go — use options when building CLI args
3. Tests for new options

### Act

1. **agent/options.go** additions:

   Add to config:
   - tools []string
   - env map[string]string
   - permissionMode PermissionMode

   PermissionMode type:
   ```go
   type PermissionMode string

   const (
       PermissionDefault       PermissionMode = "default"
       PermissionAcceptEdits   PermissionMode = "acceptEdits"
       PermissionBypass        PermissionMode = "bypassPermissions"
   )
   ```

   Tools(names ...string) Option:
   - Sets config.tools

   Env(key, value string) Option:
   - Initializes map if nil
   - Sets config.env[key] = value

   PermissionMode(mode PermissionMode) Option:
   - Sets config.permissionMode

   Update newConfig:
   - Initialize env map
   - Set default permissionMode to PermissionDefault

2. **agent/process.go** modifications:

   In startProcess:
   - If config.tools not empty: add --allowedTools flag with comma-separated list
   - If config.permissionMode set: add appropriate flag
   - Merge config.env with os.Environ() (config takes precedence)

3. **agent/options_test.go** additions:
   - Test Tools() sets tools
   - Test multiple Env() calls accumulate
   - Test PermissionMode() sets mode
   - Test defaults are correct

4. **agent/process_test.go** additions:
   - Test CLI args include tools flag
   - Test CLI args include permission mode
   - Test environment is merged correctly

5. Update **examples/hooks/main.go** to use new options:

   ```go
   a, err := agent.New(ctx,
       agent.Model("claude-sonnet-4-5"),
       agent.Tools("Bash", "Read", "Write"),
       agent.PermissionMode(agent.PermissionAcceptEdits),
       agent.Env("TMPDIR", "/tmp/sandbox"),
       agent.PreToolUse(
           agent.DenyCommands("rm -rf", "sudo"),
           agent.AllowPaths(".", "/tmp"),
       ),
   )
   ```

Run: go test ./...
Run all examples (regression check)
```

---

## Step 8: Limits and Sessions

### Context
Step 7 complete. Options for tools/env/permissions. Now add resource limits and session management.

### Deliverables
- MaxTurns option (agent and per-run)
- Timeout run option
- Resume() option
- Fork() option
- MaxTurnsError generation
- Session/conversation example

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/options.go — Options for Model, WorkDir, Tools, Env, PermissionMode, PreToolUse
- agent/errors.go — MaxTurnsError defined but not used yet
- agent/agent.go — Run(), Stream()

Run: go test ./... (should pass)

### Orient
This is step 8 of 15. Adding resource limits and session management.

Key features:
- MaxTurns prevents runaway agents
- Timeout limits wall-clock time
- Resume continues existing session
- Fork branches from existing session

### Decide
Modify:
1. agent/options.go — add limit and session options
2. agent/agent.go — implement limit checking
3. agent/process.go — pass session flags to CLI
4. Create examples/session/main.go

### Act

1. **agent/options.go** additions:

   Add to config:
   - maxTurns int
   - resume string
   - fork bool

   MaxTurns(n int) Option:
   - Sets config.maxTurns

   Resume(sessionID string) Option:
   - Sets config.resume

   Fork(sessionID string) Option:
   - Sets config.resume AND config.fork = true

   RunOption implementation:
   ```go
   type runConfig struct {
       timeout  time.Duration
       maxTurns int
   }

   func newRunConfig(opts ...RunOption) *runConfig {
       rc := &runConfig{}
       for _, opt := range opts {
           opt(rc)
       }
       return rc
   }

   func Timeout(d time.Duration) RunOption {
       return func(rc *runConfig) { rc.timeout = d }
   }

   // MaxTurns can be both Option and RunOption
   func MaxTurnsRun(n int) RunOption {
       return func(rc *runConfig) { rc.maxTurns = n }
   }
   ```

2. **agent/process.go** modifications:
   - If config.resume: add --resume flag
   - If config.fork: add --fork flag (or equivalent)

3. **agent/agent.go** modifications:

   Track turn count:
   - Add turnCount field to Agent
   - Increment on turn boundaries (detect in message stream)

   In Run()/Stream():
   - Apply timeout via context.WithTimeout if rc.timeout > 0
   - Check turn count against maxTurns
   - Return MaxTurnsError when exceeded

4. **agent/agent_test.go** additions:
   - Test MaxTurns limits conversation
   - Test MaxTurnsError fields are correct
   - Test Timeout cancels operation
   - Test per-run options override agent defaults

5. **examples/session/main.go**:

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

       // First session: establish context
       a1, err := agent.New(ctx,
           agent.Model("claude-sonnet-4-5"),
           agent.MaxTurns(10),
       )
       if err != nil {
           log.Fatal(err)
       }

       _, err = a1.Run(ctx, "Remember: my project is called Rooikat. It's a security tool.")
       if err != nil {
           log.Fatal(err)
       }

       sessionID := a1.SessionID()
       fmt.Printf("Session 1: %s\n", sessionID)
       a1.Close()

       // Second session: resume and query
       a2, err := agent.New(ctx,
           agent.Model("claude-sonnet-4-5"),
           agent.Resume(sessionID),
       )
       if err != nil {
           log.Fatal(err)
       }
       defer a2.Close()

       result, err := a2.Run(ctx, "What is my project called and what does it do?")
       if err != nil {
           log.Fatal(err)
       }

       fmt.Printf("\nResumed session response:\n%s\n", result.ResultText)
   }
   ```

6. **Makefile** addition:
   ```makefile
   example-session:
   	go run ./examples/session
   ```

Run: go test ./...
Run all examples
```

---

## Step 9: Structured Output

### Context
Step 8 complete. Limits and sessions work. Now add structured output for programmatic responses.

### Deliverables
- Schema derivation from Go structs
- Output() run option
- OutputSchema() run option
- **Structured output example**

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/options.go — Option and RunOption patterns
- agent/agent.go — Run() returns *Result with ResultText

Run: go test ./... (should pass)

### Orient
This is step 9 of 15. Adding structured output.

The pattern:
1. Derive JSON Schema from Go struct (using reflection)
2. Pass schema to Claude Code
3. Unmarshal response into user's struct

Tags: `json` for field names, `desc` for descriptions.

### Decide
Create:
1. agent/schema.go — schema derivation
2. Modify agent/options.go — Output(), OutputSchema()
3. Modify agent/process.go — pass schema to CLI
4. Modify agent/agent.go — unmarshal response
5. Create examples/structured/main.go

### Act

1. **agent/schema.go**:

   schemaFromType(t reflect.Type) (map[string]any, error):
   - Handle struct types
   - Build JSON Schema:
     ```json
     {
       "type": "object",
       "properties": {...},
       "required": [...]
     }
     ```
   - Use json tag for property names
   - Use desc tag for descriptions
   - Handle types: string, int, int64, float64, bool
   - Handle slices: {"type": "array", "items": {...}}
   - Handle nested structs: recursive call
   - Handle pointers: unwrap and mark not required
   - omitempty: don't add to required array

2. **agent/schema_test.go**:

   ```go
   type TestPlan struct {
       Title string `json:"title" desc:"Plan title"`
       Steps []TestStep `json:"steps" desc:"Steps to execute"`
   }

   type TestStep struct {
       ID     string `json:"id" desc:"Step identifier"`
       Action string `json:"action" desc:"What to do"`
       Done   bool   `json:"done,omitempty" desc:"Is complete"`
   }
   ```

   - Test schema for simple struct
   - Test schema for nested struct
   - Test schema for slice field
   - Test omitempty not in required
   - Test desc tag becomes description
   - Test unsupported types return error

3. **agent/options.go** additions:

   Add to runConfig:
   - outputSchema map[string]any
   - outputTarget any

   Output(ptr any) RunOption:
   - Use reflection to get type from ptr
   - Call schemaFromType
   - Store schema and ptr in runConfig

   OutputSchema(v any) RunOption:
   - Call schemaFromType on v's type
   - Store schema only (no target)

4. **agent/process.go** modifications:
   - If schema provided: pass to CLI (--output-format json_schema --schema '...')

5. **agent/agent.go** modifications:

   In Run():
   - If outputTarget set:
     - Parse result.ResultText as JSON
     - Unmarshal into outputTarget
     - Return error if unmarshal fails

6. **examples/structured/main.go**:

   ```go
   package main

   import (
       "context"
       "fmt"
       "log"
       "time"

       "github.com/wernerstrydom/claude-agent-sdk-go/agent"
   )

   type Recipe struct {
       Name        string       `json:"name" desc:"Recipe name"`
       PrepTime    string       `json:"prep_time" desc:"Preparation time"`
       Ingredients []Ingredient `json:"ingredients" desc:"List of ingredients"`
       Steps       []string     `json:"steps" desc:"Cooking steps"`
   }

   type Ingredient struct {
       Item   string `json:"item" desc:"Ingredient name"`
       Amount string `json:"amount" desc:"Quantity needed"`
   }

   func main() {
       ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
       defer cancel()

       a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
       if err != nil {
           log.Fatal(err)
       }
       defer a.Close()

       var recipe Recipe
       _, err = a.Run(ctx,
           "Create a simple recipe for scrambled eggs",
           agent.Output(&recipe),
       )
       if err != nil {
           log.Fatal(err)
       }

       fmt.Printf("Recipe: %s\n", recipe.Name)
       fmt.Printf("Prep Time: %s\n\n", recipe.PrepTime)

       fmt.Println("Ingredients:")
       for _, ing := range recipe.Ingredients {
           fmt.Printf("  - %s: %s\n", ing.Item, ing.Amount)
       }

       fmt.Println("\nSteps:")
       for i, step := range recipe.Steps {
           fmt.Printf("  %d. %s\n", i+1, step)
       }
   }
   ```

7. **Makefile** addition:
   ```makefile
   example-structured:
   	go run ./examples/structured
   ```

Run: go test ./...
Run all examples
```

---

## Step 10: Audit System

### Context
Step 9 complete. Structured output works. Now add the audit system for observability.

### Deliverables
- AuditEvent type
- AuditHandler type
- AuditHandler() option
- AuditToFile() option
- Event emission at key points
- **Audit example**

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/options.go — config struct with various fields
- agent/agent.go — New(), Run(), Stream(), Close()
- agent/hooks.go — hook evaluation

Run: go test ./... (should pass)

### Orient
This is step 10 of 15. Adding the audit system.

Pattern: Like slog.Handler — optional, transparent to call site.
Events are emitted at key points, handlers decide what to do with them.

Event types:
- session.start, session.end
- message.* for each message type
- hook.pre_tool_use
- error

### Decide
Create:
1. agent/audit.go — types and emission
2. Modify agent/options.go — audit options
3. Modify agent/agent.go — emit events
4. Create examples/audit/main.go

### Act

1. **agent/audit.go**:

   ```go
   type AuditEvent struct {
       Time      time.Time
       SessionID string
       Type      string
       Data      any
   }

   type AuditHandler func(AuditEvent)
   ```

   auditor struct (unexported):
   - handlers []AuditHandler
   - mu sync.RWMutex

   newAuditor(handlers []AuditHandler) *auditor

   (a *auditor) emit(sessionID, eventType string, data any):
   - Build AuditEvent
   - Call each handler (catch panics)
   - Consider: run handlers in goroutine or sync?

   Built-in handlers:

   AuditWriterHandler(w io.Writer) AuditHandler:
   - Return handler that JSON-encodes and writes to w
   - One JSON object per line (JSONL)

   AuditFileHandler(path string) (AuditHandler, func() error):
   - Open file for append
   - Return handler and close function
   - Caller responsible for calling close

2. **agent/options.go** additions:

   Add to config:
   - auditHandlers []AuditHandler
   - auditCleanup []func() error

   AuditHandler(h AuditHandler) Option:
   - Appends to config.auditHandlers

   AuditToFile(path string) Option:
   - Create file handler
   - Append handler to config.auditHandlers
   - Append cleanup to config.auditCleanup

3. **agent/agent.go** modifications:

   Add auditor field to Agent.

   In New():
   - Create auditor from config.auditHandlers
   - Emit "session.start" event

   In message processing:
   - Emit "message.text", "message.thinking", etc.
   - Emit "message.result" for Result

   In hook evaluation (agent/hooks.go or control.go):
   - Emit "hook.pre_tool_use" with hook result

   In Close():
   - Emit "session.end"
   - Call audit cleanup functions

4. **agent/audit_test.go**:
   - Test handler receives events
   - Test multiple handlers all receive events
   - Test handler panic doesn't crash
   - Test AuditWriterHandler writes JSONL
   - Test event types are correct

5. **examples/audit/main.go**:

   ```go
   package main

   import (
       "context"
       "fmt"
       "log"
       "os"
       "time"

       "github.com/wernerstrydom/claude-agent-sdk-go/agent"
   )

   func main() {
       ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
       defer cancel()

       a, err := agent.New(ctx,
           agent.Model("claude-sonnet-4-5"),
           agent.Tools("Bash"),
           agent.AuditToFile("audit.jsonl"),
           agent.AuditHandler(func(e agent.AuditEvent) {
               fmt.Printf("[%s] %s\n", e.Type, e.SessionID[:8])
           }),
           agent.PreToolUse(
               agent.DenyCommands("rm"),
           ),
       )
       if err != nil {
           log.Fatal(err)
       }
       defer a.Close()

       _, err = a.Run(ctx, "Run 'echo hello world' in bash")
       if err != nil {
           log.Fatal(err)
       }

       fmt.Println("\n--- Audit Log ---")
       data, _ := os.ReadFile("audit.jsonl")
       fmt.Println(string(data))
   }
   ```

6. **Makefile** addition:
   ```makefile
   example-audit:
   	go run ./examples/audit
   ```

Run: go test ./...
Run all examples
```

---

## Step 11: Lifecycle Hooks — PostToolUse and Stop

### Context
Step 10 complete. Audit system works. Now add PostToolUse and Stop hooks.

### Deliverables
- PostToolUseHook type
- PostToolUse() option
- StopHook type
- Stop() option
- StopEvent struct
- Audit events for these hooks

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/hooks.go — PreToolUseHook, HookResult, hookChain
- agent/options.go — PreToolUse() option
- agent/audit.go — auditor, event emission

Run: go test ./... (should pass)

### Orient
This is step 11 of 15. Adding PostToolUse and Stop hooks.

PostToolUse: Observe tool results after execution.
Stop: Cleanup when agent stops (metrics, logging, state persistence).

### Decide
Modify:
1. agent/hooks.go — add PostToolUse types
2. agent/options.go — add PostToolUse(), Stop() options
3. agent/agent.go — call hooks at appropriate times
4. agent/audit.go — emit hook events
5. Tests

### Act

1. **agent/hooks.go** additions:

   ```go
   type PostToolUseHook func(*ToolCall, *ToolResult) HookResult

   type ToolResult struct {
       ToolUseID string
       Content   any
       IsError   bool
       Duration  time.Duration
   }

   type StopEvent struct {
       SessionID string
       Reason    string  // "completed", "max_turns", "interrupted", "error"
       NumTurns  int
       CostUSD   float64
   }

   type StopHook func(*StopEvent)
   ```

   postToolUseChain similar to preToolUseChain.

2. **agent/options.go** additions:

   Add to config:
   - postToolUseHooks []PostToolUseHook
   - stopHooks []StopHook

   PostToolUse(hooks ...PostToolUseHook) Option
   Stop(hooks ...StopHook) Option

3. **agent/agent.go** modifications:

   Add fields:
   - totalCost float64 (accumulate from Results)
   - stopReason string

   After tool execution:
   - Call PostToolUse hook chain
   - Emit "hook.post_tool_use" audit event

   In Close() and on error exit:
   - Build StopEvent
   - Call Stop hooks
   - Emit "hook.stop" audit event

4. **agent/hooks_test.go** additions:
   - Test PostToolUse receives ToolCall and ToolResult
   - Test multiple PostToolUse hooks called
   - Test Stop hook receives StopEvent
   - Test StopEvent has correct fields

5. Update **examples/audit/main.go**:

   ```go
   agent.PostToolUse(func(tc *agent.ToolCall, tr *agent.ToolResult) agent.HookResult {
       fmt.Printf("[PostToolUse] %s completed (error: %v, duration: %s)\n",
           tc.Name, tr.IsError, tr.Duration)
       return agent.HookResult{Decision: agent.Continue}
   }),
   agent.Stop(func(e *agent.StopEvent) {
       fmt.Printf("[Stop] Session %s: %s (%d turns, $%.4f)\n",
           e.SessionID[:8], e.Reason, e.NumTurns, e.CostUSD)
   }),
   ```

Run: go test ./...
Run all examples
```

---

## Step 12: Remaining Lifecycle Hooks

### Context
Step 11 complete. PostToolUse and Stop hooks work. Now add PreCompact, SubagentStop, and UserPromptSubmit.

### Deliverables
- PreCompactHook type and option
- SubagentStopHook type and option
- UserPromptSubmitHook type and option
- Event types and audit events

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/hooks.go — PreToolUseHook, PostToolUseHook, StopHook
- agent/options.go — all hook options
- agent/audit.go — event emission

Run: go test ./... (should pass)

### Orient
This is step 12 of 15. Adding remaining lifecycle hooks.

PreCompact: Extract data before context window compaction.
SubagentStop: Track subagent completion.
UserPromptSubmit: Intercept/modify prompts before sending.

### Decide
Modify:
1. agent/hooks.go — add types
2. agent/options.go — add options
3. agent/parser.go — detect compact and subagent events
4. agent/agent.go — integrate hooks
5. Tests

### Act

1. **agent/hooks.go** additions:

   ```go
   type PreCompactEvent struct {
       SessionID      string
       Trigger        string  // "auto", "manual"
       TranscriptPath string
       TokenCount     int
   }

   type PreCompactResult struct {
       Archive   bool
       ArchiveTo string
       Extract   any
   }

   type PreCompactHook func(*PreCompactEvent) PreCompactResult

   type SubagentStopEvent struct {
       SessionID       string
       SubagentID      string
       SubagentType    string
       ParentToolUseID string
       NumTurns        int
       CostUSD         float64
   }

   type SubagentStopHook func(*SubagentStopEvent)

   type PromptSubmit struct {
       Prompt    string
       SessionID string
       Turn      int
   }

   type PromptHookResult struct {
       UpdatedPrompt string
       Metadata      any
   }

   type UserPromptSubmitHook func(*PromptSubmit) PromptHookResult
   ```

2. **agent/options.go** additions:

   Add to config:
   - preCompactHooks []PreCompactHook
   - subagentStopHooks []SubagentStopHook
   - userPromptSubmitHooks []UserPromptSubmitHook

   PreCompact(hooks ...PreCompactHook) Option
   SubagentStop(hooks ...SubagentStopHook) Option
   UserPromptSubmit(hooks ...UserPromptSubmitHook) Option

3. **agent/parser.go** modifications:
   - Detect compact events from Claude Code output
   - Detect subagent completion events
   - Parse into appropriate structs

4. **agent/agent.go** modifications:

   In message processing:
   - Detect PreCompact events, call hooks
   - Detect SubagentStop events, call hooks

   In Run()/Stream() before sending prompt:
   - Call UserPromptSubmit hooks
   - Use UpdatedPrompt if provided
   - Store Metadata for audit

5. **agent/hooks_test.go** additions:
   - Test PreCompact hook receives event
   - Test PreCompactResult.Archive triggers action
   - Test SubagentStop receives correct event
   - Test UserPromptSubmit modifies prompt
   - Test empty UpdatedPrompt uses original

6. **agent/audit.go** additions:
   - Emit "hook.pre_compact"
   - Emit "hook.subagent_stop"
   - Emit "hook.user_prompt_submit"

Run: go test ./...
Run all examples
```

---

## Step 13: Custom Tools

### Context
Step 12 complete. All hooks work. Now add custom in-process tools.

### Deliverables
- Tool() option
- ToolHandler with explicit parameters
- ToolHandle with struct reflection
- Tool schema generation
- **Tools example**

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/schema.go — schemaFromType()
- agent/hooks.go — ToolCall, control request handling
- agent/options.go — config struct

Run: go test ./... (should pass)

### Orient
This is step 13 of 15. Adding custom in-process tools.

Two patterns:
1. Explicit: ToolParam() for each parameter, ToolHandler() for function
2. Struct-based: ToolHandle() with reflection from input struct

Custom tools are called by intercepting tool use messages and executing Go functions.

### Decide
Create:
1. agent/tool.go — tool definition types
2. Modify agent/options.go — Tool() option
3. Modify agent/control.go — execute custom tools
4. Create examples/tools/main.go

### Act

1. **agent/tool.go**:

   ```go
   type ParamType string

   const (
       ParamString ParamType = "string"
       ParamInt    ParamType = "integer"
       ParamFloat  ParamType = "number"
       ParamBool   ParamType = "boolean"
   )

   type ToolParam struct {
       Name        string
       Type        ParamType
       Description string
       Required    bool
   }

   type Params map[string]any

   func (p Params) String(name string) string { ... }
   func (p Params) Int(name string) int { ... }
   func (p Params) Float(name string) float64 { ... }
   func (p Params) Bool(name string) bool { ... }

   type ToolDef struct {
       Name        string
       Description string
       Parameters  []ToolParam
       Schema      map[string]any
       Handler     func(context.Context, Params) (any, error)
   }

   type ToolOption func(*ToolDef)

   func ToolDescription(desc string) ToolOption
   func ToolParam(name string, t ParamType, desc string) ToolOption
   func ToolParamOptional(name string, t ParamType, desc string) ToolOption
   func ToolHandler(fn func(context.Context, Params) (any, error)) ToolOption

   // Struct-based handler using reflection
   func ToolHandle[T any](fn func(context.Context, T) (any, error)) ToolOption
   ```

   buildToolSchema(def *ToolDef) map[string]any:
   - Build JSON Schema from Parameters
   - Or from struct if using ToolHandle

2. **agent/options.go** additions:

   Add to config:
   - customTools map[string]*ToolDef

   Tool(name string, opts ...ToolOption) Option:
   - Create ToolDef
   - Apply options
   - Build schema
   - Store in config.customTools

3. **agent/control.go** modifications:

   In handleControlRequest:
   - Check if tool is in customTools
   - If yes: execute handler, return result
   - If no: proceed with normal hook evaluation

4. **agent/tool_test.go**:
   - Test ToolParam builds correct definition
   - Test ToolHandle derives schema from struct
   - Test handler receives correct parameters
   - Test handler result returned to Claude
   - Test handler error handled gracefully
   - Test Params accessors work

5. **examples/tools/main.go**:

   ```go
   package main

   import (
       "context"
       "fmt"
       "log"
       "time"

       "github.com/wernerstrydom/claude-agent-sdk-go/agent"
   )

   type CalcInput struct {
       A  float64 `json:"a" desc:"First number"`
       B  float64 `json:"b" desc:"Second number"`
       Op string  `json:"op" desc:"Operation: add, sub, mul, div"`
   }

   func main() {
       ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
       defer cancel()

       a, err := agent.New(ctx,
           agent.Model("claude-sonnet-4-5"),
           agent.Tool("calculate",
               agent.ToolDescription("Perform arithmetic calculation"),
               agent.ToolHandle(func(ctx context.Context, in CalcInput) (any, error) {
                   var result float64
                   switch in.Op {
                   case "add":
                       result = in.A + in.B
                   case "sub":
                       result = in.A - in.B
                   case "mul":
                       result = in.A * in.B
                   case "div":
                       if in.B == 0 {
                           return nil, fmt.Errorf("division by zero")
                       }
                       result = in.A / in.B
                   default:
                       return nil, fmt.Errorf("unknown operation: %s", in.Op)
                   }
                   return map[string]float64{"result": result}, nil
               }),
           ),
       )
       if err != nil {
           log.Fatal(err)
       }
       defer a.Close()

       result, err := a.Run(ctx, "What is 42 multiplied by 17? Use the calculate tool.")
       if err != nil {
           log.Fatal(err)
       }

       fmt.Printf("Result: %s\n", result.ResultText)
   }
   ```

6. **Makefile** addition:
   ```makefile
   example-tools:
   	go run ./examples/tools
   ```

Run: go test ./...
Run all examples
```

---

## Step 14: MCP Server Configuration

### Context
Step 13 complete. Custom in-process tools work. Now add support for external MCP servers.

### Deliverables
- MCPServer() option
- MCP configuration options
- CLI flag generation for MCP servers

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the existing codebase:
- agent/tool.go — custom tool definitions
- agent/options.go — Tool() option, config struct
- agent/process.go — CLI argument building

Run: go test ./... (should pass)

### Orient
This is step 14 of 15. Adding MCP server configuration.

MCP servers are external processes that provide tools.
Claude Code CLI connects to them via stdio, SSE, or HTTP.

### Decide
Create:
1. agent/mcp.go — MCP configuration types
2. Modify agent/options.go — MCPServer() option
3. Modify agent/process.go — pass MCP config to CLI
4. Tests

### Act

1. **agent/mcp.go**:

   ```go
   type MCPConfig struct {
       Name    string
       Type    string  // "stdio", "sse", "http"
       Command string  // for stdio
       Args    []string
       URL     string  // for sse/http
       Headers map[string]string
       Env     map[string]string
   }

   type MCPOption func(*MCPConfig)

   func MCPCommand(cmd string) MCPOption {
       return func(c *MCPConfig) {
           c.Type = "stdio"
           c.Command = cmd
       }
   }

   func MCPArgs(args ...string) MCPOption {
       return func(c *MCPConfig) { c.Args = args }
   }

   func MCPSSE(url string) MCPOption {
       return func(c *MCPConfig) {
           c.Type = "sse"
           c.URL = url
       }
   }

   func MCPHeader(key, value string) MCPOption {
       return func(c *MCPConfig) {
           if c.Headers == nil {
               c.Headers = make(map[string]string)
           }
           c.Headers[key] = value
       }
   }

   func MCPEnv(key, value string) MCPOption {
       return func(c *MCPConfig) {
           if c.Env == nil {
               c.Env = make(map[string]string)
           }
           c.Env[key] = value
       }
   }
   ```

2. **agent/options.go** additions:

   Add to config:
   - mcpServers map[string]*MCPConfig

   MCPServer(name string, opts ...MCPOption) Option:
   - Create MCPConfig with name
   - Apply options
   - Store in config.mcpServers

3. **agent/process.go** modifications:

   In startProcess:
   - For each MCP server:
     - Build appropriate CLI flag (--mcp or config file)
     - Handle different transport types
   - Pass environment variables for stdio servers

4. **agent/mcp_test.go**:
   - Test MCPCommand sets type to stdio
   - Test MCPArgs accumulates arguments
   - Test MCPSSE sets type and URL
   - Test MCPHeader/MCPEnv accumulate
   - Test multiple MCPServer calls work

5. Document in README (placeholder):
   ```go
   agent.MCPServer("github",
       agent.MCPCommand("npx"),
       agent.MCPArgs("@modelcontextprotocol/server-github"),
       agent.MCPEnv("GITHUB_TOKEN", os.Getenv("GITHUB_TOKEN")),
   ),
   ```

Run: go test ./...
Run all examples (MCP not needed for existing examples)
```

---

## Step 15: Subagents, Skills, and Final Polish

### Context
Step 14 complete. All core features work. Final step: subagents, skills, and comprehensive example.

### Deliverables
- Subagent() option with configuration
- Skill() option for inline skills
- SkillsDir() option for filesystem skills
- SystemPromptPreset() and SystemPromptAppend() options
- **Complete comprehensive example**
- README.md

### Prompt

```text
You are continuing implementation of the Claude Agent SDK for Go.

## OODA

### Observe
Examine the entire codebase:
- All message types, options, hooks
- Process management, parsing
- Custom tools, MCP servers
- Audit system

Run: go test ./... (should pass)
Run all examples (should work)

### Orient
This is step 15 of 15. Final features and polish.

Adding:
- Subagents: Claude can spawn child agents
- Skills: Markdown instructions loaded into context
- System prompt customization

Then: comprehensive example and documentation.

### Decide
Create:
1. agent/subagent.go — subagent configuration
2. agent/skills.go — skill loading
3. Modify agent/options.go — all new options
4. Modify agent/process.go — pass to CLI
5. Create examples/complete/main.go
6. Create README.md

### Act

1. **agent/subagent.go**:

   ```go
   type SubagentConfig struct {
       Name        string
       Description string
       Prompt      string
       Tools       []string
       Model       string
   }

   type SubagentOption func(*SubagentConfig)

   func SubagentDescription(desc string) SubagentOption
   func SubagentPrompt(prompt string) SubagentOption
   func SubagentTools(tools ...string) SubagentOption
   func SubagentModel(model string) SubagentOption
   ```

2. **agent/skills.go**:

   loadSkillsFromDir(path string) (map[string]string, error):
   - Walk directory
   - Find SKILL.md files
   - Read content
   - Return map of name -> content

3. **agent/options.go** additions:

   Add to config:
   - subagents map[string]*SubagentConfig
   - skills map[string]string
   - skillDirs []string
   - systemPromptPreset string
   - systemPromptAppend string

   Options:
   - Subagent(name string, opts ...SubagentOption) Option
   - Skill(name, content string) Option
   - SkillsDir(path string) Option
   - SystemPromptPreset(name string) Option
   - SystemPromptAppend(text string) Option

4. **agent/process.go** modifications:
   - Load skills from skillDirs
   - Pass skills to CLI
   - Pass subagent definitions to CLI
   - Pass system prompt configuration

5. **examples/complete/main.go**:

   ```go
   package main

   import (
       "context"
       "fmt"
       "log"
       "os"
       "time"

       "github.com/wernerstrydom/claude-agent-sdk-go/agent"
   )

   var goSkill = `# Go Development
   - Use gofmt for formatting
   - Handle all errors
   - Write table-driven tests
   `

   type TaskPlan struct {
       Title string   `json:"title" desc:"Plan title"`
       Steps []string `json:"steps" desc:"Steps to complete"`
   }

   func main() {
       ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
       defer cancel()

       a, err := agent.New(ctx,
           // Model and environment
           agent.Model("claude-sonnet-4-5"),
           agent.WorkDir("."),
           agent.Env("GOPROXY", "direct"),

           // Tools and permissions
           agent.Tools("Bash", "Read", "Write", "Edit", "Glob", "Grep", "Task"),
           agent.PermissionMode(agent.PermissionAcceptEdits),

           // Limits
           agent.MaxTurns(25),

           // Security hooks
           agent.PreToolUse(
               agent.DenyCommands("rm -rf /", "sudo"),
               agent.RequireCommand("make", "go build", "go test"),
               agent.AllowPaths(".", "/tmp"),
           ),

           // Observability
           agent.PostToolUse(func(tc *agent.ToolCall, tr *agent.ToolResult) agent.HookResult {
               if tr.IsError {
                   fmt.Printf("⚠️  %s failed\n", tc.Name)
               }
               return agent.HookResult{Decision: agent.Continue}
           }),
           agent.Stop(func(e *agent.StopEvent) {
               fmt.Printf("\n📊 %s: %d turns, $%.4f\n", e.Reason, e.NumTurns, e.CostUSD)
           }),

           // Audit
           agent.AuditToFile("session.jsonl"),

           // Skills and prompts
           agent.Skill("go", goSkill),
           agent.SystemPromptAppend("Explain your reasoning before acting."),

           // Subagents
           agent.Subagent("tester",
               agent.SubagentDescription("Runs tests and reports results"),
               agent.SubagentTools("Bash", "Read"),
               agent.SubagentModel("haiku"),
           ),

           // Custom tools
           agent.Tool("timestamp",
               agent.ToolDescription("Get current timestamp"),
               agent.ToolHandler(func(ctx context.Context, p agent.Params) (any, error) {
                   return map[string]string{"timestamp": time.Now().Format(time.RFC3339)}, nil
               }),
           ),
       )
       if err != nil {
           log.Fatal(err)
       }
       defer a.Close()

       fmt.Printf("🚀 Session: %s\n\n", a.SessionID()[:8])

       // Multi-turn with streaming
       prompts := []string{
           "Create a simple Go program that prints 'Hello, World!'",
           "Add a function to reverse a string",
           "Write tests for the reverse function",
       }

       for i, prompt := range prompts {
           fmt.Printf("─── Turn %d ───\n%s\n\n", i+1, prompt)

           for msg := range a.Stream(ctx, prompt) {
               switch m := msg.(type) {
               case *agent.Text:
                   fmt.Print(m.Text)
               case *agent.ToolUse:
                   fmt.Printf("\n🔧 %s\n", m.Name)
               case *agent.Result:
                   fmt.Printf("\n✅ $%.4f\n\n", m.CostUSD)
               }
           }

           if err := a.Err(); err != nil {
               log.Fatal(err)
           }
       }

       // Structured output
       var plan TaskPlan
       _, err = a.Run(ctx, "Create a plan for adding error handling", agent.Output(&plan))
       if err != nil {
           log.Fatal(err)
       }

       fmt.Printf("\n📋 Plan: %s\n", plan.Title)
       for i, step := range plan.Steps {
           fmt.Printf("   %d. %s\n", i+1, step)
       }
   }
   ```

6. **README.md**:

   ```markdown
   # Claude Agent SDK for Go

   A Go SDK for automating Claude Code CLI.

   ## Installation

   ```bash
   go get github.com/wernerstrydom/claude-agent-sdk-go
   ```

   ## Prerequisites

   - Claude Code CLI installed
   - ANTHROPIC_API_KEY environment variable set

   ## Quick Start

   ```go
   a, _ := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
   defer a.Close()

   result, _ := a.Run(ctx, "Hello, Claude!")
   fmt.Println(result.ResultText)
   ```

   ## Features

   - Blocking and streaming execution
   - Hook system for security and observability
   - Structured output with JSON schema
   - Session resume and fork
   - Custom tools and MCP servers
   - Subagents and skills
   - Comprehensive audit system

   ## Examples

   See the `examples/` directory:
   - `hello/` - Basic usage
   - `stream/` - Streaming responses
   - `hooks/` - Security hooks
   - `session/` - Session management
   - `structured/` - Structured output
   - `audit/` - Audit logging
   - `tools/` - Custom tools
   - `complete/` - Full featured example
   ```

7. **Makefile** final:
   ```makefile
   .PHONY: test lint examples

   test:
   	go test -v -race ./...

   lint:
   	golangci-lint run

   examples: example-hello example-stream example-hooks example-session \
             example-structured example-audit example-tools example-complete

   example-%:
   	go run ./examples/$*
   ```

Run: go test -v -race ./...
Run: make examples
Run: go vet ./...

The SDK is complete!
```

---

## Final Checklist

```text
Before releasing v0.1.0:

1. All tests pass:
   go test -v -race ./...

2. All examples run:
   make examples

3. No lint errors:
   golangci-lint run

4. Documentation complete:
   - README.md
   - GoDoc comments on all exported types
   - Examples for all features

5. Clean git history:
   git log --oneline

6. Tag release:
   git tag v0.1.0
   git push origin v0.1.0
```

---

## Summary

| Step | Milestone | Working Example |
|------|-----------|-----------------|
| 1 | Types compile | - |
| 2 | Process + parser | - |
| 3 | **Agent works** | ✅ hello |
| 4 | Streaming | ✅ stream |
| 5 | Hook system | - |
| 6 | **Hooks work** | ✅ hooks |
| 7 | Extended options | - |
| 8 | Limits & sessions | ✅ session |
| 9 | **Structured output** | ✅ structured |
| 10 | **Audit system** | ✅ audit |
| 11 | Lifecycle hooks | - |
| 12 | All hooks | - |
| 13 | **Custom tools** | ✅ tools |
| 14 | MCP servers | - |
| 15 | **Complete** | ✅ complete |

Total: 15 steps, 8 working examples, full feature set.
