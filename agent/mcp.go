package agent

// MCPConfig holds configuration for an MCP (Model Context Protocol) server.
// MCP servers provide external tools to Claude via stdio, SSE, or HTTP transports.
type MCPConfig struct {
	Name      string            // Server name (key for --mcp-config)
	Transport string            // "stdio", "sse", or "http"
	Command   string            // Executable (stdio only)
	Args      []string          // Command arguments (stdio only)
	URL       string            // Server URL (sse/http only)
	Headers   map[string]string // Request headers (sse/http only)
	Env       map[string]string // Environment variables (stdio only)
}

// MCPOption configures an MCP server.
type MCPOption func(*MCPConfig)

// MCPCommand sets the transport to "stdio" and specifies the command to run.
func MCPCommand(cmd string) MCPOption {
	return func(c *MCPConfig) {
		c.Transport = "stdio"
		c.Command = cmd
	}
}

// MCPArgs sets the command arguments for a stdio transport.
func MCPArgs(args ...string) MCPOption {
	return func(c *MCPConfig) {
		c.Args = args
	}
}

// MCPSSE sets the transport to "sse" (Server-Sent Events) and specifies the URL.
func MCPSSE(url string) MCPOption {
	return func(c *MCPConfig) {
		c.Transport = "sse"
		c.URL = url
	}
}

// MCPHTTP sets the transport to "http" and specifies the URL.
func MCPHTTP(url string) MCPOption {
	return func(c *MCPConfig) {
		c.Transport = "http"
		c.URL = url
	}
}

// MCPHeader adds a header to the MCP server configuration.
// Multiple calls accumulate headers. Used for sse/http transports.
func MCPHeader(key, val string) MCPOption {
	return func(c *MCPConfig) {
		if c.Headers == nil {
			c.Headers = make(map[string]string)
		}
		c.Headers[key] = val
	}
}

// MCPEnv adds an environment variable to the MCP server configuration.
// Multiple calls accumulate environment variables. Used for stdio transport.
func MCPEnv(key, val string) MCPOption {
	return func(c *MCPConfig) {
		if c.Env == nil {
			c.Env = make(map[string]string)
		}
		c.Env[key] = val
	}
}
