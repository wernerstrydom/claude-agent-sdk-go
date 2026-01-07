package agent

import (
	"encoding/json"
	"reflect"
	"time"
)

// PermissionMode controls how tool permissions are handled.
type PermissionMode string

const (
	// PermissionDefault uses standard permission checks.
	PermissionDefault PermissionMode = "default"
	// PermissionAcceptEdits automatically accepts file edit operations.
	PermissionAcceptEdits PermissionMode = "acceptEdits"
	// PermissionBypass bypasses all permission checks (use with caution).
	PermissionBypass PermissionMode = "bypassPermissions"
	// PermissionDontAsk skips permission prompts.
	PermissionDontAsk PermissionMode = "dontAsk"
	// PermissionPlan uses plan mode for permissions.
	PermissionPlan PermissionMode = "plan"
)

// config holds agent configuration.
type config struct {
	model           string
	workDir         string
	cliPath         string
	preToolUseHooks []PreToolUseHook

	// Tool configuration
	tools           []string // --tools: available tools
	allowedTools    []string // --allowedTools: permission patterns
	disallowedTools []string // --disallowedTools: deny patterns

	// Permission and environment
	permissionMode PermissionMode    // --permission-mode
	env            map[string]string // process environment variables

	// Directory and settings
	addDirs        []string // --add-dir: additional allowed directories
	settingSources []string // --setting-sources: which settings to load

	// Limits
	maxTurns int // Maximum turns allowed (0 = unlimited)

	// Session management
	resume string // Session ID to resume
	fork   bool   // Fork from resumed session (creates new session ID)

	// Structured output
	jsonSchema  string // JSON Schema for --json-schema flag
	schemaError error  // Error from schema generation (deferred until New())

	// Audit system
	auditHandlers []AuditHandler // Handlers to receive audit events
	auditCleanup  []func() error // Cleanup functions for file handlers

	// Lifecycle hooks
	postToolUseHooks      []PostToolUseHook      // Called after tool execution
	stopHooks             []StopHook             // Called when agent stops
	preCompactHooks       []PreCompactHook       // Called before context compaction
	subagentStopHooks     []SubagentStopHook     // Called when subagent completes
	userPromptSubmitHooks []UserPromptSubmitHook // Called before prompt submission

	// Custom tools
	customTools map[string]Tool // In-process tools executed by SDK

	// MCP server configuration
	mcpServers      map[string]*MCPConfig // MCP servers keyed by name
	strictMCPConfig bool                  // Only use SDK-configured MCP servers

	// Subagent configuration
	subagents map[string]*SubagentConfig // Subagents keyed by name

	// Skills configuration
	skills    map[string]*SkillConfig // Inline skills keyed by name
	skillDirs []string                // Directories to load skills from

	// System prompt configuration
	systemPromptPreset string // Preset system prompt name
	systemPromptAppend string // Text to append to system prompt
}

// Option configures an Agent.
type Option func(*config)

// Model sets the Claude model to use.
func Model(name string) Option {
	return func(c *config) {
		c.model = name
	}
}

// WorkDir sets the working directory for the agent.
func WorkDir(path string) Option {
	return func(c *config) {
		c.workDir = path
	}
}

// CLIPath overrides the default Claude CLI location.
func CLIPath(path string) Option {
	return func(c *config) {
		c.cliPath = path
	}
}

// PreToolUse adds hooks that are called before tool execution.
// Hooks are evaluated in order: first Deny wins, Allow short-circuits.
func PreToolUse(hooks ...PreToolUseHook) Option {
	return func(c *config) {
		c.preToolUseHooks = append(c.preToolUseHooks, hooks...)
	}
}

// Tools sets the available tools for the agent.
// Use specific names like "Bash", "Read", "Write", "Edit", "Glob", "Grep", "Task".
// An empty slice disables all tools.
func Tools(names ...string) Option {
	return func(c *config) {
		c.tools = names
	}
}

// AllowedTools sets tool permission patterns.
// Patterns can include globs for fine-grained control, e.g., "Bash(git:*)" allows
// only git commands in Bash.
func AllowedTools(patterns ...string) Option {
	return func(c *config) {
		c.allowedTools = patterns
	}
}

// DisallowedTools sets tool denial patterns.
// Tools matching these patterns will be blocked.
func DisallowedTools(patterns ...string) Option {
	return func(c *config) {
		c.disallowedTools = patterns
	}
}

