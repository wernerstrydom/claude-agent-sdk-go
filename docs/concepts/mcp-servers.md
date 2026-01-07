# MCP Servers

MCP (Model Context Protocol) servers extend Claude's capabilities by providing external tools. These servers run as
separate processes or services, communicating with Claude through a standardized protocol. This enables integration with
databases, APIs, file systems, and custom services without modifying the SDK.

## Why MCP Servers Exist

The Claude Code CLI includes a fixed set of built-in tools: Bash, Read, Write, Edit, Glob, Grep, and others. While these
cover common operations, many applications require specialized capabilities:

- Querying a specific database system
- Interacting with proprietary APIs
- Accessing domain-specific services
- Integrating with existing infrastructure

MCP servers bridge this gap by implementing tools that run in external processes. Claude discovers available tools from
the MCP server and invokes them through the protocol. The server executes the tool and returns results to Claude.

### Healthcare Domain Example

A healthcare application might configure MCP servers for:

- **FHIR Server** - Queries patient records using the FHIR standard
- **Drug Database** - Looks up medication information and interactions
- **Clinical Terminology** - Resolves medical codes (ICD-10, CPT, SNOMED)

Each server encapsulates domain knowledge and handles authentication, caching, and error handling independently.

## Transport Types

MCP servers communicate using one of three transport mechanisms.

### stdio Transport

The server runs as a child process. Communication occurs through standard input and output streams.

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.MCPServer("fhir-client",
        agent.MCPCommand("npx"),
        agent.MCPArgs("@healthcare/mcp-fhir-server"),
        agent.MCPEnv("FHIR_BASE_URL", "https://fhir.example.com"),
        agent.MCPEnv("FHIR_AUTH_TOKEN", os.Getenv("FHIR_TOKEN")),
    ),
)
```

Use stdio transport when:

- The server is a Node.js package (run with npx)
- The server is a local executable
- You need to pass environment variables securely

### SSE Transport

The server runs as an HTTP service using Server-Sent Events for streaming responses.

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.MCPServer("terminology",
        agent.MCPSSE("https://terminology.example.com/mcp/sse"),
        agent.MCPHeader("Authorization", "Bearer "+apiKey),
    ),
)
```

Use SSE transport when:

- The server is a shared service running remotely
- Multiple clients need to connect to the same server
- The server requires HTTP authentication

### HTTP Transport

The server exposes standard HTTP endpoints for tool invocation.

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.MCPServer("drug-database",
        agent.MCPHTTP("https://api.drugdb.example.com/mcp"),
        agent.MCPHeader("X-API-Key", apiKey),
    ),
)
```

Use HTTP transport when:

- The server is a REST-style API
- You need request/response semantics without streaming
- The server is behind a load balancer or API gateway

## Configuration Options

### MCPConfig Structure

The configuration captures all settings for an MCP server:

```go
type MCPConfig struct {
    Name      string            // Server name (key for configuration)
    Transport string            // "stdio", "sse", or "http"
    Command   string            // Executable (stdio only)
    Args      []string          // Command arguments (stdio only)
    URL       string            // Server URL (sse/http only)
    Headers   map[string]string // Request headers (sse/http only)
    Env       map[string]string // Environment variables (stdio only)
}
```

### Available Options

| Option                | Transport | Purpose                      |
|-----------------------|-----------|------------------------------|
| `MCPCommand(cmd)`     | stdio     | Sets the executable to run   |
| `MCPArgs(args...)`    | stdio     | Sets command-line arguments  |
| `MCPEnv(key, val)`    | stdio     | Adds an environment variable |
| `MCPSSE(url)`         | sse       | Sets the SSE endpoint URL    |
| `MCPHTTP(url)`        | http      | Sets the HTTP endpoint URL   |
| `MCPHeader(key, val)` | sse, http | Adds a request header        |

Multiple options can be combined:

```go
agent.MCPServer("example",
    agent.MCPCommand("python"),
    agent.MCPArgs("-m", "my_mcp_server"),
    agent.MCPEnv("API_KEY", "secret"),
    agent.MCPEnv("LOG_LEVEL", "debug"),
)
```

## Strict MCP Configuration

By default, the CLI loads MCP server configurations from user and project settings files. The `StrictMCPConfig` option
restricts the agent to only use SDK-configured servers:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.MCPServer("approved-service",
        agent.MCPHTTP("https://approved.example.com/mcp"),
    ),
    agent.StrictMCPConfig(true),  // Ignore user/project MCP configs
)
```

Use strict mode when:

- You need a controlled, reproducible environment
- Security policies require explicit tool approval
- User configurations might conflict with application requirements

## Multiple MCP Servers

Configure multiple servers to provide different tool sets:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.MCPServer("fhir",
        agent.MCPCommand("npx"),
        agent.MCPArgs("@healthcare/mcp-fhir"),
        agent.MCPEnv("FHIR_URL", fhirEndpoint),
    ),
    agent.MCPServer("terminology",
        agent.MCPSSE("https://terminology.example.com/mcp"),
        agent.MCPHeader("Authorization", "Bearer "+termToken),
    ),
    agent.MCPServer("imaging",
        agent.MCPHTTP("https://pacs.example.com/mcp"),
        agent.MCPHeader("X-API-Key", pacsKey),
    ),
)
```

Claude sees all tools from all configured servers and selects appropriate tools based on the task.

## Use Cases

### Database Integration

Provide read-only database access through an MCP server:

```go
agent.MCPServer("analytics-db",
    agent.MCPCommand("npx"),
    agent.MCPArgs("@company/mcp-postgres"),
    agent.MCPEnv("DATABASE_URL", dbConnectionString),
    agent.MCPEnv("READ_ONLY", "true"),
)
```

The MCP server implements query tools with appropriate access controls.

### API Wrapping

Expose existing REST APIs as MCP tools:

```go
agent.MCPServer("inventory-api",
    agent.MCPHTTP("https://inventory.internal/mcp"),
    agent.MCPHeader("Authorization", "Bearer "+serviceToken),
)
```

The MCP server translates tool invocations to API calls.

### Local Services

Connect to services running on the local machine:

```go
agent.MCPServer("local-llm",
    agent.MCPCommand("./local-llm-server"),
    agent.MCPArgs("--model", "codellama"),
)
```

This enables hybrid architectures combining Claude with local models.

## Security Considerations

MCP servers execute external code and may have network access. Consider:

1. **Credential Handling** - Use environment variables rather than embedding secrets in code
2. **Tool Restrictions** - MCP servers can implement arbitrary tools; review what each server provides
3. **Network Access** - SSE and HTTP servers may require firewall rules
4. **Strict Mode** - Enable `StrictMCPConfig` in production to prevent unauthorized servers

## Limitations

- The SDK configures MCP servers but does not implement the MCP protocol itself
- Server availability and tool discovery occur when Claude processes a prompt
- MCP server errors may not surface until tool invocation
- Tools from MCP servers are subject to the same permission handling as built-in tools
