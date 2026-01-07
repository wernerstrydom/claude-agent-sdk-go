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

	result, err := a.Run(ctx, "What is 2 + 2? Reply with just the number.")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Response: %s\n", result.ResultText)
	fmt.Printf("Cost: $%.4f\n", result.CostUSD)
	fmt.Printf("Session: %s\n", a.SessionID())
}
