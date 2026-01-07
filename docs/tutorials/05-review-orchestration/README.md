# Tutorial 05: Review Orchestration

This tutorial demonstrates parallel specialist reviews using multiple agents with structured output. You will build a
code review system where security, database, and performance specialists analyze code concurrently.

## Why Parallel Specialist Reviews?

Code review benefits from diverse perspectives. A security expert notices different issues than a database expert or a
performance engineer. In traditional workflows, these reviews happen sequentially, creating bottlenecks and delays.

Parallel specialist reviews address this by running multiple focused agents simultaneously:

- **Security reviewer** - Identifies vulnerabilities, injection risks, and authentication flaws
- **Database reviewer** - Catches inefficient queries, missing indexes, and connection handling issues
- **Performance reviewer** - Detects bottlenecks, memory allocations, and algorithmic inefficiencies

Each specialist operates with a focused prompt that guides their analysis. Running them in parallel reduces total review
time from the sum of individual review times to approximately the duration of the slowest reviewer.

### Healthcare Domain Example

Consider a healthcare application handling patient records. A security reviewer would examine authentication checks and
data encryption. A database reviewer would analyze query patterns for patient lookups. A performance reviewer would
identify slow operations that could delay critical care information.

## Prerequisites

Before starting this tutorial, ensure you have:

1. **Go 1.21 or later** installed
2. **Claude CLI** installed and configured with a valid API key
3. The **Claude Agent SDK** available in your project
4. Familiarity with Go concurrency patterns (goroutines, channels, sync.WaitGroup)

## Concepts

### Goroutine-Per-Agent Pattern

Each reviewer agent runs in its own goroutine. This pattern maps naturally to the task because each agent operates
independently with its own prompt, context, and session. The goroutines communicate results through a shared channel.

```
main goroutine
     |
     +-- goroutine: security reviewer
     |        |
     |        +-- agent.New()
     |        +-- agent.Run()
     |        +-- send result to channel
     |
     +-- goroutine: database reviewer
     |        |
     |        +-- agent.New()
     |        +-- agent.Run()
     |        +-- send result to channel
     |
     +-- goroutine: performance reviewer
              |
              +-- agent.New()
              +-- agent.Run()
              +-- send result to channel
```

### Channel-Based Result Collection

Results flow through a buffered channel. The buffer size matches the number of reviewers, preventing goroutine leaks if
the main goroutine exits early:

```go
results := make(chan ReviewResult, len(reviewers))
```

A `sync.WaitGroup` coordinates completion. When all reviewers finish, the main goroutine closes the channel and
processes results.

### Structured Output for Review Findings

Each reviewer returns structured JSON matching a predefined schema. This enables programmatic processing of findings,
aggregation across reviewers, and integration with issue trackers or reporting systems.

## Review Schema Definition

Define types that represent the structure of review findings:

```go
// Finding represents a single issue discovered during review.
type Finding struct {
    Severity    string `json:"severity" desc:"Issue severity: critical, high, medium, or low"`
    File        string `json:"file" desc:"Path to the file containing the issue"`
    Line        int    `json:"line" desc:"Line number where the issue occurs"`
    Description string `json:"description" desc:"Clear description of the problem"`
    Suggestion  string `json:"suggestion" desc:"Recommended fix or improvement"`
}

// Review represents the complete output from a specialist reviewer.
type Review struct {
    Reviewer string    `json:"reviewer" desc:"Name of the reviewer (security, database, performance)"`
    Findings []Finding `json:"findings" desc:"List of issues discovered"`
    Summary  string    `json:"summary" desc:"Brief overview of the review results"`
}
```

The `desc` struct tag provides descriptions that become part of the JSON schema sent to Claude. These descriptions guide
Claude toward producing well-formatted output.

### Severity Levels

The severity field uses a four-level scale:

| Severity   | Description                                                                                    |
|------------|------------------------------------------------------------------------------------------------|
| `critical` | Issues that must be fixed before deployment (security breaches, data loss risks)               |
| `high`     | Significant problems that should be fixed soon (performance degradation, reliability concerns) |
| `medium`   | Issues worth addressing but not urgent (code quality, maintainability)                         |
| `low`      | Minor improvements and suggestions (style, documentation)                                      |

## Reviewer Configuration

Each reviewer requires a specialized prompt that focuses their analysis:

