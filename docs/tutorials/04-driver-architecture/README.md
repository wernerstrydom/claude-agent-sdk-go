# Tutorial 4: Driver-Based Architecture

This tutorial demonstrates a driver-based architecture for database operations using the Claude Agent SDK. The pattern
mirrors Go's `database/sql` design, where drivers register themselves and a unified interface handles all interactions.

## Table of Contents

- [Introduction](#introduction)
- [The database/sql Pattern](#the-databasesql-pattern)
- [Driver Interface Design](#driver-interface-design)
- [Implementing Storage Drivers](#implementing-storage-drivers)
- [Driver Registry](#driver-registry)
- [Orchestrating with the Agent](#orchestrating-with-the-agent)
- [Subagents](#subagents)
- [Custom Tools](#custom-tools)
- [Skills](#skills)
- [Complete Working Example](#complete-working-example)
- [Key Concepts](#key-concepts)

## Introduction

Applications often need to support multiple database backends. A patient records system might store data in SQLite
during development and PostgreSQL in production. Rather than writing separate agent configurations for each database, a
driver-based architecture provides a single interface with pluggable implementations.

This pattern offers several benefits:

1. **Pluggable backends**: Add new database types without modifying core logic
2. **Consistent interface**: All drivers expose the same methods
3. **Domain-specific prompts**: Each driver provides specialized instructions for its database type
4. **Testability**: Swap drivers for testing without changing application code

## The database/sql Pattern

Go's `database/sql` package exemplifies this pattern. Applications import a database driver, which registers itself
during `init()`. The application then uses a generic `sql.Open()` function that selects the appropriate driver by name.

```go
import (
    "database/sql"
    _ "github.com/lib/pq"           // PostgreSQL driver registers itself
    _ "github.com/mattn/go-sqlite3" // SQLite driver registers itself
)

func main() {
    // Same interface, different backend
    db, err := sql.Open("postgres", connectionString)
    // or
    db, err := sql.Open("sqlite3", ":memory:")
}
```

The application code remains identical regardless of which database it connects to. This decoupling allows configuration
to determine behavior rather than code changes.

## Driver Interface Design

For an agent-based system, drivers must provide:

1. A name for registration and lookup
2. Database-specific instructions (a prompt) that guide Claude's behavior
3. Configuration validation to catch errors early

```go
// StorageDriver defines the interface for database backends.
// Each driver provides domain-specific knowledge that helps
// Claude interact correctly with that database type.
type StorageDriver interface {
    // Name returns the driver identifier used during registration.
    // Examples: "sqlite", "postgres", "mysql"
    Name() string

    // Prompt returns database-specific instructions for Claude.
    // This prompt is appended to the agent's system prompt and
    // contains knowledge about SQL dialects, connection patterns,
    // and database-specific features.
    Prompt() string

    // Validate checks the configuration before agent creation.
    // Returns an error if required settings are missing or invalid.
    // The config map contains driver-specific key-value pairs.
    Validate(config map[string]string) error
}
```

The `Prompt()` method is the key differentiator. Each database has unique syntax, capabilities, and conventions. By
encoding this knowledge in driver prompts, Claude can generate correct queries without explicit instructions in every
request.

## Implementing Storage Drivers

### SQLite Driver

SQLite requires minimal configuration. A file path suffices for most operations. The prompt emphasizes local file
handling and SQLite-specific syntax.

```go
// SQLiteDriver handles SQLite database operations.
// SQLite stores data in local files and uses a simpler
// SQL dialect than network databases.
type SQLiteDriver struct{}

// Name returns "sqlite" for driver registration.
func (d *SQLiteDriver) Name() string {
    return "sqlite"
}

// Prompt returns SQLite-specific instructions for Claude.
// These instructions cover:
// - File-based storage patterns
// - SQLite SQL dialect differences
// - Type affinity behavior
// - Write-Ahead Logging (WAL) mode considerations
func (d *SQLiteDriver) Prompt() string {
    return `## SQLite Database Operations

You are working with a SQLite database. SQLite stores data in a single local file.

### Connection
The database file path is provided in the DSN configuration. Use this path directly.

### SQL Dialect Notes
- Use INTEGER PRIMARY KEY for auto-incrementing IDs (not SERIAL)
- TEXT type stores any string length (no VARCHAR limits needed)
- BLOB type for binary data
- No native BOOLEAN type; use INTEGER (0 or 1)
- Datetime stored as TEXT in ISO8601 format, INTEGER as Unix timestamp, or REAL as Julian day

### Example Schema (Patient Records)
CREATE TABLE patients (
    id INTEGER PRIMARY KEY,
    mrn TEXT NOT NULL UNIQUE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    date_of_birth TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

### Transactions
SQLite supports transactions. Wrap related operations in BEGIN/COMMIT:
BEGIN TRANSACTION;
-- operations
COMMIT;

### File Locking
SQLite uses file-level locking. Only one writer can operate at a time.
For concurrent access patterns, consider WAL mode:
PRAGMA journal_mode=WAL;
`
}

// Validate checks that the DSN (file path) is provided.
func (d *SQLiteDriver) Validate(config map[string]string) error {
    if config["dsn"] == "" {
        return fmt.Errorf("sqlite driver requires 'dsn' (database file path)")
    }
    return nil
}
```

### PostgreSQL Driver

PostgreSQL requires network configuration and supports more complex features. The prompt reflects these differences.

```go
// PostgreSQLDriver handles PostgreSQL database operations.
// PostgreSQL is a network database with advanced features
// like JSON types, arrays, and full-text search.
type PostgreSQLDriver struct{}

// Name returns "postgres" for driver registration.
func (d *PostgreSQLDriver) Name() string {
    return "postgres"
}

// Prompt returns PostgreSQL-specific instructions for Claude.
// These instructions cover:
// - Connection string format
// - PostgreSQL SQL dialect
// - Advanced type support
// - Schema management
func (d *PostgreSQLDriver) Prompt() string {
    return `## PostgreSQL Database Operations

You are working with a PostgreSQL database. PostgreSQL is a network database with advanced features.

### Connection
Connection string format: postgres://user:password@host:port/database
The DSN is provided in configuration. Use standard psql or SQL commands.

### SQL Dialect Notes
- Use SERIAL or BIGSERIAL for auto-incrementing IDs
- VARCHAR(n) for bounded strings, TEXT for unbounded
- BOOLEAN type available (true/false)
- TIMESTAMP WITH TIME ZONE for datetime with timezone awareness
- Native JSON and JSONB types for structured data
- Array types supported (e.g., INTEGER[], TEXT[])

### Example Schema (Patient Records)
CREATE TABLE patients (
    id SERIAL PRIMARY KEY,
    mrn VARCHAR(50) NOT NULL UNIQUE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    date_of_birth DATE NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_patients_mrn ON patients(mrn);
CREATE INDEX idx_patients_name ON patients(last_name, first_name);

### Transactions
PostgreSQL supports full ACID transactions:
BEGIN;
-- operations
COMMIT; -- or ROLLBACK;

### Schema Inspection
\dt - list tables
\d tablename - describe table
SELECT * FROM information_schema.columns WHERE table_name = 'tablename';
`
}

// Validate checks that required connection parameters are provided.
func (d *PostgreSQLDriver) Validate(config map[string]string) error {
    if config["dsn"] == "" {
        return fmt.Errorf("postgres driver requires 'dsn' (connection string)")
    }
    return nil
}
```

## Driver Registry

The registry manages driver registration and lookup. Drivers register themselves, and the application retrieves them by
name at runtime.

```go
package storage

import (
    "fmt"
    "sync"
)

// drivers holds all registered storage drivers.
// The map is protected by a mutex for concurrent access safety.
var (
    driversMu sync.RWMutex
    drivers   = make(map[string]StorageDriver)
)

// Register adds a driver to the registry.
// Call this during package init() or application startup.
// Panics if a driver with the same name is already registered.
func Register(driver StorageDriver) {
    driversMu.Lock()
    defer driversMu.Unlock()

    name := driver.Name()
    if _, exists := drivers[name]; exists {
        panic(fmt.Sprintf("storage: driver %q already registered", name))
    }
    drivers[name] = driver
}

// Get retrieves a driver by name.
// Returns an error if the driver is not registered.
func Get(name string) (StorageDriver, error) {
    driversMu.RLock()
    defer driversMu.RUnlock()

    driver, ok := drivers[name]
    if !ok {
        return nil, fmt.Errorf("storage: unknown driver %q", name)
    }
    return driver, nil
}

// List returns the names of all registered drivers.
func List() []string {
    driversMu.RLock()
    defer driversMu.RUnlock()

    names := make([]string, 0, len(drivers))
    for name := range drivers {
        names = append(names, name)
    }
    return names
}
```

Drivers register during initialization:

```go
package storage

func init() {
    Register(&SQLiteDriver{})
    Register(&PostgreSQLDriver{})
}
```

## Orchestrating with the Agent

With drivers defined and registered, the agent combines the driver's prompt with application-specific instructions.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"yourproject/storage"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Configuration determines which driver to use
	driverName := "sqlite" // Could come from config file, env var, etc.
	config := map[string]string{
		"dsn": "./patients.db",
	}

	// Retrieve the driver
	driver, err := storage.Get(driverName)
	if err != nil {
		log.Fatalf("Failed to get driver: %v", err)
	}

	// Validate configuration
	if err := driver.Validate(config); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create agent with driver-specific prompt
	a, err := agent.New(ctx,
		agent.Model("claude-sonnet-4-5"),
		agent.WorkDir("."),
		agent.Tools("Bash", "Read", "Write"),
		agent.SystemPromptAppend(driver.Prompt()),
	)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer func() { _ = a.Close() }()

	// The agent now has database-specific knowledge
	result, err := a.Run(ctx, `
        Create a patients table if it doesn't exist, then insert a sample patient:
        - MRN: PAT-001
        - First Name: Alice
        - Last Name: Johnson
        - Date of Birth: 1985-03-15

        After inserting, query and display all patients.
    `)
	if err != nil {
		log.Fatalf("Failed to run: %v", err)
	}

	fmt.Printf("Result: %s\n", result.ResultText)
	fmt.Printf("Cost: $%.4f\n", result.CostUSD)
}
```

## Subagents

> **Coming Soon**: Subagent configuration is defined in the SDK but runtime support for spawning subagents via the Task
> tool is not yet implemented. The following shows the planned API.

Subagents allow decomposition of complex tasks. A parent agent can spawn specialized child agents for specific
operations. In a healthcare context, separate subagents might handle patient lookup, lab results, or billing queries.

### Planned API

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.Tools("Bash", "Read", "Write", "Task"),

    // Define a subagent for patient queries
    agent.Subagent("patient-lookup",
        agent.SubagentDescription("Searches and retrieves patient records by MRN or name"),
        agent.SubagentPrompt(`You are a specialized agent for patient record lookup.
Given a patient identifier (MRN or name), search the database and return
the matching patient record(s). Always verify the MRN format before querying.`),
        agent.SubagentTools("Bash", "Read"),
        agent.SubagentModel("claude-haiku-4-5"), // Faster, cheaper for simple lookups
    ),

    // Define a subagent for schema migrations
    agent.Subagent("schema-migrator",
        agent.SubagentDescription("Handles database schema changes and migrations"),
        agent.SubagentPrompt(`You are a specialized agent for database schema management.
Review proposed schema changes for safety, create migration scripts with rollback
procedures, and execute migrations in the correct order.`),
        agent.SubagentTools("Bash", "Read", "Write"),
        agent.SubagentModel("claude-sonnet-4-5"), // More capable for complex reasoning
    ),
)
```

When subagents are fully supported, the parent agent can invoke them via the Task tool. Claude decides when to delegate
based on the task description and subagent descriptions.

## Custom Tools

> **Coming Soon**: Custom tool execution is defined in the SDK but runtime interception of tool calls for in-process
> execution is not yet implemented. The following shows the planned API.

Custom tools allow Go functions to be called directly by Claude. This bridges the agent's capabilities with your
application's business logic.

### Planned API

```go
// Create a custom tool for driver switching
switchDriverTool := agent.NewFuncTool(
    "switch_database_driver",
    "Switches to a different database driver. Use when the user requests a database type change.",
    map[string]any{
        "type": "object",
        "properties": map[string]any{
            "driver_name": map[string]any{
                "type":        "string",
                "description": "Name of the driver to switch to (sqlite, postgres)",
                "enum":        []string{"sqlite", "postgres"},
            },
            "dsn": map[string]any{
                "type":        "string",
                "description": "Connection string or file path for the new driver",
            },
        },
        "required": []string{"driver_name", "dsn"},
    },
    func(ctx context.Context, input map[string]any) (any, error) {
        driverName := input["driver_name"].(string)
        dsn := input["dsn"].(string)

        driver, err := storage.Get(driverName)
        if err != nil {
            return nil, err
        }

        config := map[string]string{"dsn": dsn}
        if err := driver.Validate(config); err != nil {
            return nil, err
        }

        return map[string]any{
            "status":  "switched",
            "driver":  driverName,
            "prompt":  driver.Prompt(),
        }, nil
    },
)

a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.CustomTool(switchDriverTool),
)
```

When custom tools are fully supported, Claude can invoke them during execution. The SDK intercepts the tool call,
executes your Go function, and returns the result to Claude.

## Skills

> **Coming Soon**: Skill loading is defined in the SDK but integration with Claude's skill system is not yet fully
> implemented. The following shows the planned API.

Skills provide domain knowledge without modifying the system prompt directly. They load markdown instructions that
Claude can reference during execution.

### Planned API

Inline skills provide knowledge directly:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.Skill("healthcare-compliance", `# Healthcare Data Compliance

## HIPAA Requirements
- Always encrypt patient data at rest and in transit
- Log all access to patient records
- Implement minimum necessary access principle

## Audit Requirements
- Record timestamp, user, and action for all data access
- Maintain audit logs for minimum 6 years
- Include before/after values for modifications

## Data Retention
- Patient records: retain indefinitely or per state law
- Audit logs: minimum 6 years
- Temporary data: purge within 24 hours
`),
)
```

Skills from directories load multiple skill files:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.SkillsDir("./skills/healthcare"),
    agent.SkillsDir("./skills/databases"),
)
```

Skill files use the naming convention `skillname.skill.md` or `SKILL.md` in a named directory.

## Complete Working Example

The following example works with the current SDK implementation. It demonstrates the driver pattern with prompt
composition, which functions today.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "sync"
    "time"

    "github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// StorageDriver defines the interface for database backends.
type StorageDriver interface {
    Name() string
    Prompt() string
    Validate(config map[string]string) error
}

// Driver registry
var (
    driversMu sync.RWMutex
    drivers   = make(map[string]StorageDriver)
)

func registerDriver(driver StorageDriver) {
    driversMu.Lock()
    defer driversMu.Unlock()
    drivers[driver.Name()] = driver
}

func getDriver(name string) (StorageDriver, error) {
    driversMu.RLock()
    defer driversMu.RUnlock()
    driver, ok := drivers[name]
    if !ok {
        return nil, fmt.Errorf("unknown driver: %s", name)
    }
    return driver, nil
}

// SQLiteDriver implementation
type SQLiteDriver struct{}

func (d *SQLiteDriver) Name() string { return "sqlite" }

func (d *SQLiteDriver) Prompt() string {
    return `## SQLite Database Operations

You are working with a SQLite database stored in a local file.

### SQL Dialect
- INTEGER PRIMARY KEY for auto-incrementing IDs
- TEXT for strings, INTEGER for numbers, REAL for floats
- Datetime as TEXT in ISO8601 format

### Example Patient Schema
CREATE TABLE IF NOT EXISTS patients (
    id INTEGER PRIMARY KEY,
    mrn TEXT NOT NULL UNIQUE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    date_of_birth TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

When executing SQL, use the sqlite3 command-line tool.
`
}

func (d *SQLiteDriver) Validate(config map[string]string) error {
    if config["dsn"] == "" {
        return fmt.Errorf("sqlite driver requires 'dsn' (database file path)")
    }
    return nil
}

// PostgreSQLDriver implementation
type PostgreSQLDriver struct{}

func (d *PostgreSQLDriver) Name() string { return "postgres" }

func (d *PostgreSQLDriver) Prompt() string {
    return `## PostgreSQL Database Operations

You are working with a PostgreSQL database.

### SQL Dialect
- SERIAL for auto-incrementing IDs
- VARCHAR(n) or TEXT for strings
- TIMESTAMP WITH TIME ZONE for datetime
- Native BOOLEAN type

### Example Patient Schema
CREATE TABLE IF NOT EXISTS patients (
    id SERIAL PRIMARY KEY,
    mrn VARCHAR(50) NOT NULL UNIQUE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    date_of_birth DATE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

When executing SQL, use the psql command-line tool.
`
}

func (d *PostgreSQLDriver) Validate(config map[string]string) error {
    if config["dsn"] == "" {
        return fmt.Errorf("postgres driver requires 'dsn' (connection string)")
    }
    return nil
}

func init() {
    registerDriver(&SQLiteDriver{})
    registerDriver(&PostgreSQLDriver{})
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    // Get driver from environment or default to sqlite
    driverName := os.Getenv("DB_DRIVER")
    if driverName == "" {
        driverName = "sqlite"
    }

    dsn := os.Getenv("DB_DSN")
    if dsn == "" && driverName == "sqlite" {
        dsn = "./patients.db"
    }

    config := map[string]string{"dsn": dsn}

    // Retrieve and validate driver
    driver, err := getDriver(driverName)
    if err != nil {
        log.Fatalf("Failed to get driver: %v", err)
    }

    if err := driver.Validate(config); err != nil {
        log.Fatalf("Invalid configuration: %v", err)
    }

    fmt.Printf("Using driver: %s\n", driver.Name())
    fmt.Printf("DSN: %s\n", dsn)

    // Build application-specific prompt
    appPrompt := fmt.Sprintf(`You are a healthcare data management assistant.

%s

Configuration:
- Database: %s
- DSN: %s

When creating or modifying patient records, always:
1. Verify the MRN format (should be PAT-XXX)
2. Validate date formats
3. Log the operation performed
`, driver.Prompt(), driver.Name(), dsn)

    // Create agent with combined prompts
    a, err := agent.New(ctx,
        agent.Model("claude-sonnet-4-5"),
        agent.WorkDir("."),
        agent.Tools("Bash", "Read", "Write", "Edit"),
        agent.SystemPromptAppend(appPrompt),
        agent.PreToolUse(
            agent.DenyCommands("rm -rf", "sudo"),
        ),
    )
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }
    defer func() { _ = a.Close() }()

    // Run a database operation
    result, err := a.Run(ctx, `
        1. Create the patients table if it doesn't exist
        2. Insert a test patient: MRN=PAT-001, Name=Alice Johnson, DOB=1985-03-15
        3. Query and display all patients
        4. Report what you did
    `)
    if err != nil {
        log.Fatalf("Failed to run: %v", err)
    }

    fmt.Println("\n--- Result ---")
    fmt.Println(result.ResultText)
    fmt.Printf("\nCost: $%.4f | Turns: %d\n", result.CostUSD, result.NumTurns)
}
```

To run with different drivers:

```bash
# SQLite (default)
go run main.go

# PostgreSQL
DB_DRIVER=postgres DB_DSN="postgres://user:pass@localhost/healthcare" go run main.go
```

## Key Concepts

### Separation of Concerns

The driver pattern separates three concerns:

1. **Interface definition**: `StorageDriver` specifies what drivers must provide
2. **Implementation**: Each driver encapsulates database-specific knowledge
3. **Orchestration**: The main application selects and configures drivers without knowing their internals

This separation allows each component to evolve independently.

### Plugin Architecture

New drivers can be added without modifying existing code:

1. Implement the `StorageDriver` interface
2. Register the driver during initialization
3. Users can now select the new driver by name

The registry pattern enables this extensibility while maintaining type safety.

### Domain-Specific Configuration

Each driver's `Prompt()` method encodes domain knowledge specific to that database:

- SQL dialect differences
- Type mappings
- Best practices
- Example schemas

This knowledge travels with the driver, ensuring Claude receives appropriate context regardless of which backend is
selected.

### Future Integration Points

When subagents, custom tools, and skills are fully implemented, this architecture extends naturally:

- **Subagents**: Specialized agents per driver (e.g., PostgreSQL admin subagent)
- **Custom tools**: Go functions that interact with driver state or switch drivers at runtime
- **Skills**: Compliance and best-practice knowledge loaded per domain

The driver pattern provides the foundation; these features will enhance its capabilities.
