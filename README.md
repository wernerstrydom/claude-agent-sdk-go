# Claude Agent SDK for Go

A Go SDK for automating the [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code), inspired by Anthropic's official [Python](https://pypi.org/project/claude-agent-sdk/) and [TypeScript](https://www.npmjs.com/package/@anthropic-ai/claude-agent-sdk) Agent SDKs.

> **Unofficial project.** This is a personal project. It is not affiliated with, endorsed by, or supported by Anthropic. Please read the [important note on authentication](#important-note-on-authentication) before using this SDK.

## Why This Exists

If you've used Claude Code, you've probably run `claude -p "..."` more than once for repetitive tasks. Maybe you've written bash scripts to loop over prompts -- the [Ralph Wiggum](https://github.com/anthropics/claude-code/blob/main/plugins/ralph-wiggum/README.md) pattern is a well-known example. Bash works, but it gets unwieldy when you need to parse JSON responses, handle YAML configs, manage concurrency, or compose multi-step workflows.

This SDK lets you do the same thing in Go. A few examples of where it's useful:

- **Batch generation** -- Generate "Hello World" programs in 20 languages by calling `Run()` in a loop with a different language each time, instead of running `claude -p "Write hello world in {lang}"` twenty times from a shell script.
- **Ralph-style loops** -- Implement the iterative [Ralph loop](https://shipyard.build/blog/claude-code-ralph-loop/) pattern with proper structured data handling, progress tracking, and exit conditions in Go rather than bash.
- **Structured output** -- Parse Claude's responses directly into Go structs instead of piping JSON through `jq`.
- **Workflow orchestration** -- Chain multiple prompts with hooks that enforce security policies, audit tool usage, or gate file access.

## How It Works

This SDK spawns the `claude` CLI as a subprocess and communicates with it over stdin/stdout using the CLI's JSON streaming protocol. It does not extract OAuth tokens or API keys, does not make HTTP requests to Anthropic's API, and does not bypass Claude Code in any way. All interaction flows through the CLI exactly as if you were running it yourself.

```
Your Go code  →  SDK  →  claude CLI (subprocess)  →  Anthropic
               stdin/stdout JSON
```

However you've authenticated Claude Code -- API key, OAuth, or a cloud provider -- that's what the SDK uses. It has no involvement in authentication.

## Important Note on Authentication

Because this SDK automates Claude Code, the same terms of service that apply to Claude Code apply here.

**Use an API key.** This is the safest and clearest path. Authenticate Claude Code with an `ANTHROPIC_API_KEY` from [Claude Console](https://console.anthropic.com/) and your usage is governed by the [Commercial Terms of Service](https://www.anthropic.com/legal/commercial-terms), which permit building products and services.

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

**Consumer plan tokens (Free/Pro/Max) are a different story.** Anthropic's [Legal and Compliance](https://code.claude.com/docs/en/legal-and-compliance) page states that OAuth tokens from consumer plans are intended exclusively for Claude Code and Claude.ai. Using them in any other tool or service -- including the Agent SDK -- is explicitly not permitted under the [Consumer Terms of Service](https://www.anthropic.com/legal/consumer-terms). Whether personal or hobby use of a consumer subscription through this SDK is allowed is not clearly addressed in the terms. The safest reading is that it is not.

**If you access Claude through AWS Bedrock or Google Vertex AI**, your existing commercial agreement with those providers applies.

When in doubt, use an API key.

### Relevant Links

- [Commercial Terms of Service](https://www.anthropic.com/legal/commercial-terms)
- [Consumer Terms of Service](https://www.anthropic.com/legal/consumer-terms)
- [Claude Code Legal and Compliance](https://code.claude.com/docs/en/legal-and-compliance)
- [Anthropic Usage Policy](https://www.anthropic.com/legal/aup)

## Prerequisites

- Go 1.21 or later
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) installed and available on `PATH`
- Claude Code authenticated (API key recommended)

## Installation

```bash
go get github.com/wernerstrydom/claude-agent-sdk-go/agent
```

## Example: Hello World in Five Languages

This is the kind of thing that's tedious in bash but straightforward in Go. It spawns one agent per language using a goroutine, collects results through a channel, and prints them as they arrive:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

type helloResult struct {
	Lang string
	Code string
	Cost float64
}

const prompt = `Write a minimal "Hello, World!" program in %s.
Reply with only the code, no explanation or markdown fences.`

func main() {
	ctx := context.Background()
	langs := []string{"Go", "Python", "Rust", "TypeScript", "C"}

	results := make(chan helloResult, len(langs))
	var wg sync.WaitGroup

	for _, lang := range langs {
		wg.Add(1)
		go func(lang string) {
			defer wg.Done()

			a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"), agent.MaxTurns(1))
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

			results <- helloResult{Lang: lang, Code: result.ResultText, Cost: result.CostUSD}
		}(lang)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var totalCost float64
	for r := range results {
		fmt.Printf("── %s ──\n%s\n\n", r.Lang, r.Code)
		totalCost += r.Cost
	}
	fmt.Printf("Total cost: $%.4f\n", totalCost)
}
```

Compare this to the bash equivalent -- five sequential `claude -p` calls with no concurrency, no structured cost tracking, and string munging to collect results.

## Quick Start

A minimal example with a single prompt:

```go
a, err := agent.New(ctx, agent.Model("claude-sonnet-4-5"))
if err != nil {
	log.Fatal(err)
}
defer a.Close()

result, err := a.Run(ctx, "What is 2+2?")
if err != nil {
	log.Fatal(err)
}
fmt.Println(result.ResultText)
```

## Streaming

```go
for msg := range a.Stream(ctx, "Explain Go channels") {
	switch m := msg.(type) {
	case *agent.Text:
		fmt.Print(m.Text)
	case *agent.ToolUse:
		fmt.Printf("\n[tool: %s]\n", m.Name)
	case *agent.Result:
		fmt.Printf("\nCost: $%.4f\n", m.CostUSD)
	}
}
```

## Features

- **Core API** -- `New()`, `Run()`, `Stream()`, `Close()` for agent lifecycle
- **37+ configuration options** -- Model, working directory, environment variables, tools, permissions, and more
- **6 hook types** -- `PreToolUse`, `PostToolUse`, `OnStop`, `PreCompact`, `SubagentStop`, `UserPromptSubmit`
- **5 built-in hooks** -- `DenyCommands`, `RequireCommand`, `AllowPaths`, `DenyPaths`, `RedirectPath`
- **Structured output** -- JSON schema-constrained responses via `WithSchema` and `RunStructured`
- **Sessions** -- `MaxTurns`, `Resume`, `Fork` for session management
- **Custom tools** -- In-process tool execution with `CustomTool`
- **MCP servers** -- External tool providers via `MCPServer`
- **Subagents** -- Delegated task execution with `Subagent`
- **Skills** -- Domain knowledge injection with `Skill` and `SkillsDir`
- **Audit system** -- Event logging with `AuditToFile`

## Security Hooks

Compose hooks to enforce security policies on tool execution:

```go
a, _ := agent.New(ctx,
	agent.PreToolUse(
		agent.DenyCommands("sudo", "rm -rf"),
		agent.AllowPaths("/sandbox", "/tmp"),
		agent.DenyPaths("/etc", "/var"),
	),
)
```

Hook chains evaluate in order: the first `Deny` wins immediately, `Allow` short-circuits remaining hooks, and `Continue` passes to the next hook.

## More Examples

The [usage examples](docs/usage/) have more detailed, runnable programs:

- [**Plan loop**](docs/usage/plan-loop.md) -- Ralph-style iterate-until-done: read a JSON plan, implement each item, review in a separate pass
- [**Batch generation**](docs/usage/batch-generation.md) -- Generate programs across 20+ languages concurrently with goroutines and channels
- [**Driver scaffolding**](docs/usage/driver-scaffolding.md) -- Generate `Store` interface implementations for MySQL, MongoDB, Azure Table Storage, and more
- [**Repository maintenance**](docs/usage/repository-maintenance.md) -- Iterate through repositories checking for dependency updates, language migrations, and missing capabilities

## Documentation

See the [docs/](docs/) directory for detailed documentation:

- [Getting Started](docs/getting-started/installation.md) -- Prerequisites and installation
- [Usage Examples](docs/usage/) -- Practical examples reducing developer toil
- **Concepts** -- [Agents](docs/concepts/agents.md), [Sessions](docs/concepts/sessions.md), [Hooks](docs/concepts/hooks.md), [Structured Output](docs/concepts/structured-output.md), [Subagents](docs/concepts/subagents.md), [MCP Servers](docs/concepts/mcp-servers.md), [Skills](docs/concepts/skills.md), [Audit](docs/concepts/audit.md)
- [Tutorial Series](docs/tutorials/README.md) -- Build a TODO application progressively across 6 tutorials

## Testing

```bash
go test ./...    # 257 passing tests
go build ./...   # verify compilation
```

## License

[MIT](LICENSE)
