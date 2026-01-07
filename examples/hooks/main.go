package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	a, err := agent.New(ctx,
		agent.Model("claude-sonnet-4-5"),
		agent.PreToolUse(
			// Block privileged and network commands
			agent.DenyCommands("sudo", "curl", "wget"),

			// Suggest make over direct go commands
			agent.RequireCommand("make", "go build", "go test"),

			// Restrict file access to current directory and /tmp
			agent.AllowPaths(".", "/tmp"),

			// Custom hook for logging tool calls
			func(tc *agent.ToolCall) agent.HookResult {
				fmt.Printf("[Hook] Tool: %s\n", tc.Name)
				return agent.HookResult{Decision: agent.Continue}
			},
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	fmt.Println("Running with hooks enabled...")

	result, err := a.Run(ctx, "List the files in the current directory using ls -la")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nResult: %s\n", result.ResultText)
	fmt.Printf("Cost: $%.4f\n", result.CostUSD)
}
