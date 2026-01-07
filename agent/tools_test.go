package agent

import (
	"context"
	"errors"
	"testing"
)

func TestNewFuncTool(t *testing.T) {
	fn := func(ctx context.Context, input map[string]any) (any, error) {
		return "result", nil
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}

	tool := NewFuncTool("greeting", "Says hello", schema, fn)

	if tool.Name() != "greeting" {
		t.Errorf("got name %q, want %q", tool.Name(), "greeting")
	}
	if tool.Description() != "Says hello" {
		t.Errorf("got description %q, want %q", tool.Description(), "Says hello")
	}
	if tool.InputSchema() == nil {
		t.Error("expected non-nil schema")
	}
	if tool.InputSchema()["type"] != "object" {
		t.Errorf("got schema type %v, want 'object'", tool.InputSchema()["type"])
	}
}

func TestFuncToolExecute(t *testing.T) {
	fn := func(ctx context.Context, input map[string]any) (any, error) {
		name := input["name"].(string)
		return "Hello, " + name + "!", nil
	}

	tool := NewFuncTool("greet", "Greets a person", nil, fn)

	result, err := tool.Execute(context.Background(), map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello, Alice!" {
		t.Errorf("got %q, want %q", result, "Hello, Alice!")
	}
}

func TestFuncToolExecuteError(t *testing.T) {
	fn := func(ctx context.Context, input map[string]any) (any, error) {
		return nil, errors.New("something went wrong")
	}

	tool := NewFuncTool("failing", "Always fails", nil, fn)

	result, err := tool.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if err.Error() != "something went wrong" {
		t.Errorf("got error %q, want %q", err.Error(), "something went wrong")
	}
}

func TestFuncToolExecuteNilFunction(t *testing.T) {
	tool := NewFuncTool("empty", "No function", nil, nil)

	_, err := tool.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected an error for nil function")
	}

	var toolErr *ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("expected ToolError, got %T", err)
	}
	if toolErr.ToolName != "empty" {
		t.Errorf("got tool name %q, want %q", toolErr.ToolName, "empty")
	}
}

