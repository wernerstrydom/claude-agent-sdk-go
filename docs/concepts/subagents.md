# Subagents

Subagents are child agents that the parent agent can spawn to handle autonomous tasks. When Claude determines that a
task would benefit from delegation, it uses the Task tool to create a subagent that runs independently with its own
configuration, model, and tool set.

## Why Subagents Exist

Complex tasks often decompose into subtasks with different characteristics. Consider a code refactoring task:

1. Understanding the current code structure (requires reading many files)
2. Running tests to verify current behavior (requires Bash access)
3. Making changes (requires file editing)
4. Verifying changes work (requires running tests again)

Each subtask has different tool requirements and different cost profiles. A fast, inexpensive model can run tests
repeatedly, while a more capable model handles the nuanced refactoring decisions.

Subagents enable this decomposition by:

- Running autonomously without blocking the parent
- Using different models optimized for their specific task
- Operating with restricted tool sets for safety
- Accumulating their own cost and turn metrics

### Healthcare Domain Example

A clinical decision support system might use subagents to:

1. **Literature Search Agent** - Searches medical databases for relevant studies
2. **Drug Interaction Checker** - Validates medication combinations
3. **Documentation Agent** - Formats findings into clinical note format

Each subagent specializes in one concern, using tools and models appropriate to that task.

## Configuring Subagents

The `Subagent` option registers a named subagent configuration. Claude can then spawn this subagent using the Task tool.

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.Subagent("test-runner",
        agent.SubagentDescription("Runs test suites and reports failures"),
        agent.SubagentPrompt("You are a test execution agent. Run the specified tests and report results concisely."),
        agent.SubagentTools("Bash", "Read"),
        agent.SubagentModel("claude-haiku-3"),
    ),
    agent.Subagent("code-reviewer",
        agent.SubagentDescription("Reviews code changes for issues"),
        agent.SubagentPrompt("You are a code review agent. Analyze the code for bugs, security issues, and style problems."),
        agent.SubagentTools("Read", "Glob", "Grep"),
    ),
)
```

### Configuration Options

Each subagent accepts the following options:

| Option                | Purpose                                                                                                           |
|-----------------------|-------------------------------------------------------------------------------------------------------------------|
| `SubagentDescription` | Explains to Claude when to use this subagent. Claude reads this description to decide which subagent fits a task. |
| `SubagentPrompt`      | System prompt or instructions specific to the subagent. Defines the subagent's role and behavior.                 |
| `SubagentTools`       | Tools available to the subagent. Defaults to inheriting from the parent if not specified.                         |
| `SubagentModel`       | Model override. Use "haiku" for fast/cheap tasks, leave empty to inherit the parent model.                        |

### SubagentConfig Structure

Internally, subagent configuration uses the following structure:

```go
type SubagentConfig struct {
    Name        string   // Subagent name (key for Task tool)
    Description string   // Description for Claude's selection
    Prompt      string   // System prompt or instructions
    Tools       []string // Available tools
    Model       string   // Model override (empty = inherit)
}
```

## How Claude Spawns Subagents

When the parent agent determines a subagent should handle a task, it invokes the Task tool with:

1. The subagent name (matching a configured subagent)
2. The task description to pass to the subagent

The subagent runs autonomously, and its result returns to the parent agent when complete.

```
Parent Agent
    |
    +-- Task("test-runner", "Run all tests in pkg/auth")
    |       |
    |       +-- Subagent executes tests
    |       +-- Subagent returns results
    |
    +-- Parent continues with test results
```

## Context Inheritance

Subagents inherit certain context from their parent:

- **Working directory** - Subagents operate in the same directory
- **Environment variables** - Passed through from the parent process
- **Tool availability** - Constrained by SubagentTools configuration

Subagents do not inherit:

- **Conversation history** - Each subagent starts fresh with only the task description
- **Session ID** - Subagents receive their own session identifier
- **Hooks** - Parent hooks do not apply to subagent tool invocations

## Observing Subagent Execution

The `SubagentStop` hook notifies when a subagent completes:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.Subagent("test-runner",
        agent.SubagentDescription("Runs tests"),
        agent.SubagentTools("Bash", "Read"),
    ),
    agent.SubagentStop(func(e *agent.SubagentStopEvent) {
        log.Printf("Subagent %s (%s) completed: %d turns, $%.4f",
            e.SubagentID, e.SubagentType, e.NumTurns, e.CostUSD)
    }),
)
```

The `SubagentStopEvent` provides:

| Field             | Description                                     |
|-------------------|-------------------------------------------------|
| `SessionID`       | Parent session identifier                       |
| `SubagentID`      | Unique identifier for this subagent instance    |
| `SubagentType`    | Type of subagent (e.g., "Task")                 |
| `ParentToolUseID` | Links to the tool_use that spawned the subagent |
| `NumTurns`        | Number of turns the subagent took               |
| `CostUSD`         | Cost incurred by the subagent                   |

## Use Cases

### Parallel Task Execution

Subagents enable concurrent work on independent tasks:

```go
agent.Subagent("linter",
    agent.SubagentDescription("Runs code linting and returns issues"),
    agent.SubagentTools("Bash", "Read"),
    agent.SubagentModel("claude-haiku-3"),
),
agent.Subagent("type-checker",
    agent.SubagentDescription("Runs type checking and returns errors"),
    agent.SubagentTools("Bash", "Read"),
    agent.SubagentModel("claude-haiku-3"),
),
```

Claude can spawn both subagents simultaneously when asked to validate code quality.

### Cost Optimization

Delegate simple tasks to cheaper models:

```go
agent.Subagent("file-finder",
    agent.SubagentDescription("Locates files matching patterns"),
    agent.SubagentTools("Glob", "Grep", "Read"),
    agent.SubagentModel("claude-haiku-3"),  // Fast, inexpensive
),
```

Reserve expensive models for tasks requiring deeper reasoning.

### Tool Isolation

Restrict subagent capabilities for safety:

```go
agent.Subagent("reader",
    agent.SubagentDescription("Reads and summarizes file contents"),
    agent.SubagentTools("Read", "Glob"),  // No write or execute
),
```

The subagent cannot modify files or execute commands, limiting potential harm.

## Limitations

- Subagents cannot spawn their own subagents (no recursive delegation)
- Parent hooks do not intercept subagent tool calls
- Subagent conversation context is isolated from the parent
- The parent waits for subagent completion (no true parallel execution in a single Run call)