```go
// ReviewerConfig defines a specialist reviewer.
type ReviewerConfig struct {
    Name   string
    Prompt string
}

var reviewers = []ReviewerConfig{
    {
        Name: "security",
        Prompt: `You are a security specialist reviewing code for vulnerabilities.
Focus on:
- SQL injection and command injection risks
- Authentication and authorization flaws
- Sensitive data exposure
- Input validation gaps
- Insecure cryptographic practices

Analyze the code and report findings in the required JSON format.`,
    },
    {
        Name: "database",
        Prompt: `You are a database specialist reviewing code for data access issues.
Focus on:
- N+1 query patterns
- Missing indexes on frequently queried columns
- Connection pool mismanagement
- Transaction handling errors
- Inefficient joins and subqueries

Analyze the code and report findings in the required JSON format.`,
    },
    {
        Name: "performance",
        Prompt: `You are a performance specialist reviewing code for bottlenecks.
Focus on:
- Unnecessary allocations in hot paths
- Blocking operations that could be async
- Algorithmic complexity issues
- Resource leaks (goroutines, file handles)
- Inefficient data structures

Analyze the code and report findings in the required JSON format.`,
    },
}
```

## Making Reviewers Read-Only

Reviewers should analyze code without modifying it. The `DenyPaths` and `DenyCommands` hooks enforce this constraint:

```go
func createReviewer(ctx context.Context, cfg ReviewerConfig) (*agent.Agent, error) {
    return agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),

        // Restrict to read-only tools
        agent.Tools("Read", "Glob", "Grep"),

        // Deny all write operations
        agent.PreToolUse(
            // Block any file modifications
            agent.DenyPaths("*"), // Deny Write/Edit to all paths
        ),

        // System prompt establishes the reviewer role
        agent.SystemPromptAppend(cfg.Prompt),

        // Limit turn count to prevent runaway sessions
        agent.MaxTurns(10),
    )
}
```

By specifying only read tools (`Read`, `Glob`, `Grep`), the reviewer cannot invoke `Write`, `Edit`, or `Bash`. The
`DenyPaths` hook provides defense in depth.

## Parallel Execution Pattern

The core orchestration pattern launches all reviewers concurrently and collects their results:

```go
// ReviewResult pairs a reviewer name with its output or error.
type ReviewResult struct {
    Reviewer string
    Review   *Review
    Err      error
    Cost     float64
    Duration time.Duration
}

func runParallelReviews(ctx context.Context, targetPath string) ([]ReviewResult, error) {
    var wg sync.WaitGroup
    results := make(chan ReviewResult, len(reviewers))

    // Launch each reviewer in its own goroutine
    for _, cfg := range reviewers {
        wg.Add(1)
        go func(cfg ReviewerConfig) {
            defer wg.Done()
            result := runSingleReview(ctx, cfg, targetPath)
            results <- result
        }(cfg)
    }

    // Wait for all reviewers to complete, then close the channel
    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results
    var allResults []ReviewResult
    for result := range results {
        allResults = append(allResults, result)
    }

    return allResults, nil
}
```

### Single Review Execution

Each reviewer creates its own agent, sends a prompt, and parses the structured response:

```go
func runSingleReview(ctx context.Context, cfg ReviewerConfig, targetPath string) ReviewResult {
    start := time.Now()

    // Create a specialized agent for this reviewer
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Tools("Read", "Glob", "Grep"),
        agent.MaxTurns(10),
        agent.WithSchema(Review{}),
        agent.SystemPromptAppend(cfg.Prompt),
    )
    if err != nil {
        return ReviewResult{
            Reviewer: cfg.Name,
            Err:      fmt.Errorf("failed to create agent: %w", err),
        }
    }
    defer a.Close()

    // Build the review prompt
    prompt := fmt.Sprintf(`Review the code in %s for issues related to your specialty.
Return your findings as JSON matching the required schema.
Set the "reviewer" field to "%s".`, targetPath, cfg.Name)

    // Run the review and unmarshal the structured response
    var review Review
    result, err := a.RunWithSchema(ctx, prompt, &review)
    if err != nil {
        return ReviewResult{
            Reviewer: cfg.Name,
            Err:      fmt.Errorf("review failed: %w", err),
        }
    }

    return ReviewResult{
        Reviewer: cfg.Name,
        Review:   &review,
        Cost:     result.CostUSD,
        Duration: time.Since(start),
    }
}
```

## Graceful Cancellation

