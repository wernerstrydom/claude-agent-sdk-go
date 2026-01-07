package agent_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// ExampleNew demonstrates creating a new agent.
func ExampleNew() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"example-session"}'
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
	fmt.Println("Session:", a.SessionID())
	// Output:
	// Agent created successfully
	// Session: example-session
}

// ExampleAgent_Run demonstrates running a prompt.
func ExampleAgent_Run() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"run-example"}'
read line
echo '{"type":"assistant","content":[{"type":"text","text":"4"}]}'
echo '{"type":"result","result":"4","num_turns":1,"cost_usd":0.001}'
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

// ExampleAgent_SessionID demonstrates getting the session ID.
func ExampleAgent_SessionID() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"sess-12345"}'
sleep 10
`
	os.WriteFile(fakeClaude, []byte(script), 0755)

	ctx := context.Background()
	a, _ := agent.New(ctx, agent.CLIPath(fakeClaude))
	defer a.Close()

	fmt.Println("Session ID:", a.SessionID())
	// Output:
	// Session ID: sess-12345
}

// ExampleAgent_Stream demonstrates streaming messages from an agent.
func ExampleAgent_Stream() {
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"stream-example"}'
read line
echo '{"type":"assistant","content":[{"type":"text","text":"Go channels are typed conduits."}]}'
echo '{"type":"assistant","content":[{"type":"text","text":" They enable communication between goroutines."}]}'
echo '{"type":"result","result":"Done","num_turns":1,"cost_usd":0.002}'
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
	// Create a fake CLI for testing
	tmpDir, _ := os.MkdirTemp("", "claude-test")
	defer os.RemoveAll(tmpDir)

	fakeClaude := filepath.Join(tmpDir, "claude")
	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"err-example"}'
read line
echo '{"type":"result","result":"OK","num_turns":1,"cost_usd":0.001}'
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