// CustomTool registers one or more custom in-process tools.
// Custom tools are executed directly by the SDK without going through the CLI,
// enabling fast execution with access to application state.
//
// When Claude invokes a custom tool, the SDK:
// 1. Intercepts the tool call
// 2. Executes the tool's Execute method
// 3. Returns the result to Claude
//
// Custom tools are evaluated after PreToolUse hooks, so hooks can still
// deny or modify custom tool invocations.
//
// Example:
//
//	calculator := agent.NewFuncTool(
//	    "calculator",
//	    "Evaluates arithmetic expressions",
//	    map[string]any{
//	        "type": "object",
//	        "properties": map[string]any{
//	            "expression": map[string]any{"type": "string"},
//	        },
//	        "required": []string{"expression"},
//	    },
//	    func(ctx context.Context, input map[string]any) (any, error) {
//	        // ... implementation
//	        return result, nil
//	    },
//	)
//	a, _ := agent.New(ctx, agent.CustomTool(calculator))
func CustomTool(tools ...Tool) Option {
	return func(c *config) {
		if c.customTools == nil {
			c.customTools = make(map[string]Tool)
		}
		for _, tool := range tools {
			c.customTools[tool.Name()] = tool
		}
	}
}

// MCPServer configures an MCP (Model Context Protocol) server.
// MCP servers provide external tools to Claude via stdio, SSE, or HTTP transports.
//
// Example:
//
//	a, _ := agent.New(ctx,
//	    agent.MCPServer("github",
//	        agent.MCPHTTP("https://api.githubcopilot.com/mcp"),
//	        agent.MCPHeader("Authorization", "Bearer token"),
//	    ),
//	    agent.MCPServer("local-server",
//	        agent.MCPCommand("npx"),
//	        agent.MCPArgs("my-mcp-server"),
//	        agent.MCPEnv("API_KEY", "secret"),
//	    ),
//	)
func MCPServer(name string, opts ...MCPOption) Option {
	return func(c *config) {
		cfg := &MCPConfig{Name: name}
		for _, opt := range opts {
			opt(cfg)
		}
		if c.mcpServers == nil {
			c.mcpServers = make(map[string]*MCPConfig)
		}
		c.mcpServers[name] = cfg
	}
}

// StrictMCPConfig ensures only SDK-configured MCP servers are used.
// When enabled, user and project MCP configurations are ignored,
// providing a controlled environment with only explicitly configured servers.
func StrictMCPConfig(strict bool) Option {
	return func(c *config) {
		c.strictMCPConfig = strict
	}
}

// Subagent configures a subagent that can be spawned by the Task tool.
// Subagents are child agents that run autonomously with their own configuration.
//
// Example:
//
//	a, _ := agent.New(ctx,
//	    agent.Subagent("tester",
//	        agent.SubagentDescription("Runs tests and reports results"),
//	        agent.SubagentTools("Bash", "Read"),
//	        agent.SubagentModel("haiku"),
//	    ),
//	)
func Subagent(name string, opts ...SubagentOption) Option {
	return func(c *config) {
		cfg := &SubagentConfig{Name: name}
		for _, opt := range opts {
			opt(cfg)
		}
		if c.subagents == nil {
			c.subagents = make(map[string]*SubagentConfig)
		}
		c.subagents[name] = cfg
	}
}

// Skill adds an inline skill with the given name and content.
// Skills are markdown instructions loaded into Claude's context.
//
// Example:
//
//	goSkill := `# Go Development
//	- Use gofmt for formatting
//	- Handle all errors`
//	a, _ := agent.New(ctx, agent.Skill("go", goSkill))
func Skill(name, content string) Option {
	return func(c *config) {
		if c.skills == nil {
			c.skills = make(map[string]*SkillConfig)
		}
		c.skills[name] = &SkillConfig{
			Name:    name,
			Content: content,
		}
	}
}

// SkillsDir loads skills from a directory.
// Skill files can be named SKILL.md (in a named directory) or *.skill.md.
// Multiple directories can be added by calling SkillsDir multiple times.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.SkillsDir("./skills"))
func SkillsDir(path string) Option {
	return func(c *config) {
		c.skillDirs = append(c.skillDirs, path)
	}
}

// SystemPromptPreset sets a preset system prompt by name.
// Presets provide predefined personas and behaviors for Claude.
func SystemPromptPreset(name string) Option {
	return func(c *config) {
		c.systemPromptPreset = name
	}
}

// SystemPromptAppend adds text to the end of the system prompt.
// Use this to add custom instructions without replacing the default prompt.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.SystemPromptAppend("Always explain your reasoning."))
func SystemPromptAppend(text string) Option {
	return func(c *config) {
		c.systemPromptAppend = text
	}
}

