package agent_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// ExampleNew demonstrates creating a new agent.
// Note: With stream-json input mode, session ID is captured lazily after first message.
func ExampleNew() {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
sleep 10
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, err := agent.New(ctx, agent.CLIPath(fakeClaude))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	fmt.Println("Agent created successfully")
	// Output:
	// Agent created successfully
}

// ExampleAgent_Run demonstrates running a prompt.
func ExampleAgent_Run() {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"run-example"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"4"}]}}'
echo '{"type":"result","result":"4","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, _ := agent.New(ctx, agent.CLIPath(fakeClaude))
	defer a.Close()

	result, err := a.Run(ctx, "What is 2 + 2?")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	fmt.Printf("Cost: $%.3f\n", result.CostUSD)
	// Output:
	// Result: 4
	// Cost: $0.001
}

// ExampleAgent_SessionID demonstrates getting the session ID after a message exchange.
func ExampleAgent_SessionID() {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"sess-12345"}'
echo '{"type":"result","result":"OK","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, _ := agent.New(ctx, agent.CLIPath(fakeClaude))
	defer a.Close()

	// Session ID is empty before first message exchange
	// Send a message to trigger init
	for range a.Stream(ctx, "test") {
	}

	fmt.Println("Session ID:", a.SessionID())
	// Output:
	// Session ID: sess-12345
}

// ExampleAgent_Stream demonstrates streaming messages from an agent.
func ExampleAgent_Stream() {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"stream-example"}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Go channels are typed conduits."}]}}'
echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":" They enable communication between goroutines."}]}}'
echo '{"type":"result","result":"Done","num_turns":1,"total_cost_usd":0.002}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, _ := agent.New(ctx, agent.CLIPath(fakeClaude))
	defer a.Close()

	for msg := range a.Stream(ctx, "Explain Go channels") {
		switch m := msg.(type) {
		case *agent.Text:
			fmt.Print(m.Text)
		case *agent.Result:
			fmt.Printf("\nCost: $%.3f\n", m.CostUSD)
		}
	}
	// Output:
	// Go channels are typed conduits. They enable communication between goroutines.
	// Cost: $0.002
}

// ExampleAgent_Err demonstrates checking for errors after streaming.
func ExampleAgent_Err() {
	// Create a fake CLI for testing (stream-json mode)
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"err-example"}'
echo '{"type":"result","result":"OK","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, _ := agent.New(ctx, agent.CLIPath(fakeClaude))
	defer a.Close()

	// Consume all messages
	for range a.Stream(ctx, "test") {
	}

	// Check for errors
	if err := a.Err(); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("No errors")
	}
	// Output:
	// No errors
}

// Example_agentWithHooks demonstrates creating an agent with security hooks.
// This is the recommended way to configure hooks for production use.
func Example_agentWithHooks() {
	// This example shows the complete pattern for creating a secure agent.
	// The hooks are evaluated in order when Claude attempts to use tools.

	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"hooks-example"}'
echo '{"type":"result","result":"Listed files","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with security hooks
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.PreToolUse(
			// Block privileged commands
			agent.DenyCommands("sudo", "su", "chmod 777"),

			// Enforce build system usage
			agent.RequireCommand("make", "go build", "go test"),

			// Restrict file access to project directory
			agent.AllowPaths(".", "/tmp"),

			// Redirect temp files to sandbox
			agent.RedirectPath("/tmp", "./sandbox/tmp"),
		),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "List files in the current directory")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Listed files
}

// ExampleModel demonstrates specifying a Claude model for the agent.
func ExampleModel() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"model-example"}'
echo '{"type":"result","result":"Model configured","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with a specific model
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.Model("claude-sonnet-4-5"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "Hello")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Model configured
}

// ExampleWorkDir demonstrates setting the working directory for an agent.
func ExampleWorkDir() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"workdir-example"}'
echo '{"type":"result","result":"Working in project dir","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	// Create a project directory to use as working directory
	projectDir := filepath.Join(tmpDir, "project")
	os.Mkdir(projectDir, 0755)

	ctx := context.Background()

	// Create agent with a specific working directory
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.WorkDir(projectDir),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "What directory are we in?")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Working in project dir
}

// ExampleDenyPaths demonstrates blocking file operations on specific paths.
func ExampleDenyPaths() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"denypaths-example"}'
echo '{"type":"result","result":"Path restrictions active","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent that blocks access to sensitive paths
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.PreToolUse(
			// Block access to system directories and SSH keys
			agent.DenyPaths("/etc", "/usr", "~/.ssh"),
		),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "List files")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Path restrictions active
}

// ExampleTools demonstrates specifying available tools for the agent.
func ExampleTools() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"tools-example"}'
echo '{"type":"result","result":"Tools configured","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with specific tools enabled
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.Tools("Bash", "Read", "Write", "Edit"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "Hello")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Tools configured
}

// ExampleAllowedTools demonstrates fine-grained tool permissions.
func ExampleAllowedTools() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"allowedtools-example"}'
echo '{"type":"result","result":"Allowed tools configured","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with tool permission patterns
	// Bash(git:*) allows only git commands in Bash
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.AllowedTools("Bash(git:*)", "Read", "Write"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "Hello")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Allowed tools configured
}

// ExamplePermissionPrompt demonstrates setting permission handling mode.
func ExamplePermissionPrompt() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"permission-example"}'
echo '{"type":"result","result":"Permission mode configured","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with auto-accept for file edits
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "Hello")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Permission mode configured
}

// ExampleEnv demonstrates setting environment variables for the agent.
func ExampleEnv() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"env-example"}'
echo '{"type":"result","result":"Environment configured","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with custom environment variables
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.Env("TMPDIR", "/sandbox/tmp"),
		agent.Env("HOME", "/sandbox/home"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "Hello")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Environment configured
}

// ExampleAddDir demonstrates adding allowed directories for the agent.
func ExampleAddDir() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"adddir-example"}'
echo '{"type":"result","result":"Directories added","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create agent with additional allowed directories
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.AddDir("/data", "/shared"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "Hello")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Directories added
}

// Example_agentWithExtendedOptions demonstrates a fully configured agent
// with all the extended options available in Step 7.
func Example_agentWithExtendedOptions() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
read line
echo '{"type":"system","subtype":"init","session_id":"extended-example"}'
echo '{"type":"result","result":"Fully configured agent ready","num_turns":1,"total_cost_usd":0.001}'
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()

	// Create a fully configured agent with all extended options
	a, err := agent.New(ctx,
		agent.CLIPath(fakeClaude),
		agent.Model("claude-sonnet-4-5"),

		// Tool configuration
		agent.Tools("Bash", "Read", "Write", "Edit"),
		agent.AllowedTools("Bash(git:*)", "Bash(make:*)"),

		// Permission mode
		agent.PermissionPrompt(agent.PermissionAcceptEdits),

		// Environment
		agent.Env("TMPDIR", "/sandbox/tmp"),
		agent.Env("GOPROXY", "direct"),

		// Additional directories
		agent.AddDir("/project/data"),

		// Security hooks
		agent.PreToolUse(
			agent.DenyCommands("rm -rf", "sudo"),
			agent.AllowPaths(".", "/tmp"),
		),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer a.Close()

	result, err := a.Run(ctx, "Hello")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Result:", result.ResultText)
	// Output:
	// Result: Fully configured agent ready
}
