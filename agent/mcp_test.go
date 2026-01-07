package agent

import (
	"testing"
)

func TestMCPCommand(t *testing.T) {
	cfg := &MCPConfig{}
	MCPCommand("npx")(cfg)

	if cfg.Transport != "stdio" {
		t.Errorf("expected transport 'stdio', got %q", cfg.Transport)
	}
	if cfg.Command != "npx" {
		t.Errorf("expected command 'npx', got %q", cfg.Command)
	}
}

func TestMCPArgs(t *testing.T) {
	cfg := &MCPConfig{}
	MCPArgs("-y", "my-server", "--port", "3000")(cfg)

	expected := []string{"-y", "my-server", "--port", "3000"}
	if len(cfg.Args) != len(expected) {
		t.Fatalf("expected %d args, got %d", len(expected), len(cfg.Args))
	}
	for i, arg := range expected {
		if cfg.Args[i] != arg {
			t.Errorf("expected arg[%d] = %q, got %q", i, arg, cfg.Args[i])
		}
	}
}

func TestMCPSSE(t *testing.T) {
	cfg := &MCPConfig{}
	MCPSSE("https://example.com/sse")(cfg)

	if cfg.Transport != "sse" {
		t.Errorf("expected transport 'sse', got %q", cfg.Transport)
	}
	if cfg.URL != "https://example.com/sse" {
		t.Errorf("expected URL 'https://example.com/sse', got %q", cfg.URL)
	}
}

func TestMCPHTTP(t *testing.T) {
	cfg := &MCPConfig{}
	MCPHTTP("https://api.example.com/mcp")(cfg)

	if cfg.Transport != "http" {
		t.Errorf("expected transport 'http', got %q", cfg.Transport)
	}
	if cfg.URL != "https://api.example.com/mcp" {
		t.Errorf("expected URL 'https://api.example.com/mcp', got %q", cfg.URL)
	}
}

func TestMCPHeader(t *testing.T) {
	cfg := &MCPConfig{}
	MCPHeader("Authorization", "Bearer token123")(cfg)
	MCPHeader("X-Custom", "value")(cfg)

	if cfg.Headers == nil {
		t.Fatal("expected Headers map to be initialized")
	}
	if cfg.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("expected Authorization header 'Bearer token123', got %q", cfg.Headers["Authorization"])
	}
	if cfg.Headers["X-Custom"] != "value" {
		t.Errorf("expected X-Custom header 'value', got %q", cfg.Headers["X-Custom"])
	}
}

func TestMCPEnv(t *testing.T) {
	cfg := &MCPConfig{}
	MCPEnv("API_KEY", "secret123")(cfg)
	MCPEnv("DEBUG", "true")(cfg)

	if cfg.Env == nil {
		t.Fatal("expected Env map to be initialized")
	}
	if cfg.Env["API_KEY"] != "secret123" {
		t.Errorf("expected API_KEY 'secret123', got %q", cfg.Env["API_KEY"])
	}
	if cfg.Env["DEBUG"] != "true" {
		t.Errorf("expected DEBUG 'true', got %q", cfg.Env["DEBUG"])
	}
}

func TestMCPServer(t *testing.T) {
	cfg := newConfig(
		MCPServer("test-server",
			MCPCommand("npx"),
			MCPArgs("-y", "my-mcp-server"),
			MCPEnv("KEY", "value"),
		),
	)

	if cfg.mcpServers == nil {
		t.Fatal("expected mcpServers to be initialized")
	}
	server, ok := cfg.mcpServers["test-server"]
	if !ok {
		t.Fatal("expected 'test-server' to be in mcpServers")
	}
	if server.Name != "test-server" {
		t.Errorf("expected name 'test-server', got %q", server.Name)
	}
	if server.Transport != "stdio" {
		t.Errorf("expected transport 'stdio', got %q", server.Transport)
	}
	if server.Command != "npx" {
		t.Errorf("expected command 'npx', got %q", server.Command)
	}
	if len(server.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(server.Args))
	}
	if server.Env["KEY"] != "value" {
		t.Errorf("expected env KEY='value', got %q", server.Env["KEY"])
	}
}

