package agent

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorTypesImplementError(t *testing.T) {
	// Compile-time check that all types implement error
	var _ error = &StartError{}
	var _ error = &ProcessError{}
	var _ error = &MaxTurnsError{}
	var _ error = &HookInterruptError{}
	var _ error = &TaskError{}
}

func TestStartError(t *testing.T) {
	t.Run("without cause", func(t *testing.T) {
		err := &StartError{Reason: "cli not found"}
		if !strings.HasPrefix(err.Error(), "agent: ") {
			t.Errorf("error should have 'agent: ' prefix, got: %s", err.Error())
		}
		if !strings.Contains(err.Error(), "cli not found") {
			t.Errorf("error should contain reason, got: %s", err.Error())
		}
	})

	t.Run("with cause", func(t *testing.T) {
		cause := fmt.Errorf("file not found")
		err := &StartError{Reason: "cli not found", Cause: cause}
		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("error should contain cause, got: %s", err.Error())
		}
	})

	t.Run("unwrap", func(t *testing.T) {
		cause := fmt.Errorf("underlying error")
		err := &StartError{Reason: "test", Cause: cause}
		if err.Unwrap() != cause {
			t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
		}
	})
}

func TestProcessError(t *testing.T) {
	err := &ProcessError{ExitCode: 1, Stderr: "command failed"}
	if !strings.HasPrefix(err.Error(), "agent: ") {
		t.Errorf("error should have 'agent: ' prefix, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("error should contain exit code, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "command failed") {
		t.Errorf("error should contain stderr, got: %s", err.Error())
	}
}

func TestMaxTurnsError(t *testing.T) {
	err := &MaxTurnsError{Turns: 15, MaxAllowed: 10, SessionID: "sess-123"}
	if !strings.HasPrefix(err.Error(), "agent: ") {
		t.Errorf("error should have 'agent: ' prefix, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "15") {
		t.Errorf("error should contain turns, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "10") {
		t.Errorf("error should contain max allowed, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "sess-123") {
		t.Errorf("error should contain session ID, got: %s", err.Error())
	}
}

func TestHookInterruptError(t *testing.T) {
	err := &HookInterruptError{Hook: "security", Tool: "Bash", Reason: "dangerous command"}
	if !strings.HasPrefix(err.Error(), "agent: ") {
		t.Errorf("error should have 'agent: ' prefix, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "security") {
		t.Errorf("error should contain hook name, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "Bash") {
		t.Errorf("error should contain tool name, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "dangerous command") {
		t.Errorf("error should contain reason, got: %s", err.Error())
	}
}

func TestTaskError(t *testing.T) {
	err := &TaskError{SessionID: "sess-456", Message: "task failed"}
	if !strings.HasPrefix(err.Error(), "agent: ") {
		t.Errorf("error should have 'agent: ' prefix, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "sess-456") {
		t.Errorf("error should contain session ID, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "task failed") {
		t.Errorf("error should contain message, got: %s", err.Error())
	}
}

func TestErrorsAs(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target any
	}{
		{
			name:   "StartError",
			err:    &StartError{Reason: "test"},
			target: &StartError{},
		},
		{
			name:   "ProcessError",
			err:    &ProcessError{ExitCode: 1},
			target: &ProcessError{},
		},
		{
			name:   "MaxTurnsError",
			err:    &MaxTurnsError{Turns: 10},
			target: &MaxTurnsError{},
		},
		{
			name:   "HookInterruptError",
			err:    &HookInterruptError{Hook: "test"},
			target: &HookInterruptError{},
		},
		{
			name:   "TaskError",
			err:    &TaskError{SessionID: "test"},
			target: &TaskError{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap the error
			wrapped := fmt.Errorf("wrapped: %w", tt.err)

			switch target := tt.target.(type) {
			case *StartError:
				var e *StartError
				if !errors.As(wrapped, &e) {
					t.Errorf("errors.As failed for %s", tt.name)
				}
			case *ProcessError:
				var e *ProcessError
				if !errors.As(wrapped, &e) {
					t.Errorf("errors.As failed for %s", tt.name)
				}
			case *MaxTurnsError:
				var e *MaxTurnsError
				if !errors.As(wrapped, &e) {
					t.Errorf("errors.As failed for %s", tt.name)
				}
			case *HookInterruptError:
				var e *HookInterruptError
				if !errors.As(wrapped, &e) {
					t.Errorf("errors.As failed for %s", tt.name)
				}
			case *TaskError:
				var e *TaskError
				if !errors.As(wrapped, &e) {
					t.Errorf("errors.As failed for %s", tt.name)
				}
			default:
				t.Errorf("unknown target type: %T", target)
			}
		})
	}
}

func TestUnwrapChain(t *testing.T) {
	root := fmt.Errorf("root cause")
	middle := &StartError{Reason: "middle", Cause: root}
	outer := fmt.Errorf("outer: %w", middle)

	// errors.Is should find root through the chain
	if !errors.Is(outer, root) {
		t.Error("errors.Is should find root cause through unwrap chain")
	}

	// errors.As should find StartError through the chain
	var startErr *StartError
	if !errors.As(outer, &startErr) {
		t.Error("errors.As should find StartError through unwrap chain")
	}
	if startErr.Reason != "middle" {
		t.Errorf("StartError.Reason = %q, want %q", startErr.Reason, "middle")
	}
}