// PermissionPrompt sets how tool permissions are handled.
func PermissionPrompt(mode PermissionMode) Option {
	return func(c *config) {
		c.permissionMode = mode
	}
}

// Env sets an environment variable for the agent process.
// Multiple calls accumulate environment variables.
func Env(key, value string) Option {
	return func(c *config) {
		if c.env == nil {
			c.env = make(map[string]string)
		}
		c.env[key] = value
	}
}

// AddDir adds directories the agent is allowed to access.
// These are passed to the CLI via --add-dir flag.
func AddDir(paths ...string) Option {
	return func(c *config) {
		c.addDirs = append(c.addDirs, paths...)
	}
}

// SettingSources controls which settings files are loaded.
// Valid values: "user", "project", "local".
func SettingSources(sources ...string) Option {
	return func(c *config) {
		c.settingSources = sources
	}
}

// MaxTurns sets the maximum number of turns allowed.
// A turn is a complete assistant response. When exceeded, Run() returns MaxTurnsError.
// A value of 0 means unlimited (default).
func MaxTurns(n int) Option {
	return func(c *config) {
		c.maxTurns = n
	}
}

// Resume continues a previous session by its ID.
// The session ID can be obtained from Agent.SessionID() or from a previous result.
func Resume(sessionID string) Option {
	return func(c *config) {
		c.resume = sessionID
	}
}

// Fork branches from an existing session, creating a new session ID.
// The original session remains unchanged. This is useful for trying
// different approaches while preserving the original conversation.
func Fork(sessionID string) Option {
	return func(c *config) {
		c.resume = sessionID
		c.fork = true
	}
}

// WithSchema configures the agent for structured output using the provided
// type as a template. All responses will be formatted as JSON matching
// the generated schema.
//
// Example:
//
//	type Response struct {
//	    Answer string `json:"answer" desc:"The answer"`
//	}
//	a, _ := agent.New(ctx, agent.WithSchema(Response{}))
func WithSchema(example any) Option {
	return func(c *config) {
		t := reflect.TypeOf(example)
		if t == nil {
			c.schemaError = &SchemaError{Type: "nil", Reason: "example cannot be nil"}
			return
		}

		schema, err := schemaFromType(t)
		if err != nil {
			c.schemaError = err
			return
		}

		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			c.schemaError = &SchemaError{
				Type:   t.String(),
				Reason: "failed to marshal schema",
				Cause:  err,
			}
			return
		}

		c.jsonSchema = string(schemaJSON)
	}
}

// WithSchemaRaw configures the agent with a custom JSON Schema.
// Use this for schemas that cannot be derived from Go types.
//
// Example:
//
//	schema := map[string]any{
//	    "type": "object",
//	    "properties": map[string]any{
//	        "name": map[string]any{"type": "string"},
//	    },
//	}
//	a, _ := agent.New(ctx, agent.WithSchemaRaw(schema))
func WithSchemaRaw(schema map[string]any) Option {
	return func(c *config) {
		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			c.schemaError = &SchemaError{
				Reason: "failed to marshal schema",
				Cause:  err,
			}
			return
		}
		c.jsonSchema = string(schemaJSON)
	}
}

// Audit adds a handler that receives audit events during agent execution.
// Multiple handlers can be added by calling Audit multiple times.
// Events are emitted at key points: session.start, session.end, message.*,
// hook.pre_tool_use, and error.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.Audit(func(e agent.AuditEvent) {
//	    log.Printf("[%s] %s: %v", e.Type, e.SessionID, e.Data)
//	}))
func Audit(h AuditHandler) Option {
	return func(c *config) {
		c.auditHandlers = append(c.auditHandlers, h)
	}
}

// AuditToFile configures the agent to write audit events to a file in JSONL format.
// The file is created or appended to. The file is closed when the agent is closed.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.AuditToFile("audit.jsonl"))
func AuditToFile(path string) Option {
	return func(c *config) {
		handler, cleanup, err := AuditFileHandler(path)
		if err != nil {
			// Store error for later reporting - we can't return it from Option
			// Use a special error that will be checked in New()
			c.schemaError = &StartError{
				Reason: "failed to open audit file",
				Cause:  err,
			}
			return
		}
		c.auditHandlers = append(c.auditHandlers, handler)
		c.auditCleanup = append(c.auditCleanup, cleanup)
	}
}