Context cancellation propagates to all running agents. When the user presses Ctrl+C, all agents stop:

```go
func main() {
    ctx, cancel := signal.NotifyContext(
        context.Background(),
        syscall.SIGINT,
        syscall.SIGTERM,
    )
    defer cancel()

    results, err := runParallelReviews(ctx, "./internal/...")
    // ...
}
```

The agents detect context cancellation and terminate cleanly, releasing resources.

## Cost Aggregation

Track total cost across all reviewers:

```go
func aggregateCosts(results []ReviewResult) (total float64, perReviewer map[string]float64) {
    perReviewer = make(map[string]float64)
    for _, r := range results {
        perReviewer[r.Reviewer] = r.Cost
        total += r.Cost
    }
    return total, perReviewer
}
```

Cost tracking enables:

- Budgeting for automated review pipelines
- Comparing efficiency of different reviewer configurations
- Identifying which reviewers consume the most resources

## Results Aggregation and Reporting

Combine findings from all reviewers into a unified report:

```go
func generateReport(results []ReviewResult) {
    // Group findings by severity
    bySeverity := map[string][]Finding{
        "critical": {},
        "high":     {},
        "medium":   {},
        "low":      {},
    }

    for _, r := range results {
        if r.Err != nil {
            fmt.Printf("Reviewer %s failed: %v\n", r.Reviewer, r.Err)
            continue
        }

        for _, f := range r.Review.Findings {
            f.Description = fmt.Sprintf("[%s] %s", r.Reviewer, f.Description)
            bySeverity[f.Severity] = append(bySeverity[f.Severity], f)
        }
    }

    // Print findings in severity order
    for _, severity := range []string{"critical", "high", "medium", "low"} {
        findings := bySeverity[severity]
        if len(findings) == 0 {
            continue
        }

        fmt.Printf("\n=== %s (%d) ===\n", strings.ToUpper(severity), len(findings))
        for _, f := range findings {
            fmt.Printf("%s:%d - %s\n", f.File, f.Line, f.Description)
            fmt.Printf("  Suggestion: %s\n\n", f.Suggestion)
        }
    }
}
```

## Complete Working Example

This example brings together all the concepts into a working program:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/signal"
    "strings"
    "sync"
    "syscall"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// Finding represents a single issue discovered during review.
type Finding struct {
    Severity    string `json:"severity" desc:"Issue severity: critical, high, medium, or low"`
    File        string `json:"file" desc:"Path to the file containing the issue"`
    Line        int    `json:"line" desc:"Line number where the issue occurs"`
    Description string `json:"description" desc:"Clear description of the problem"`
    Suggestion  string `json:"suggestion" desc:"Recommended fix or improvement"`
}

// Review represents the complete output from a specialist reviewer.
type Review struct {
    Reviewer string    `json:"reviewer" desc:"Name of the reviewer (security, database, performance)"`
    Findings []Finding `json:"findings" desc:"List of issues discovered"`
    Summary  string    `json:"summary" desc:"Brief overview of the review results"`
}

// ReviewerConfig defines a specialist reviewer.
type ReviewerConfig struct {
    Name   string
    Prompt string
}

// ReviewResult pairs a reviewer name with its output or error.
type ReviewResult struct {
    Reviewer string
    Review   *Review
    Err      error
    Cost     float64
    Duration time.Duration
}

var reviewers = []ReviewerConfig{
    {
        Name: "security",
        Prompt: `You are a security specialist reviewing code for vulnerabilities.
Focus on:
- SQL injection and command injection risks
- Authentication and authorization flaws
- Sensitive data exposure
- Input validation gaps
- Insecure cryptographic practices

Analyze the code and report findings in the required JSON format.`,
    },
    {
        Name: "database",
        Prompt: `You are a database specialist reviewing code for data access issues.
Focus on:
- N+1 query patterns
- Missing indexes on frequently queried columns
- Connection pool mismanagement
- Transaction handling errors
- Inefficient joins and subqueries

Analyze the code and report findings in the required JSON format.`,
    },
    {
        Name: "performance",
        Prompt: `You are a performance specialist reviewing code for bottlenecks.
Focus on:
- Unnecessary allocations in hot paths
- Blocking operations that could be async
- Algorithmic complexity issues
- Resource leaks (goroutines, file handles)
- Inefficient data structures

Analyze the code and report findings in the required JSON format.`,
    },
}

func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}