func TestFuncToolExecuteWithContext(t *testing.T) {
	fn := func(ctx context.Context, input map[string]any) (any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return "done", nil
		}
	}

	tool := NewFuncTool("ctx-aware", "Context-aware tool", nil, fn)

	// Test with active context
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "done" {
		t.Errorf("got %q, want %q", result, "done")
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = tool.Execute(ctx, nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFuncToolReturnsComplexResult(t *testing.T) {
	fn := func(ctx context.Context, input map[string]any) (any, error) {
		return map[string]any{
			"status": "success",
			"count":  42,
			"items":  []string{"a", "b", "c"},
			"nested": map[string]any{"key": "value"},
		}, nil
	}

	tool := NewFuncTool("complex", "Returns complex data", nil, fn)

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if data["status"] != "success" {
		t.Errorf("got status %v, want 'success'", data["status"])
	}
	if data["count"] != 42 {
		t.Errorf("got count %v, want 42", data["count"])
	}
}

func TestToolErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      *ToolError
		expected string
	}{
		{
			name:     "without cause",
			err:      &ToolError{ToolName: "test", Message: "failed"},
			expected: "tool test: failed",
		},
		{
			name:     "with cause",
			err:      &ToolError{ToolName: "calc", Message: "division error", Cause: errors.New("divide by zero")},
			expected: "tool calc: division error: divide by zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestToolErrorUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &ToolError{ToolName: "test", Message: "wrapper", Cause: cause}

	if err.Unwrap() != cause {
		t.Error("Unwrap did not return the cause")
	}

	// Test with nil cause
	errNoCause := &ToolError{ToolName: "test", Message: "no cause"}
	if errNoCause.Unwrap() != nil {
		t.Error("Unwrap should return nil for no cause")
	}
}

func TestCustomToolOption(t *testing.T) {
	tool1 := NewFuncTool("tool1", "First tool", nil, nil)
	tool2 := NewFuncTool("tool2", "Second tool", nil, nil)

	cfg := newConfig(CustomTool(tool1, tool2))

	if cfg.customTools == nil {
		t.Fatal("customTools map should not be nil")
	}
	if len(cfg.customTools) != 2 {
		t.Errorf("got %d tools, want 2", len(cfg.customTools))
	}
	if cfg.customTools["tool1"] != tool1 {
		t.Error("tool1 not registered correctly")
	}
	if cfg.customTools["tool2"] != tool2 {
		t.Error("tool2 not registered correctly")
	}
}

func TestCustomToolOptionMultipleCalls(t *testing.T) {
	tool1 := NewFuncTool("tool1", "First", nil, nil)
	tool2 := NewFuncTool("tool2", "Second", nil, nil)
	tool3 := NewFuncTool("tool3", "Third", nil, nil)

	// Multiple CustomTool calls should accumulate
	cfg := newConfig(
		CustomTool(tool1),
		CustomTool(tool2, tool3),
	)

	if len(cfg.customTools) != 3 {
		t.Errorf("got %d tools, want 3", len(cfg.customTools))
	}
}

func TestCustomToolOptionOverwrites(t *testing.T) {
	tool1 := NewFuncTool("samename", "First version", nil, nil)
	tool2 := NewFuncTool("samename", "Second version", nil, nil)

	cfg := newConfig(CustomTool(tool1, tool2))

	if len(cfg.customTools) != 1 {
		t.Errorf("got %d tools, want 1 (should overwrite)", len(cfg.customTools))
	}
	if cfg.customTools["samename"].Description() != "Second version" {
		t.Error("second tool should overwrite first")
	}
}

// Test custom tool with various input types
func TestFuncToolVariousInputTypes(t *testing.T) {
	fn := func(ctx context.Context, input map[string]any) (any, error) {
		return map[string]any{
			"string":  input["s"],
			"number":  input["n"],
			"boolean": input["b"],
			"array":   input["a"],
			"object":  input["o"],
		}, nil
	}

	tool := NewFuncTool("types", "Tests various types", nil, fn)

	input := map[string]any{
		"s": "hello",
		"n": 42.5,
		"b": true,
		"a": []any{1, 2, 3},
		"o": map[string]any{"nested": "value"},
	}

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := result.(map[string]any)
	if data["string"] != "hello" {
		t.Errorf("string not preserved")
	}
	if data["number"] != 42.5 {
		t.Errorf("number not preserved")
	}
	if data["boolean"] != true {
		t.Errorf("boolean not preserved")
	}
}

func TestFuncToolNilInput(t *testing.T) {
	fn := func(ctx context.Context, input map[string]any) (any, error) {
		if input == nil {
			return "nil input", nil
		}
		return "non-nil input", nil
	}

	tool := NewFuncTool("nil-test", "Tests nil input", nil, fn)

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "nil input" {
		t.Errorf("got %q, want %q", result, "nil input")
	}
}

func TestFuncToolEmptyInput(t *testing.T) {
	fn := func(ctx context.Context, input map[string]any) (any, error) {
		return len(input), nil
	}

	tool := NewFuncTool("empty-test", "Tests empty input", nil, fn)

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 0 {
		t.Errorf("got %v, want 0", result)
	}
}

func TestFuncToolNilSchema(t *testing.T) {
	tool := NewFuncTool("no-schema", "Tool without schema", nil, func(ctx context.Context, input map[string]any) (any, error) {
		return nil, nil
	})

	if tool.InputSchema() != nil {
		t.Error("expected nil schema")
	}
}

func TestFuncToolEmptyName(t *testing.T) {
	tool := NewFuncTool("", "Empty name tool", nil, nil)

	if tool.Name() != "" {
		t.Errorf("got name %q, want empty string", tool.Name())
	}
}

func TestFuncToolEmptyDescription(t *testing.T) {
	tool := NewFuncTool("nodesc", "", nil, nil)

	if tool.Description() != "" {
		t.Errorf("got description %q, want empty string", tool.Description())
	}
}

// Test that tools can mutate state (simulating application state access)
type counter struct {
	value int
}

func TestFuncToolAccessesState(t *testing.T) {
	state := &counter{value: 0}

	tool := NewFuncTool("counter", "Increments a counter", nil, func(ctx context.Context, input map[string]any) (any, error) {
		state.value++
		return state.value, nil
	})

	// Execute multiple times
	for i := 1; i <= 3; i++ {
		result, err := tool.Execute(context.Background(), nil)
		if err != nil {
			t.Fatalf("execution %d: unexpected error: %v", i, err)
		}
		if result != i {
			t.Errorf("execution %d: got %v, want %d", i, result, i)
		}
	}

	if state.value != 3 {
		t.Errorf("final state: got %d, want 3", state.value)
	}
}
