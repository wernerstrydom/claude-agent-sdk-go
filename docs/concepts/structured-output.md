# Structured Output

Structured output allows the SDK to return responses in a predictable JSON format defined by a schema. This capability
transforms Claude from a conversational assistant into a reliable data extraction and transformation tool.

## Why Structured Output Matters

When automating tasks programmatically, free-form text responses require parsing and interpretation. This introduces
fragility into automation pipelines because:

1. Text formatting varies between responses
2. Extracting specific values requires heuristics or additional parsing logic
3. Unexpected response structures can break downstream processing

Structured output eliminates these problems by constraining Claude's responses to match a predefined JSON Schema. The
response is guaranteed to be valid JSON that conforms to the schema, enabling direct deserialization into Go types.

### Healthcare Domain Example

Consider a medical records processing system that extracts patient information from clinical notes. Without structured
output, extracting a diagnosis requires text parsing:

```
Patient presents with chest pain. Assessment: Suspected angina pectoris.
Recommend ECG and stress test. Follow-up in 2 weeks.
```

With structured output, the same information becomes machine-readable:

```json
{
  "diagnosis": "Suspected angina pectoris",
  "symptoms": ["chest pain"],
  "tests_ordered": ["ECG", "stress test"],
  "follow_up_days": 14
}
```

## API Overview

The SDK provides two approaches for defining schemas.

### Type-Based Schema Generation

The `WithSchema` option generates a JSON Schema from a Go struct type. This approach ensures type safety because the
schema derives directly from the type you will deserialize into.

```go
type DiagnosisExtraction struct {
    Diagnosis     string   `json:"diagnosis" desc:"Primary diagnosis from the clinical note"`
    Symptoms      []string `json:"symptoms" desc:"List of reported symptoms"`
    TestsOrdered  []string `json:"tests_ordered,omitempty" desc:"Ordered diagnostic tests"`
    FollowUpDays  *int     `json:"follow_up_days,omitempty" desc:"Days until follow-up appointment"`
}

a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.WithSchema(DiagnosisExtraction{}),
)
if err != nil {
    log.Fatal(err)
}
defer a.Close()

result, err := a.Run(ctx, "Extract diagnosis information from: " + clinicalNote)
```

The schema generator supports several Go types:

| Go Type              | JSON Schema Type                     |
|----------------------|--------------------------------------|
| `string`             | `string`                             |
| `int`, `int64`, etc. | `integer`                            |
| `float32`, `float64` | `number`                             |
| `bool`               | `boolean`                            |
| `[]T`                | `array`                              |
| `map[string]T`       | `object` with `additionalProperties` |
| Struct               | `object` with `properties`           |

### Struct Tags

The schema generator recognizes the following struct tags:

- `json:"name"` - Sets the JSON property name
- `json:"name,omitempty"` - Makes the field optional (not required)
- `desc:"description"` - Adds a description to help Claude understand the field's purpose

Pointer fields are treated as optional in the generated schema.

### Raw Schema Definition

When the Go type system cannot express your schema requirements, use `WithSchemaRaw` to provide a schema directly:

```go
schema := map[string]any{
    "type": "object",
    "properties": map[string]any{
        "icd10_code": map[string]any{
            "type":        "string",
            "pattern":     "^[A-Z][0-9]{2}(\\.[0-9]{1,2})?$",
            "description": "ICD-10 diagnosis code",
        },
        "confidence": map[string]any{
            "type":    "number",
            "minimum": 0,
            "maximum": 1,
        },
    },
    "required": []string{"icd10_code", "confidence"},
}

a, err := agent.New(ctx,
    agent.Model("claude-sonnet-4-5"),
    agent.WithSchemaRaw(schema),
)
```

This approach supports JSON Schema features that cannot be expressed through Go types, such as:

- Pattern validation for strings
- Numeric ranges with `minimum` and `maximum`
- Enumerated values with `enum`
- Conditional schemas with `oneOf`, `anyOf`, `allOf`

## Parsing Responses

When an agent is configured with a schema, the `Result.Text` field contains the JSON response. Parse it using standard
JSON unmarshaling:

```go
result, err := a.Run(ctx, prompt)
if err != nil {
    log.Fatal(err)
}

var diagnosis DiagnosisExtraction
if err := json.Unmarshal([]byte(result.Text), &diagnosis); err != nil {
    log.Fatal(err)
}

fmt.Printf("Diagnosis: %s\n", diagnosis.Diagnosis)
```

## Validation and Error Handling

The CLI validates responses against the provided schema. However, validation errors can still occur if:

1. The schema is malformed
2. The schema uses features not supported by the validator
3. Network or process errors interrupt the response

Handle these cases by checking for errors at each stage:

```go
// Schema generation errors surface in New()
a, err := agent.New(ctx, agent.WithSchema(InvalidType{}))
if err != nil {
    var schemaErr *agent.SchemaError
    if errors.As(err, &schemaErr) {
        log.Printf("Schema error for type %s: %s", schemaErr.Type, schemaErr.Reason)
        return
    }
    log.Fatal(err)
}
defer a.Close()

// Runtime errors surface in Run()
result, err := a.Run(ctx, prompt)
if err != nil {
    log.Fatal(err)
}

// JSON parsing errors occur during unmarshaling
var data MyType
if err := json.Unmarshal([]byte(result.Text), &data); err != nil {
    log.Printf("Failed to parse response: %v", err)
    log.Printf("Raw response: %s", result.Text)
    return
}
```

## Fallback Patterns Without Structured Output

When not using the `WithSchema` option, you can still obtain structured data by including formatting instructions in the
prompt. This approach provides less guarantees but works with any agent configuration.

```go
prompt := `Extract the following from the clinical note in JSON format:
- diagnosis (string)
- symptoms (array of strings)
- tests_ordered (array of strings, may be empty)

Clinical note: ` + clinicalNote

result, err := a.Run(ctx, prompt)
if err != nil {
    log.Fatal(err)
}

// Response may be wrapped in markdown code blocks
text := strings.TrimPrefix(result.Text, "```json\n")
text = strings.TrimSuffix(text, "\n```")

var data map[string]any
if err := json.Unmarshal([]byte(text), &data); err != nil {
    log.Printf("Failed to parse: %v", err)
}
```

This pattern is less reliable because Claude may include explanatory text, use different JSON formatting, or omit the
structure entirely. Use `WithSchema` for production automation.

## Limitations

The schema generator has the following constraints:

- Maximum nesting depth of 10 levels (prevents infinite recursion with circular types)
- Map keys must be strings (maps with non-string keys are not supported)
- Function, channel, and complex types are not supported
- Embedded struct fields are flattened into the parent schema

## Further Reading

- [JSON Schema Specification](https://json-schema.org/) - Complete documentation for JSON Schema
- [Go encoding/json Package](https://pkg.go.dev/encoding/json) - Go's JSON marshaling conventions
