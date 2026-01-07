package agent

import "fmt"

// StartError indicates the agent failed to start.
type StartError struct {
	Reason string
	Cause  error
}

func (e *StartError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("agent: start failed: %s: %v", e.Reason, e.Cause)
	}
	return fmt.Sprintf("agent: start failed: %s", e.Reason)
}

func (e *StartError) Unwrap() error {
	return e.Cause
}

// ProcessError indicates the Claude Code process exited with an error.
type ProcessError struct {
	ExitCode int
	Stderr   string
}

func (e *ProcessError) Error() string {
	return fmt.Sprintf("agent: process exited with code %d: %s", e.ExitCode, e.Stderr)
}

// MaxTurnsError indicates the agent exceeded the maximum number of turns.
type MaxTurnsError struct {
	Turns      int
	MaxAllowed int
	SessionID  string
}

func (e *MaxTurnsError) Error() string {
	return fmt.Sprintf("agent: max turns exceeded: %d/%d (session: %s)", e.Turns, e.MaxAllowed, e.SessionID)
}

// HookInterruptError indicates a hook blocked execution.
type HookInterruptError struct {
	Hook   string
	Tool   string
	Reason string
}

func (e *HookInterruptError) Error() string {
	return fmt.Sprintf("agent: hook %s blocked tool %s: %s", e.Hook, e.Tool, e.Reason)
}

// TaskError indicates a task-level error.
type TaskError struct {
	SessionID string
	Message   string
}

func (e *TaskError) Error() string {
	return fmt.Sprintf("agent: task error (session: %s): %s", e.SessionID, e.Message)
}

// SchemaError indicates a JSON Schema generation or unmarshaling error.
type SchemaError struct {
	Type   string // Go type name
	Reason string
	Cause  error
}

func (e *SchemaError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("agent: schema error for type %s: %s: %v", e.Type, e.Reason, e.Cause)
	}
	return fmt.Sprintf("agent: schema error for type %s: %s", e.Type, e.Reason)
}

func (e *SchemaError) Unwrap() error {
	return e.Cause
}
