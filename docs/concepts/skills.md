# Skills

Skills are markdown documents that inject domain knowledge, coding standards, or behavioral guidelines into Claude's
context. Unlike the system prompt which defines Claude's fundamental behavior, skills provide specialized knowledge for
specific tasks or domains.

## Why Skills Exist

Claude operates with general knowledge but lacks context about your specific:

- Project conventions and coding standards
- Domain terminology and workflows
- Organizational policies and constraints
- Preferred patterns and anti-patterns

Skills address this by loading relevant documentation into Claude's context before processing prompts. This grounds
Claude's responses in your specific requirements without modifying the underlying system prompt.

### Healthcare Domain Example

A clinical application might define skills for:

- **HIPAA Compliance** - Data handling requirements for protected health information
- **Clinical Documentation** - Standards for medical note formatting
- **ICD-10 Coding** - Guidelines for diagnosis code selection
- **Medication Safety** - Rules for drug interaction checking

Each skill provides domain expertise that Claude applies when relevant.

## Inline Skills

The `Skill` option defines a skill directly in code:

```go
hipaaSkill := `# HIPAA Compliance Guidelines

When handling patient data:

1. Never include full SSN, only last 4 digits if necessary
2. De-identify data by default - use patient IDs instead of names
3. Avoid logging PHI (Protected Health Information)
4. Encrypt data at rest and in transit

Violations to flag:
- Direct storage of patient names in logs
- Unencrypted file writes containing diagnoses
- API responses exposing full patient records`

a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.Skill("hipaa", hipaaSkill),
)
```

Claude incorporates this knowledge when processing prompts, flagging potential compliance issues and following the
specified guidelines.

## Skills from Files

For larger skill definitions, load from markdown files using `SkillsDir`:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.SkillsDir("./skills"),
)
```

The SDK recognizes two naming conventions:

### Convention 1: Named Directory with SKILL.md

```
skills/
  hipaa/
    SKILL.md      <- Skill named "hipaa"
  coding-standards/
    SKILL.md      <- Skill named "coding-standards"
```

The directory name becomes the skill name.

### Convention 2: Suffix Naming

```
skills/
  hipaa.skill.md              <- Skill named "hipaa"
  coding-standards.skill.md   <- Skill named "coding-standards"
```

The filename (minus `.skill.md`) becomes the skill name.

Both conventions can coexist in the same directory.

## Skill File Format

Skill files are markdown documents. Use headers, lists, and code blocks to structure the content clearly:

```markdown
# Go Development Standards

## Error Handling

Always handle errors explicitly. Do not ignore errors with blank identifiers.

Correct:
```go
result, err := doSomething()
if err != nil {
    return fmt.Errorf("doSomething failed: %w", err)
}
```

Incorrect:

```go
result, _ := doSomething()  // Never do this
```

## Naming Conventions

- Use camelCase for unexported identifiers
- Use PascalCase for exported identifiers
- Acronyms should be all caps: `HTTPServer`, `userID`

## Testing Requirements

- Table-driven tests for functions with multiple cases
- Test file naming: `*_test.go` in same package
- Minimum coverage: 80% for new code

```

## Combining Multiple Skills

Load skills from multiple sources by combining options:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    // Inline skill for project-specific rules
    agent.Skill("project", projectGuidelines),
    // Directory of shared skills
    agent.SkillsDir("./skills"),
    // Another directory for team-specific skills
    agent.SkillsDir("/shared/team-skills"),
)
```

All skills load into Claude's context together.

## System Prompt Customization

Beyond skills, you can customize Claude's system prompt directly.

### Preset System Prompts

The `SystemPromptPreset` option selects a predefined persona:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.SystemPromptPreset("code-review"),
)
```

Presets provide optimized instructions for specific use cases.

### Appending to System Prompt

The `SystemPromptAppend` option adds text to the end of the system prompt without replacing it:

```go
a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.SystemPromptAppend(`
Always explain your reasoning before providing code.
When suggesting changes, explain the tradeoffs.
If you're uncertain, say so.
`),
)
```

This preserves Claude's default behaviors while adding custom instructions.

## Skills vs System Prompt Append

Choose the appropriate mechanism based on the content type:

| Content Type            | Use Skills | Use SystemPromptAppend |
|-------------------------|------------|------------------------|
| Domain knowledge        | Yes        | No                     |
| Coding standards        | Yes        | No                     |
| Behavioral rules        | No         | Yes                    |
| Project conventions     | Yes        | No                     |
| Response formatting     | No         | Yes                    |
| Reference documentation | Yes        | No                     |

Skills work best for factual information Claude should reference. System prompt append works best for behavioral
instructions.

## Use Cases

### Coding Standards Enforcement

```go
agent.Skill("go-style", `# Go Style Guide

## Import Organization
1. Standard library imports
2. External dependencies
3. Internal packages

Separate groups with blank lines.

## Function Signatures
- Context first: func DoThing(ctx context.Context, ...)
- Options last: func New(ctx context.Context, opts ...Option)
- Return errors last: func Read() ([]byte, error)
`)
```

Claude applies these standards when generating or reviewing code.

### Domain Terminology

```go
agent.Skill("medical-terms", `# Medical Terminology

## Abbreviations
- Dx: Diagnosis
- Rx: Prescription
- Hx: History
- Sx: Symptoms
- Tx: Treatment

## Common Acronyms
- EMR: Electronic Medical Record
- PHI: Protected Health Information
- CPT: Current Procedural Terminology
- ICD: International Classification of Diseases
`)
```

Claude uses correct terminology in medical contexts.

### Security Policies

```go
agent.Skill("security", `# Security Requirements

## Forbidden Operations
- No hardcoded credentials
- No eval() or equivalent dynamic execution
- No SQL string concatenation (use parameterized queries)

## Required Patterns
- Input validation on all user data
- Output encoding for HTML contexts
- HTTPS for all external connections
`)
```

Claude checks for and avoids security anti-patterns.

## SkillConfig Structure

Internally, skills use the following structure:

```go
type SkillConfig struct {
    Name    string // Skill name (key for lookup)
    Content string // Markdown content
}
```

Skills are loaded at agent creation time and remain in context for the session.

## Limitations

- Skills increase context token usage; balance detail against token limits
- Very long skills may be compressed during context window compaction
- Skills cannot be added or removed mid-session
- Skill names must be unique; duplicate names overwrite earlier definitions
