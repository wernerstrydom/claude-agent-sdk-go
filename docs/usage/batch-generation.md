# Batch Generation

Generate the same program in many languages concurrently. Each language gets its own agent and goroutine, and results are collected through a channel.

This is the Go equivalent of running `claude -p "Write hello world in {lang}"` twenty times from a shell script -- but with concurrency, structured output, and cost tracking.

## The Code

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

type generated struct {
	Lang     string
	Filename string
	Code     string
	Cost     float64
}

var langs = []string{
	"Go", "Python", "Rust", "TypeScript", "C",
	"C++", "Java", "C#", "Ruby", "Swift",
	"Kotlin", "Scala", "Haskell", "Elixir", "Zig",
	"Lua", "Perl", "R", "Dart", "OCaml",
}

const prompt = `Write a "Hello, World!" program in %s.

Reply with ONLY:
1. The filename (e.g. hello.go)
2. A blank line
3. The code

No markdown fences, no explanation.`

func main() {
	ctx := context.Background()
	outDir := "generated"
	os.MkdirAll(outDir, 0755)

	results := make(chan generated, len(langs))
	var wg sync.WaitGroup

	// Limit concurrency to avoid overwhelming the CLI
	sem := make(chan struct{}, 5)

	for _, lang := range langs {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			a, err := agent.New(ctx,
				agent.Model("claude-sonnet-4-5"),
				agent.MaxTurns(1),
			)
			if err != nil {
				log.Printf("%s: %v", lang, err)
				return
			}
			defer a.Close()

			result, err := a.Run(ctx, fmt.Sprintf(prompt, lang))
			if err != nil {
				log.Printf("%s: %v", lang, err)
				return
			}

			// Parse filename from first line
			text := result.ResultText
			filename := lang + ".txt"
			for i, c := range text {
				if c == '\n' {
					filename = text[:i]
					text = text[i+1:]
					// Skip blank line
					if len(text) > 0 && text[0] == '\n' {
						text = text[1:]
					}
					break
				}
			}

			results <- generated{
				Lang:     lang,
				Filename: filename,
				Code:     text,
				Cost:     result.CostUSD,
			}
		}(lang)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var totalCost float64
	var count int
	for r := range results {
		path := filepath.Join(outDir, r.Filename)
		if err := os.WriteFile(path, []byte(r.Code), 0644); err != nil {
			log.Printf("write %s: %v", path, err)
			continue
		}
		fmt.Printf("%-12s → %s\n", r.Lang, r.Filename)
		totalCost += r.Cost
		count++
	}
	fmt.Printf("\n%d files generated, total cost: $%.4f\n", count, totalCost)
}
```

## How It Works

1. A buffered channel `results` collects output from all goroutines.
2. A semaphore channel `sem` limits concurrency to 5 simultaneous agents -- enough to keep throughput high without spawning 20 CLI processes at once.
3. Each goroutine creates its own agent with `MaxTurns(1)` since only a single response is needed.
4. The main goroutine reads from the channel as results arrive, writes each file, and accumulates cost.
5. `sync.WaitGroup` ensures the channel is closed only after all goroutines finish.

## Adapting This Pattern

The language list and prompt are the only things specific to "Hello World." The same structure works for any batch task where you want to run the same prompt with different parameters:

- Generate unit test files for a list of source files
- Translate documentation into multiple languages
- Create configuration files for different environments
- Scaffold boilerplate for a list of microservices