func run() error {
    if len(os.Args) < 2 {
        return fmt.Errorf("usage: %s <path-to-review>", os.Args[0])
    }
    targetPath := os.Args[1]

    ctx, cancel := signal.NotifyContext(
        context.Background(),
        syscall.SIGINT,
        syscall.SIGTERM,
    )
    defer cancel()

    fmt.Printf("Starting parallel code review of %s\n", targetPath)
    fmt.Printf("Reviewers: %d\n\n", len(reviewers))

    start := time.Now()
    results, err := runParallelReviews(ctx, targetPath)
    if err != nil {
        return err
    }
    totalDuration := time.Since(start)

    // Generate report
    generateReport(results)

    // Cost summary
    total, perReviewer := aggregateCosts(results)
    fmt.Println("\n=== Cost Summary ===")
    for reviewer, cost := range perReviewer {
        fmt.Printf("  %s: $%.4f\n", reviewer, cost)
    }
    fmt.Printf("  Total: $%.4f\n", total)
    fmt.Printf("  Wall time: %v\n", totalDuration)

    return nil
}

func runParallelReviews(ctx context.Context, targetPath string) ([]ReviewResult, error) {
    var wg sync.WaitGroup
    results := make(chan ReviewResult, len(reviewers))

    for _, cfg := range reviewers {
        wg.Add(1)
        go func(cfg ReviewerConfig) {
            defer wg.Done()
            result := runSingleReview(ctx, cfg, targetPath)
            results <- result
        }(cfg)
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    var allResults []ReviewResult
    for result := range results {
        allResults = append(allResults, result)
    }

    return allResults, nil
}

func runSingleReview(ctx context.Context, cfg ReviewerConfig, targetPath string) ReviewResult {
    start := time.Now()

    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.Tools("Read", "Glob", "Grep"),
        agent.MaxTurns(10),
        agent.WithSchema(Review{}),
        agent.SystemPromptAppend(cfg.Prompt),
    )
    if err != nil {
        return ReviewResult{
            Reviewer: cfg.Name,
            Err:      fmt.Errorf("failed to create agent: %w", err),
        }
    }
    defer a.Close()

    prompt := fmt.Sprintf(`Review the code in %s for issues related to your specialty.
Return your findings as JSON matching the required schema.
Set the "reviewer" field to "%s".`, targetPath, cfg.Name)

    var review Review
    result, err := a.RunWithSchema(ctx, prompt, &review)
    if err != nil {
        // Attempt fallback: parse JSON from result text
        if result != nil && result.ResultText != "" {
            if parseErr := json.Unmarshal([]byte(result.ResultText), &review); parseErr == nil {
                return ReviewResult{
                    Reviewer: cfg.Name,
                    Review:   &review,
                    Cost:     result.CostUSD,
                    Duration: time.Since(start),
                }
            }
        }
        return ReviewResult{
            Reviewer: cfg.Name,
            Err:      fmt.Errorf("review failed: %w", err),
        }
    }

    return ReviewResult{
        Reviewer: cfg.Name,
        Review:   &review,
        Cost:     result.CostUSD,
        Duration: time.Since(start),
    }
}

func aggregateCosts(results []ReviewResult) (total float64, perReviewer map[string]float64) {
    perReviewer = make(map[string]float64)
    for _, r := range results {
        perReviewer[r.Reviewer] = r.Cost
        total += r.Cost
    }
    return total, perReviewer
}

func generateReport(results []ReviewResult) {
    bySeverity := map[string][]Finding{
        "critical": {},
        "high":     {},
        "medium":   {},
        "low":      {},
    }

    for _, r := range results {
        if r.Err != nil {
            fmt.Printf("Reviewer %s failed: %v\n", r.Reviewer, r.Err)
            continue
        }

        fmt.Printf("Reviewer %s completed in %v\n", r.Reviewer, r.Duration)
        fmt.Printf("  Summary: %s\n", r.Review.Summary)
        fmt.Printf("  Findings: %d\n\n", len(r.Review.Findings))

        for _, f := range r.Review.Findings {
            tagged := Finding{
                Severity:    f.Severity,
                File:        f.File,
                Line:        f.Line,
                Description: fmt.Sprintf("[%s] %s", r.Reviewer, f.Description),
                Suggestion:  f.Suggestion,
            }
            bySeverity[f.Severity] = append(bySeverity[f.Severity], tagged)
        }
    }

    fmt.Println("\n=== Aggregated Findings ===")
    for _, severity := range []string{"critical", "high", "medium", "low"} {
        findings := bySeverity[severity]
        if len(findings) == 0 {
            continue
        }

        fmt.Printf("\n--- %s (%d) ---\n", strings.ToUpper(severity), len(findings))
        for _, f := range findings {
            fmt.Printf("%s:%d - %s\n", f.File, f.Line, f.Description)
            fmt.Printf("  Suggestion: %s\n\n", f.Suggestion)
        }
    }
}
```

## Running the Example

Build and run the review orchestrator:

```bash
go run main.go ./internal/
```

Expected output:

```
Starting parallel code review of ./internal/
Reviewers: 3

