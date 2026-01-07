# Plan Loop

The [Ralph Wiggum](https://github.com/anthropics/claude-code/blob/main/plugins/ralph-wiggum/README.md) pattern iterates through a plan until everything is done. In bash, you'd pipe JSON through `jq` and track state in temp files. In Go, you unmarshal into structs, track progress in memory, and get proper error handling for free.

This example reads a JSON plan file, implements each item, then reviews the implementation in a separate pass.

## The Plan

```json
{
  "project": "url-shortener",
  "items": [
    {"id": "1", "description": "Create a Store interface with Get, Put, Delete methods", "status": "pending"},
    {"id": "2", "description": "Implement an in-memory Store for development", "status": "pending"},
    {"id": "3", "description": "Add HTTP handlers for /shorten and /{code} endpoints", "status": "pending"},
    {"id": "4", "description": "Write table-driven tests for the HTTP handlers", "status": "pending"}
  ]
}
```

Save this as `plan.json` in the working directory.

## The Code

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

type Plan struct {
	Project string `json:"project"`
	Items   []Item `json:"items"`
}

type Item struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"` // "pending", "implemented", "reviewed"
}

func main() {
	data, err := os.ReadFile("plan.json")
	if err != nil {
		log.Fatal(err)
	}

	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Implementation pass
	impl, err := agent.New(ctx,
		agent.Model("claude-sonnet-4-5"),
		agent.WorkDir("."),
		agent.MaxTurns(20),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer impl.Close()

	for i, item := range plan.Items {
		if item.Status != "pending" {
			continue
		}

		prompt := fmt.Sprintf(
			"Project: %s\n\nImplement the following item:\nID: %s\nDescription: %s\n\n"+
				"Write the code. Do not explain, just implement.",
			plan.Project, item.ID, item.Description,
		)

		result, err := impl.Run(ctx, prompt)
		if err != nil {
			log.Printf("item %s: %v", item.ID, err)
			continue
		}

		plan.Items[i].Status = "implemented"
		fmt.Printf("implemented %s ($%.4f)\n", item.ID, result.CostUSD)
	}

	// Review pass with a fresh agent
	reviewer, err := agent.New(ctx,
		agent.Model("claude-sonnet-4-5"),
		agent.WorkDir("."),
		agent.MaxTurns(10),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer reviewer.Close()

	for i, item := range plan.Items {
		if item.Status != "implemented" {
			continue
		}

		prompt := fmt.Sprintf(
			"Review the implementation for item %s: %s\n\n"+
				"Check for bugs, missing edge cases, and style issues. "+
				"If changes are needed, describe them. Otherwise say LGTM.",
			item.ID, item.Description,
		)

		result, err := reviewer.Run(ctx, prompt)
		if err != nil {
			log.Printf("review %s: %v", item.ID, err)
			continue
		}

		plan.Items[i].Status = "reviewed"
		fmt.Printf("reviewed %s: %s\n", item.ID, result.ResultText[:min(80, len(result.ResultText))])
	}

	// Save updated plan
	out, _ := json.MarshalIndent(plan, "", "  ")
	os.WriteFile("plan.json", out, 0644)
}
```

## How It Works

1. The plan is loaded from JSON and parsed into Go structs.
2. An **implementation agent** walks through each pending item, sends a prompt, and marks it as `"implemented"` on success.
3. A separate **review agent** walks through each implemented item and marks it as `"reviewed"`.
4. The updated plan is written back to `plan.json`, so you can re-run the program to pick up where you left off.

The two-agent approach keeps the implementation and review concerns separate. Each agent maintains its own session context, so the reviewer doesn't see the implementation agent's history -- it evaluates the code on disk independently.
