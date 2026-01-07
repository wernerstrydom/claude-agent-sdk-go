# Tutorial Series: Building a TODO Application

This tutorial series teaches the Claude Agent SDK through progressive development of a TODO web application. Each
tutorial introduces new SDK features while building functional components.

## Why a TODO Application?

A TODO application serves as an effective teaching vehicle for several reasons:

1. **Familiar domain** - The concept requires no domain-specific knowledge
2. **Progressive complexity** - Features can be added incrementally
3. **Real-world patterns** - CRUD operations, persistence, and APIs appear in most applications
4. **Testable components** - Each feature has clear success criteria

The application evolves from a simple CLI tool to a full web service with database persistence.

## Series Overview

The tutorials progress through SDK features in order of complexity.

### Part 1: Oneshot Prompt

Create an agent that generates a TODO application with a single prompt.

**SDK Features Covered:**

- `agent.New()` - Creating an agent
- `agent.Stream()` - Receiving incremental output
- `agent.Close()` - Releasing resources
- `agent.Model()` - Selecting a Claude model
- Message types: `Text`, `ToolUse`, `ToolResult`, `Result`, `Error`

**Application Outcome:**
A working TODO web application generated in a single interaction.

[Start Tutorial 01](01-oneshot-prompt/README.md)

### Part 2: Plan and Implement

Separate planning from implementation using security hooks.

**SDK Features Covered:**

- `agent.PreToolUse()` - Hook registration
- `agent.DenyCommands()` - Blocking dangerous commands
- `agent.AllowPaths()` - Restricting file access
- `agent.DenyPaths()` - Blocking sensitive directories
- `agent.Decision` values - Allow, Deny, Continue
- Custom hook creation

**Application Outcome:**
A two-phase workflow where planning produces a specification without side effects, and implementation executes within
defined security boundaries.

[Start Tutorial 02](02-plan-and-implement/README.md)

### Part 3: Task Decomposition with Structured Output

Use structured output to decompose work into tasks and execute them sequentially.

**SDK Features Covered:**

- `agent.WithSchema()` - Schema from Go types
- `agent.RunWithSchema()` - Structured responses
- `agent.RunStructured()` - One-shot structured queries
- `agent.Audit()` - Custom audit handlers
- `agent.AuditToFile()` - File-based logging
- `agent.AuditEvent` - Event structure
- `agent.PostToolUse()` - Post-execution observation
- Cost aggregation across multiple agents
- Fresh context per task (isolation)
- Dependency ordering

**Application Outcome:**
Automated task decomposition with structured project plans, sequential execution, and comprehensive audit logging.

[Start Tutorial 03](03-json-task-decomposition/README.md)

### Part 4: Driver-Based Architecture

Apply the database/sql driver pattern to create pluggable database backends for the TODO application.

**SDK Features Covered:**

- `agent.Subagent()` - Subagent configuration
- `agent.SubagentDescription()` - Task definition
- `agent.CustomTool()` - Registering tools
- `agent.NewFuncTool()` - Function-based tools
- `agent.Skill()` - Inline skills
- Driver interface design
- Registry pattern

**Application Outcome:**
A TODO application supporting multiple database backends (SQLite, PostgreSQL) via a unified driver interface.

[Start Tutorial 04](04-driver-architecture/README.md)

### Part 5: Review Orchestration

Run multiple specialist agents in parallel for concurrent code review.

**SDK Features Covered:**

- Goroutine-per-agent pattern
- Parallel agent execution
- Channel-based result aggregation
- Structured output for review results
- Context cancellation across agents
- Cost aggregation

**Application Outcome:**
A parallel code review system with security, database, and performance specialists running concurrently.

[Start Tutorial 05](05-review-orchestration/README.md)

### Part 6: Self-Improving Agent System

Build an agent system that learns from its outputs and accumulates domain knowledge.

**SDK Features Covered:**

- `agent.Audit()` - Session capture via audit handlers
- `agent.PostToolUse()` - Tool execution observation
- `agent.RunStructured()` - Structured lesson extraction
- `agent.SkillsDir()` - Loading skills from files
- `agent.Skill()` - Inline skill injection
- `agent.MaxTurns()` - Session limits
- `agent.Resume()` - Session continuity
- `agent.Fork()` - Session branching
- `agent.PreCompact()` - Context management hooks

**Application Outcome:**
A self-improvement loop that captures sessions, extracts lessons, generates skills, and loads them in future sessions.

[Start Tutorial 06](06-self-improvement/README.md)

## Prerequisites

Before starting the tutorials, ensure you have:

1. Completed the [Installation](../getting-started/installation.md) guide
2. Go 1.21 or later installed
3. Claude Code CLI installed and authenticated
4. Basic familiarity with Go programming

## Project Structure

Each tutorial builds on the previous one. The final project structure looks like:

```
todo-app/
├── go.mod
├── go.sum
├── main.go
├── internal/
│   ├── todo/
│   │   ├── model.go
│   │   ├── store.go
│   │   └── store_test.go
│   ├── agent/
│   │   ├── tools.go
│   │   ├── hooks.go
│   │   └── skills/
│   │       └── todo.skill.md
│   └── api/
│       ├── handler.go
│       └── handler_test.go
├── cmd/
│   └── server/
│       └── main.go
└── audit/
    └── .gitkeep
```

## Learning Approach

Each tutorial follows a consistent structure:

1. **Objective** - What we will build
2. **Concepts** - SDK features explained with context
3. **Implementation** - Step-by-step code development
4. **Verification** - Testing that the code works
5. **Exercises** - Optional challenges to reinforce learning

Code is introduced incrementally. Each tutorial starts with the completed code from the previous tutorial.

## Getting Started

Begin with [Part 1: Oneshot Prompt](01-oneshot-prompt/README.md).

If you encounter issues, refer to the [Installation](../getting-started/installation.md) guide or the
main [Documentation](../README.md).
