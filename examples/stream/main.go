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

	a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()

	fmt.Println("Streaming response:")

	for msg := range a.Stream(ctx, "Explain what Go channels are in 3 sentences.") {
		switch m := msg.(type) {
		case *agent.Text:
			fmt.Print(m.Text)
		case *agent.Thinking:
			fmt.Printf("[thinking...]\n")
		case *agent.ToolUse:
			fmt.Printf("\n[tool: %s]\n", m.Name)
		case *agent.Result:
			fmt.Printf("\n\n---\nDone: %d turns, $%.4f\n", m.NumTurns, m.CostUSD)
		case *agent.Error:
			log.Printf("Error: %v\n", m.Err)
		}
	}

	if err := a.Err(); err != nil {
		log.Fatal(err)
	}
}
