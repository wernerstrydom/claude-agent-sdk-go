package agent

import "context"

// Tool is the interface for custom in-process tools.
// Custom tools are executed directly by the SDK without going through the CLI,
// enabling fast, in-process execution with access to application state.
type Tool interface {
	// Name returns the unique identifier for this tool.
	// Claude will use this name to invoke the tool.
	Name() string

	// Description returns a human-readable description of what the tool does.
	// This is shown to Claude to help it understand when to use the tool.
	Description() string

	// InputSchema returns the JSON Schema for the tool's input parameters.
	// Return nil if the tool accepts no parameters.
	InputSchema() map[string]any

	// Execute runs the tool with the given context and input parameters.
	// The input is a map of parameter names to values matching the schema.
	// Returns the tool result and any error that occurred.
	Execute(ctx context.Context, input map[string]any) (any, error)
}

// Compile-time check that FuncTool implements Tool.
var _ Tool = (*FuncTool)(nil)

// FuncTool is a function-based implementation of Tool.
// Use NewFuncTool to create instances.
type FuncTool struct {
	name        string
	description string
	schema      map[string]any
	fn          func(context.Context, map[string]any) (any, error)
}

// NewFuncTool creates a new Tool from a function.
//
// Example:
//
//	tool := agent.NewFuncTool(
//	    "calculator",
//	    "Performs arithmetic calculations",
//	    map[string]any{
//	        "type": "object",
//	        "properties": map[string]any{
//	            "expression": map[string]any{
//	                "type":        "string",
//	                "description": "The arithmetic expression to evaluate",
//	            },
//	        },
//	        "required": []string{"expression"},
//	    },
//	    func(ctx context.Context, input map[string]any) (any, error) {
//	        expr := input["expression"].(string)
//	        // ... evaluate expression ...
//	        return result, nil
//	    },
//	)
func NewFuncTool(
	name, description string,
	inputSchema map[string]any,
	fn func(context.Context, map[string]any) (any, error),
) *FuncTool {
	return &FuncTool{
		name:        name,
		description: description,
		schema:      inputSchema,
		fn:          fn,
	}
}

// Name returns the tool's name.
func (t *FuncTool) Name() string {
	return t.name
}

// Description returns the tool's description.
func (t *FuncTool) Description() string {
	return t.description
}

// InputSchema returns the tool's input schema.
func (t *FuncTool) InputSchema() map[string]any {
	return t.schema
}

// Execute runs the tool function.
func (t *FuncTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	if t.fn == nil {
		return nil, &ToolError{
			ToolName: t.name,
			Message:  "no function defined",
		}
	}
	return t.fn(ctx, input)
}

// ToolError represents an error during custom tool execution.
type ToolError struct {
	ToolName string
	Message  string
	Cause    error
}

// Error implements the error interface.
func (e *ToolError) Error() string {
	if e.Cause != nil {
		return "tool " + e.ToolName + ": " + e.Message + ": " + e.Cause.Error()
	}
	return "tool " + e.ToolName + ": " + e.Message
}

// Unwrap returns the underlying error.
func (e *ToolError) Unwrap() error {
	return e.Cause
}
