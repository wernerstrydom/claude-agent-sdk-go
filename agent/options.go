package agent

// config holds agent configuration.
type config struct {
	model           string
	workDir         string
	cliPath         string
	preToolUseHooks []PreToolUseHook
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

// newConfig creates a config with defaults, then applies options.
func newConfig(opts ...Option) *config {
	c := &config{
		model:   "claude-sonnet-4-5",
		workDir: ".",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
