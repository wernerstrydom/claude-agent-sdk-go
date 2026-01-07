//go:build integration

package agent_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// ExampleNew demonstrates creating a new agent.
func ExampleNew() {
	ctx := context.Background()
	a, err := agent.New(ctx, agent.Model("claude-haiku-3-5"))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer func() { _ = a.Close() }()

	fmt.Println("Agent created successfully")
	// Output:
	// Agent created successfully
}

// ExampleAgent_Run demonstrates running a prompt and getting a result.
func ExampleAgent_Run() {
	ctx := context.Background()
	a, err := agent.New(ctx, agent.Model("claude-haiku-3-5"))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer func() { _ = a.Close() }()

	result, err := a.Run(ctx, "What is the capital of France? Reply with only the city name, nothing else.")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Normalize the response
	answer := strings.TrimSpace(strings.ToLower(result.ResultText))
	if strings.Contains(answer, "paris") {
		fmt.Println("Paris")
	} else {
		fmt.Printf("Unexpected: %s\n", result.ResultText)
	}
	// Output:
	// Paris
}

// ExampleAgent_Stream demonstrates streaming messages from an agent.
func ExampleAgent_Stream() {
	ctx := context.Background()
	a, err := agent.New(ctx, agent.Model("claude-haiku-3-5"))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer func() { _ = a.Close() }()

	var fullText strings.Builder

	for msg := range a.Stream(ctx, "What is 2+2? Reply with only the number.") {
		switch m := msg.(type) {
		case *agent.Text:
			fullText.WriteString(m.Text)
		case *agent.Result:
			// Stream complete
		}
	}

	// Check for errors after streaming
	if err := a.Err(); err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Normalize and check the answer
	answer := strings.TrimSpace(fullText.String())
	if strings.Contains(answer, "4") {
		fmt.Println("4")
	} else {
		fmt.Printf("Unexpected: %s\n", answer)
	}
	// Output:
	// 4
}

// ExampleModel demonstrates specifying a Claude model for the agent.
func ExampleModel() {
	ctx := context.Background()

	// Create agent with a specific model (haiku for cost efficiency)
	a, err := agent.New(ctx, agent.Model("claude-haiku-3-5"))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer func() { _ = a.Close() }()

	result, err := a.Run(ctx, "How many continents are there? Reply with only the number.")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Extract the number from response
	answer := strings.TrimSpace(result.ResultText)
	if strings.Contains(answer, "7") {
		fmt.Println("7")
	} else {
		fmt.Printf("Unexpected: %s\n", answer)
	}
	// Output:
	// 7
}

// ExamplePreToolUse demonstrates using hooks to control tool execution.
func ExamplePreToolUse() {
	ctx := context.Background()

	// Create agent with security hooks
	a, err := agent.New(ctx,
		agent.Model("claude-haiku-3-5"),
		agent.PreToolUse(
			// Block dangerous commands
			agent.DenyCommands("rm -rf", "sudo"),
			// Allow access to current directory
			agent.AllowPaths("."),
		),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer func() { _ = a.Close() }()

	// Ask a simple question that doesn't need tools
	result, err := a.Run(ctx, "What is the chemical symbol for water? Reply with only the chemical formula.")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Normalize and check the answer
	answer := strings.TrimSpace(strings.ToUpper(result.ResultText))
	// Match H2O in various formats
	h2oPattern := regexp.MustCompile(`H\s*2\s*O|H₂O`)
	if h2oPattern.MatchString(answer) || strings.Contains(answer, "H2O") {
		fmt.Println("H2O")
	} else {
		fmt.Printf("Unexpected: %s\n", result.ResultText)
	}
	// Output:
	// H2O
}

// ExampleWithSchema demonstrates structured output with JSON schema.
func ExampleWithSchema() {
	type MathAnswer struct {
		Answer      string `json:"answer" desc:"The numeric answer"`
		Explanation string `json:"explanation" desc:"Brief explanation of the calculation"`
	}

	ctx := context.Background()

	// Create agent with structured output schema
	a, err := agent.New(ctx,
		agent.Model("claude-haiku-3-5"),
		agent.WithSchema(MathAnswer{}),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer func() { _ = a.Close() }()

	var answer MathAnswer
	_, err = a.RunWithSchema(ctx, "What is 7 multiplied by 8?", &answer)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Verify the answer contains 56
	if strings.Contains(answer.Answer, "56") {
		fmt.Println("56")
	} else {
		fmt.Printf("Unexpected: %s\n", answer.Answer)
	}
	// Output:
	// 56
}

// ExampleRunStructured demonstrates the one-shot structured output helper.
func ExampleRunStructured() {
	type FactAnswer struct {
		Fact    string `json:"fact" desc:"The factual answer"`
		Country string `json:"country" desc:"The country being asked about"`
	}

	ctx := context.Background()

	var answer FactAnswer
	_, err := agent.RunStructured(
		ctx,
		"What is the capital of Japan? Return the city name and country.",
		&answer,
		agent.Model("claude-haiku-3-5"),
	)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Verify the answer contains Tokyo
	if strings.Contains(strings.ToLower(answer.Fact), "tokyo") {
		fmt.Println("Tokyo")
	} else {
		fmt.Printf("Unexpected: %s\n", answer.Fact)
	}
	// Output:
	// Tokyo
}

// ExampleAgent_SessionID demonstrates getting the session ID.
func ExampleAgent_SessionID() {
	ctx := context.Background()
	a, err := agent.New(ctx, agent.Model("claude-haiku-3-5"))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer func() { _ = a.Close() }()

	// Session ID is empty before first message
	if a.SessionID() == "" {
		fmt.Println("Session ID empty before message: true")
	}

	// Run a prompt to trigger session init
	_, err = a.Run(ctx, "Say hello")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Session ID is now populated
	if a.SessionID() != "" {
		fmt.Println("Session ID present after message: true")
	}
	// Output:
	// Session ID empty before message: true
	// Session ID present after message: true
}
