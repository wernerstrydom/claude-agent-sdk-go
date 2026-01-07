// Package main demonstrates a comprehensive example using all features of the Claude Agent SDK.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// goSkill provides Go development guidelines loaded as a skill.
var goSkill = `# Go Development Guidelines
- Use gofmt for formatting
- Handle all errors explicitly
- Write table-driven tests
- Use meaningful variable names
`

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create a comprehensive agent with all features
	a, err := agent.New(ctx,
		// Model and environment
		agent.Model("claude-sonnet-4-5"),
		agent.WorkDir("."),
		agent.Env("GOPROXY", "direct"),

		// Tools and permissions
		agent.Tools("Bash", "Read", "Write", "Edit", "Glob", "Grep", "Task"),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),

		// Limits
		agent.MaxTurns(25),

		// Security hooks
		agent.PreToolUse(
			agent.DenyCommands("rm -rf /", "sudo"),
			agent.RequireCommand("make", "go build", "go test"),
			agent.AllowPaths(".", "/tmp"),
		),

		// Observability
		agent.PostToolUse(func(tc *agent.ToolCall, tr *agent.ToolResultContext) agent.HookResult {
			if tr.IsError {
				fmt.Printf("⚠️  %s failed\n", tc.Name)
			}
			return agent.HookResult{Decision: agent.Continue}
		}),
		agent.OnStop(func(e *agent.StopEvent) {
			fmt.Printf("\n📊 %s: %d turns, $%.4f\n", e.Reason, e.NumTurns, e.CostUSD)
		}),

		// Audit
		agent.AuditToFile("session.jsonl"),

		// Skills and prompts
		agent.Skill("go", goSkill),
		agent.SystemPromptAppend("Explain your reasoning before acting."),

		// Subagents
		agent.Subagent("tester",
			agent.SubagentDescription("Runs tests and reports results"),
			agent.SubagentTools("Bash", "Read"),
			agent.SubagentModel("haiku"),
		),

		// Custom tools
		agent.CustomTool(agent.NewFuncTool(
			"timestamp",
			"Get current timestamp",
			map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
			func(ctx context.Context, input map[string]any) (any, error) {
				return map[string]string{"timestamp": time.Now().Format(time.RFC3339)}, nil
			},
		)),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = a.Close() }()

	// Remove audit file on exit for cleanliness
	defer func() { _ = os.Remove("session.jsonl") }()

	fmt.Printf("🚀 Session: %s\n\n", a.SessionID()[:8])

	// Multi-turn with streaming
	prompts := []string{
		"What is the current time? Use the timestamp tool.",
		"Explain what a Go channel is in one sentence.",
	}

	for i, prompt := range prompts {
		fmt.Printf("─── Turn %d ───\n%s\n\n", i+1, prompt)

		for msg := range a.Stream(ctx, prompt) {
			switch m := msg.(type) {
			case *agent.Text:
				fmt.Print(m.Text)
			case *agent.ToolUse:
				fmt.Printf("\n🔧 %s\n", m.Name)
			case *agent.Result:
				fmt.Printf("\n✅ $%.4f\n\n", m.CostUSD)
			}
		}

		if err := a.Err(); err != nil {
			log.Fatal(err)
		}
	}

	// Final request
	result, err := a.Run(ctx, "Summarize what we discussed in one sentence.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n📋 Summary: %s\n", result.ResultText)
}
