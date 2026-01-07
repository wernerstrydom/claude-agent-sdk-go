package agent

import (
	"context"
	"encoding/json"
	"time"
)

// ControlRequest represents a permission request from Claude Code CLI.
type ControlRequest struct {
	RequestID string
	Type      string // e.g., "tool_use"
	Tool      *ToolCall
}

// controlResponse is the JSON structure for responding to control requests.
type controlResponse struct {
	RequestID    string         `json:"request_id"`
	Decision     string         `json:"decision"` // "allow" or "deny"
	Reason       string         `json:"reason,omitempty"`
	UpdatedInput map[string]any `json:"updated_input,omitempty"`
}

// handleControlRequest evaluates hooks and sends a response to the process.
// For custom tools, it executes the tool in-process and returns the result.
func (a *Agent) handleControlRequest(ctx context.Context, req *ControlRequest) error {
	if req.Tool == nil {
		// No tool call to evaluate - allow by default
		return a.sendControlResponse(req.RequestID, Allow, "", nil)
	}

	// Check if this is a custom tool
	customTool := a.cfg.customTools[req.Tool.Name]

	// Evaluate hook chain
	result := a.hookChain.evaluate(req.Tool)

	// Emit hook.pre_tool_use audit event
	a.auditor.emit(a.sessionID, "hook.pre_tool_use", map[string]any{
		"tool":        req.Tool.Name,
		"input":       req.Tool.Input,
		"decision":    result.Decision.String(),
		"reason":      result.Reason,
		"custom_tool": customTool != nil,
	})

	// If denied, send denial response
	if result.Decision == Deny {
		return a.sendControlResponse(
			req.RequestID,
			result.Decision,
			result.Reason,
			result.UpdatedInput,
		)
	}

	// If this is a custom tool and allowed, execute it
	if customTool != nil {
		return a.executeCustomTool(ctx, req, customTool, result.UpdatedInput)
	}

	// For non-custom tools, send allow response
	return a.sendControlResponse(
		req.RequestID,
		result.Decision,
		result.Reason,
		result.UpdatedInput,
	)
}

// executeCustomTool executes a custom tool and sends the result to the CLI.
func (a *Agent) executeCustomTool(ctx context.Context, req *ControlRequest, tool Tool, updatedInput map[string]any) error {
	// Use updated input if provided by hooks, otherwise use original
	input := req.Tool.Input
	if updatedInput != nil {
		input = updatedInput
	}

	// Emit tool.custom.start audit event
	a.auditor.emit(a.sessionID, "tool.custom.start", map[string]any{
		"tool":       req.Tool.Name,
		"input":      input,
		"request_id": req.RequestID,
	})

	start := time.Now()

	// Execute the custom tool
	result, err := tool.Execute(ctx, input)
	duration := time.Since(start)

	if err != nil {
		// Emit tool.custom.error audit event
		a.auditor.emit(a.sessionID, "tool.custom.error", map[string]any{
			"tool":     req.Tool.Name,
			"input":    input,
			"error":    err.Error(),
			"duration": duration.String(),
		})

		// Send error result back to CLI
		return a.sendCustomToolResult(req.RequestID, err.Error(), true)
	}

	// Emit tool.custom.complete audit event
	a.auditor.emit(a.sessionID, "tool.custom.complete", map[string]any{
		"tool":     req.Tool.Name,
		"input":    input,
		"result":   result,
		"duration": duration.String(),
	})

	// Send success result back to CLI
	return a.sendCustomToolResult(req.RequestID, result, false)
}

// customToolResponse is the JSON structure for returning custom tool results.
type customToolResponse struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
	Result    any    `json:"result,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// sendCustomToolResult sends a custom tool result to the CLI.
func (a *Agent) sendCustomToolResult(requestID string, result any, isError bool) error {
	resp := customToolResponse{
		RequestID: requestID,
		Decision:  "allow",
		Result:    result,
		IsError:   isError,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return a.proc.write(data)
}

// sendControlResponse sends a control response to the process.
func (a *Agent) sendControlResponse(requestID string, decision Decision, reason string, updatedInput map[string]any) error {
	decisionStr := "allow"
	if decision == Deny {
		decisionStr = "deny"
	}

	resp := controlResponse{
		RequestID:    requestID,
		Decision:     decisionStr,
		Reason:       reason,
		UpdatedInput: updatedInput,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return a.proc.write(data)
}