func TestMultipleMCPServers(t *testing.T) {
	cfg := newConfig(
		MCPServer("server1",
			MCPCommand("npx"),
			MCPArgs("server1-pkg"),
		),
		MCPServer("server2",
			MCPHTTP("https://api.example.com"),
			MCPHeader("Authorization", "Bearer token"),
		),
		MCPServer("server3",
			MCPSSE("https://sse.example.com"),
		),
	)

	if len(cfg.mcpServers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(cfg.mcpServers))
	}

	// Check server1 (stdio)
	s1 := cfg.mcpServers["server1"]
	if s1.Transport != "stdio" {
		t.Errorf("server1: expected transport 'stdio', got %q", s1.Transport)
	}
	if s1.Command != "npx" {
		t.Errorf("server1: expected command 'npx', got %q", s1.Command)
	}

	// Check server2 (http)
	s2 := cfg.mcpServers["server2"]
	if s2.Transport != "http" {
		t.Errorf("server2: expected transport 'http', got %q", s2.Transport)
	}
	if s2.URL != "https://api.example.com" {
		t.Errorf("server2: expected URL 'https://api.example.com', got %q", s2.URL)
	}
	if s2.Headers["Authorization"] != "Bearer token" {
		t.Errorf("server2: expected Authorization header, got %q", s2.Headers["Authorization"])
	}

	// Check server3 (sse)
	s3 := cfg.mcpServers["server3"]
	if s3.Transport != "sse" {
		t.Errorf("server3: expected transport 'sse', got %q", s3.Transport)
	}
	if s3.URL != "https://sse.example.com" {
		t.Errorf("server3: expected URL 'https://sse.example.com', got %q", s3.URL)
	}
}

func TestMCPServerOverwrite(t *testing.T) {
	cfg := newConfig(
		MCPServer("duplicate",
			MCPCommand("first-cmd"),
		),
		MCPServer("duplicate",
			MCPCommand("second-cmd"),
		),
	)

	if len(cfg.mcpServers) != 1 {
		t.Fatalf("expected 1 server (overwritten), got %d", len(cfg.mcpServers))
	}

	server := cfg.mcpServers["duplicate"]
	if server.Command != "second-cmd" {
		t.Errorf("expected command 'second-cmd' (overwritten), got %q", server.Command)
	}
}

func TestMCPOptionApplied(t *testing.T) {
	cfg := newConfig(
		MCPServer("test",
			MCPHTTP("https://example.com"),
			MCPHeader("Auth", "token"),
		),
	)

	if cfg.mcpServers == nil {
		t.Fatal("expected mcpServers to be non-nil")
	}
	if _, exists := cfg.mcpServers["test"]; !exists {
		t.Error("expected 'test' server to exist")
	}
}

func TestStrictMCPConfig(t *testing.T) {
	// Default should be false
	cfg := newConfig()
	if cfg.strictMCPConfig {
		t.Error("expected strictMCPConfig to be false by default")
	}

	// Should be true when enabled
	cfg = newConfig(StrictMCPConfig(true))
	if !cfg.strictMCPConfig {
		t.Error("expected strictMCPConfig to be true")
	}

	// Should be false when explicitly disabled
	cfg = newConfig(StrictMCPConfig(false))
	if cfg.strictMCPConfig {
		t.Error("expected strictMCPConfig to be false")
	}
}

func TestMCPHTTPWithHeaders(t *testing.T) {
	cfg := newConfig(
		MCPServer("github",
			MCPHTTP("https://api.github.com/mcp"),
			MCPHeader("Authorization", "Bearer ghp_token"),
			MCPHeader("X-GitHub-Api-Version", "2022-11-28"),
		),
	)

	server := cfg.mcpServers["github"]
	if server.Transport != "http" {
		t.Errorf("expected transport 'http', got %q", server.Transport)
	}
	if len(server.Headers) != 2 {
		t.Errorf("expected 2 headers, got %d", len(server.Headers))
	}
}

func TestMCPStdioWithEnv(t *testing.T) {
	cfg := newConfig(
		MCPServer("local",
			MCPCommand("node"),
			MCPArgs("server.js"),
			MCPEnv("NODE_ENV", "production"),
			MCPEnv("PORT", "3000"),
			MCPEnv("SECRET", "s3cr3t"),
		),
	)

	server := cfg.mcpServers["local"]
	if server.Transport != "stdio" {
		t.Errorf("expected transport 'stdio', got %q", server.Transport)
	}
	if len(server.Env) != 3 {
		t.Errorf("expected 3 env vars, got %d", len(server.Env))
	}
	if server.Env["NODE_ENV"] != "production" {
		t.Errorf("expected NODE_ENV='production', got %q", server.Env["NODE_ENV"])
	}
}
