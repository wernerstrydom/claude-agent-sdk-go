package agent

// config holds agent configuration.
type config struct {
	model   string
	workDir string
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
