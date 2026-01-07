# Usage Examples

Practical examples showing how the SDK reduces developer toil. Each example is a complete, runnable Go program.

| Example | Description |
|---------|-------------|
| [Plan Loop](plan-loop.md) | Ralph-style iterate-until-done pattern: read a JSON plan, implement each item, review in a separate pass |
| [Batch Generation](batch-generation.md) | Generate the same program in 20 languages concurrently using goroutines and channels |
| [Driver Scaffolding](driver-scaffolding.md) | Scaffold `Store` interface implementations for MySQL, MongoDB, Azure Table Storage, and more |
| [Repository Maintenance](repository-maintenance.md) | Iterate through repositories checking for dependency updates, language migrations, and missing capabilities |

## Common Themes

These examples share a few patterns:

- **Loop over a list** -- languages, backends, repos, plan items. The SDK turns `claude -p` into `a.Run()` inside a `for` loop.
- **Structured data in, structured data out** -- Go structs for plans, results, and reports instead of string parsing.
- **Concurrency where it helps** -- goroutines and channels for independent tasks, sequential loops when order matters.
- **Cost tracking** -- `result.CostUSD` on every response, accumulated across runs.

The goal in each case is to reduce toil: tasks that are repetitive, predictable, and low-judgment but still time-consuming when done by hand.
