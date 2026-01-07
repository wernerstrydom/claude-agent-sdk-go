package agent

import (
	"encoding/json"
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
func (a *Agent) handleControlRequest(req *ControlRequest) error {
	if req.Tool == nil {
		// No tool call to evaluate - allow by default
		return a.sendControlResponse(req.RequestID, Allow, "", nil)
	}

	// Evaluate hook chain
	result := a.hookChain.evaluate(req.Tool)

	return a.sendControlResponse(
		req.RequestID,
		result.Decision,
		result.Reason,
		result.UpdatedInput,
	)
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
