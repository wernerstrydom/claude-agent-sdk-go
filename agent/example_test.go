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
