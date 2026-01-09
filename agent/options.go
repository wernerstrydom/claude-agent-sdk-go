package agent

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