// PostToolUse adds hooks that are called after tool execution completes.
// These hooks receive the original tool call and the result, allowing for
// observation, logging, and metrics collection.
//
// Unlike PreToolUse hooks, PostToolUse hooks cannot modify or block the result.
// All hooks are called in order regardless of their return values.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.PostToolUse(
//	    func(tc *agent.ToolCall, tr *agent.ToolResultContext) agent.HookResult {
//	        log.Printf("Tool %s completed in %v", tc.Name, tr.Duration)
//	        return agent.HookResult{Decision: agent.Continue}
//	    },
//	))
func PostToolUse(hooks ...PostToolUseHook) Option {
	return func(c *config) {
		c.postToolUseHooks = append(c.postToolUseHooks, hooks...)
	}
}

// OnStop adds hooks that are called when the agent session ends.
// Stop hooks receive information about the session including total turns,
// cost, and the reason for stopping.
//
// Stop hooks are for cleanup, metrics reporting, and logging. They are called
// in Close() before resources are released.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.OnStop(func(e *agent.StopEvent) {
//	    log.Printf("Session %s ended: %s (%d turns, $%.4f)",
//	        e.SessionID, e.Reason, e.NumTurns, e.CostUSD)
//	}))
func OnStop(hooks ...StopHook) Option {
	return func(c *config) {
		c.stopHooks = append(c.stopHooks, hooks...)
	}
}

// PreCompact adds hooks that are called before context window compaction.
// These hooks can archive the current transcript or extract important data
// before Claude compacts the context window.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.PreCompact(func(e *agent.PreCompactEvent) agent.PreCompactResult {
//	    log.Printf("Compacting session %s (trigger: %s, tokens: %d)",
//	        e.SessionID, e.Trigger, e.TokenCount)
//	    return agent.PreCompactResult{
//	        Archive:   true,
//	        ArchiveTo: fmt.Sprintf("archives/%s.json", time.Now().Format("20060102-150405")),
//	    }
//	}))
func PreCompact(hooks ...PreCompactHook) Option {
	return func(c *config) {
		c.preCompactHooks = append(c.preCompactHooks, hooks...)
	}
}

// SubagentStop adds hooks that are called when a subagent completes execution.
// Subagents are spawned by the Task tool and run autonomously. These hooks
// allow observation of subagent execution for metrics and logging.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.SubagentStop(func(e *agent.SubagentStopEvent) {
//	    log.Printf("Subagent %s (%s) completed: %d turns, $%.4f",
//	        e.SubagentID, e.SubagentType, e.NumTurns, e.CostUSD)
//	}))
func SubagentStop(hooks ...SubagentStopHook) Option {
	return func(c *config) {
		c.subagentStopHooks = append(c.subagentStopHooks, hooks...)
	}
}

// UserPromptSubmit adds hooks that are called before a prompt is sent to Claude.
// These hooks can modify the prompt or attach metadata for audit purposes.
//
// The hooks are called in order, and each hook receives the potentially
// modified prompt from previous hooks. If a hook returns a non-empty
// UpdatedPrompt, that becomes the new prompt for subsequent hooks.
//
// Example:
//
//	a, _ := agent.New(ctx, agent.UserPromptSubmit(func(e *agent.PromptSubmitEvent) agent.PromptSubmitResult {
//	    // Add system context to all prompts
//	    return agent.PromptSubmitResult{
//	        UpdatedPrompt: e.Prompt + "\n[Context: Production environment]",
//	        Metadata: map[string]any{"original_length": len(e.Prompt)},
//	    }
//	}))
func UserPromptSubmit(hooks ...UserPromptSubmitHook) Option {
	return func(c *config) {
		c.userPromptSubmitHooks = append(c.userPromptSubmitHooks, hooks...)
	}
}

// newConfig creates a config with defaults, then applies options.
func newConfig(opts ...Option) *config {
	c := &config{
		model:          "claude-sonnet-4-5",
		workDir:        ".",
		permissionMode: PermissionDefault,
		env:            make(map[string]string),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// runConfig holds per-run configuration.
type runConfig struct {
	timeout  time.Duration // Per-run timeout (0 = use context timeout)
	maxTurns int           // Per-run max turns override (0 = use agent default)
}

// RunOption configures a single Run() call.
type RunOption func(*runConfig)

// newRunConfig creates a runConfig and applies options.
func newRunConfig(opts ...RunOption) *runConfig {
	rc := &runConfig{}
	for _, opt := range opts {
		opt(rc)
	}
	return rc
}

// Timeout sets a timeout for this Run() call.
// If the operation exceeds this duration, the context is cancelled.
func Timeout(d time.Duration) RunOption {
	return func(rc *runConfig) {
		rc.timeout = d
	}
}

// MaxTurnsRun overrides the agent-level MaxTurns for this Run() call.
func MaxTurnsRun(n int) RunOption {
	return func(rc *runConfig) {
		rc.maxTurns = n
	}
}
