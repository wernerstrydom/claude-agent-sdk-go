// Package agent provides a Go SDK for automating Claude Code CLI.
package agent

import "time"

// MessageMeta contains metadata common to all message types.
type MessageMeta struct {
	Timestamp  time.Time
	SessionID  string
	Turn       int
	Sequence   int
	ParentID   string
	SubagentID string
}

// Message is the interface implemented by all message types.
type Message interface {
	message() // unexported marker method
}

// ToolInfo describes a tool available to the agent.
type ToolInfo struct {
	Name        string
	Description string
}

// MCPStatus describes the status of an MCP server.
type MCPStatus struct {
	Name   string
	Status string
}

// Usage contains token usage information.
type Usage struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
}

// SystemInit is sent when the agent initializes.
type SystemInit struct {
	MessageMeta
	TranscriptPath string
	Tools          []ToolInfo
	MCPServers     []MCPStatus
}

func (SystemInit) message() {}

// Text contains assistant text output.
type Text struct {
	MessageMeta
	Text string
}

func (Text) message() {}

// Thinking contains the assistant's thinking process.
type Thinking struct {
	MessageMeta
	Thinking  string
	Signature string
}

func (Thinking) message() {}

// ToolUse represents a tool invocation by the assistant.
type ToolUse struct {
	MessageMeta
	ID    string
	Name  string
	Input map[string]any
}

func (ToolUse) message() {}

// ToolResult contains the result of a tool execution.
type ToolResult struct {
	MessageMeta
	ToolUseID string
	Content   any
	IsError   bool
	Duration  time.Duration
}

func (ToolResult) message() {}

// Result is the final result of an agent run.
type Result struct {
	MessageMeta
	DurationTotal time.Duration
	DurationAPI   time.Duration
	NumTurns      int
	CostUSD       float64
	Usage         Usage
	ResultText    string
	IsError       bool
}

func (Result) message() {}

// Error represents an error during agent execution.
type Error struct {
	MessageMeta
	Err error
}

func (Error) message() {}