Reviewer performance completed in 8.234s
  Summary: Found 2 performance concerns in the request handling code.
  Findings: 2

Reviewer security completed in 9.123s
  Summary: No critical security issues found. Minor improvements suggested.
  Findings: 1

Reviewer database completed in 7.891s
  Summary: Identified N+1 query pattern in user listing endpoint.
  Findings: 3

=== Aggregated Findings ===

--- HIGH (2) ---
internal/api/handler.go:45 - [database] N+1 query pattern loading user preferences
  Suggestion: Use eager loading or batch the preference queries

internal/api/handler.go:78 - [performance] Allocation inside hot loop
  Suggestion: Move buffer allocation outside the loop

--- MEDIUM (3) ---
...

=== Cost Summary ===
  security: $0.0234
  database: $0.0198
  performance: $0.0212
  Total: $0.0644
  Wall time: 9.456s
```

## Coming Soon: Fork and Session Management

Future SDK versions will include additional features for review orchestration.

### Fork Method

The `Fork` method will create child sessions that share context with the parent but maintain independent state:

```go
// Not yet implemented - planned API
child, err := parent.Fork(ctx)
if err != nil {
    return err
}
defer child.Close()

// Child has access to parent's conversation history
// but makes independent changes
result, err := child.Run(ctx, "Given the security issues found, suggest fixes")
```

Forking will enable scenarios such as:

- Running follow-up analysis based on initial findings
- Trying different fix approaches without affecting the original review
- Creating specialized sub-reviews that build on general findings

### OnStop Hook for Cleanup

The `OnStop` hook receives notification when an agent session ends:

```go
a, _ := agent.New(ctx,
    agent.OnStop(func(e *agent.StopEvent) {
        log.Printf("Reviewer %s completed: %d turns, $%.4f",
            e.SessionID, e.NumTurns, e.CostUSD)
        metricsReporter.RecordReview(e)
    }),
)
```

This hook is useful for:

- Recording metrics about review duration and cost
- Triggering downstream actions when reviews complete
- Cleaning up resources associated with the review

## Key Concepts Summary

This tutorial covered several important patterns:

| Concept                  | Description                                                   |
|--------------------------|---------------------------------------------------------------|
| Goroutine-per-agent      | Each agent runs in its own goroutine for parallel execution   |
| Channel-based collection | Results flow through a buffered channel to the main goroutine |
| Structured output        | JSON schemas ensure consistent, parseable review findings     |
| Read-only enforcement    | Tools and hooks restrict reviewers to analysis only           |
| Cost aggregation         | Track spending across multiple concurrent agents              |
| Graceful cancellation    | Context cancellation propagates to all running agents         |

## Exercises

1. **Add a new reviewer**: Create an accessibility reviewer that checks for ARIA attributes and keyboard navigation.

2. **Implement severity thresholds**: Modify the program to exit with a non-zero status if any critical findings are
   present.

3. **Add retry logic**: Implement exponential backoff retry for reviewers that fail due to transient errors.

4. **Output to JSON**: Add an option to output the aggregated report as JSON for integration with other tools.

5. **Limit concurrency**: Modify `runParallelReviews` to limit the number of concurrent reviewers using a semaphore
   pattern.

## Summary

This tutorial demonstrated:

- Running multiple agents in parallel using goroutines
- Collecting results through channels
- Using structured output schemas for consistent findings
- Enforcing read-only behavior through tool selection and hooks
- Aggregating costs and findings across reviewers
- Handling cancellation gracefully

The parallel specialist review pattern scales to many different use cases beyond code review, including document
analysis, data validation, and multi-perspective content generation. The key insight is that independent analysis tasks
map naturally to concurrent agent execution.
