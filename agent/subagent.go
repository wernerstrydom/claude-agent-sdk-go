package agent

// SubagentConfig defines configuration for a subagent that can be spawned by the Task tool.
// Subagents are child agents that run autonomously with their own configuration.
type SubagentConfig struct {
	Name        string   // Subagent name (used as key for Task tool)
	Description string   // Description shown to Claude for subagent selection
	Prompt      string   // System prompt or instructions for the subagent
	Tools       []string // Tools available to the subagent
	Model       string   // Model override for the subagent (empty = inherit from parent)
}

// SubagentOption configures a subagent.
type SubagentOption func(*SubagentConfig)

// SubagentDescription sets the description for the subagent.
// This description helps Claude understand when to use this subagent.
func SubagentDescription(desc string) SubagentOption {
	return func(c *SubagentConfig) {
		c.Description = desc
	}
}

// SubagentPrompt sets the system prompt or instructions for the subagent.
func SubagentPrompt(prompt string) SubagentOption {
	return func(c *SubagentConfig) {
		c.Prompt = prompt
	}
}

// SubagentTools sets the tools available to the subagent.
// If not set, the subagent inherits tools from the parent agent.
func SubagentTools(tools ...string) SubagentOption {
	return func(c *SubagentConfig) {
		c.Tools = tools
	}
}

// SubagentModel sets the model for the subagent.
// Common values: "haiku" for fast/cheap tasks, "sonnet" for balanced tasks.
// If not set, the subagent inherits the model from the parent agent.
func SubagentModel(model string) SubagentOption {
	return func(c *SubagentConfig) {
		c.Model = model
	}
}
