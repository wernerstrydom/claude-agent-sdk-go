package agent

import (
	"bufio"
	"encoding/json"
	"io"
	"time"
)

// parser parses JSON lines from Claude Code CLI output.
type parser struct {
	scanner   *bufio.Scanner
	sessionID string
	turn      int
	sequence  int
}

// rawMessage is used for initial JSON parsing before type discrimination.
type rawMessage struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
	Message json.RawMessage `json:"message,omitempty"` // For assistant messages

	// System init fields
	SessionID      string   `json:"session_id,omitempty"`
	TranscriptPath string   `json:"transcript_path,omitempty"`
	Tools          []string `json:"tools,omitempty"` // Tools is []string, not []ToolInfo
	MCPServers     []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"mcp_servers,omitempty"`

	// Result fields
	DurationMS    float64 `json:"duration_ms,omitempty"`
	DurationAPIMS float64 `json:"duration_api_ms,omitempty"`
	NumTurns      int     `json:"num_turns,omitempty"`
	TotalCostUSD  float64 `json:"total_cost_usd,omitempty"` // total_cost_usd, not cost_usd
	IsError       bool    `json:"is_error,omitempty"`
	Result        string  `json:"result,omitempty"`
	Usage         *Usage  `json:"usage,omitempty"`

	// Permission/Control request fields
	RequestID string         `json:"request_id,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolInput map[string]any `json:"tool_input,omitempty"`
}

// contentBlock represents a content block in an assistant message.
type contentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Thinking  string         `json:"thinking,omitempty"`
	Signature string         `json:"signature,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
}

// newParser creates a new parser for the given reader.
func newParser(r io.Reader) *parser {
	scanner := bufio.NewScanner(r)
	// Set a larger buffer for potentially long JSON lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	return &parser{
		scanner:  scanner,
		turn:     1,
		sequence: 0,
	}
}

// next returns the next message from the stream.
func (p *parser) next() (Message, error) {
	if !p.scanner.Scan() {
		if err := p.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	line := p.scanner.Bytes()
	if len(line) == 0 {
		// Skip empty lines, try next
		return p.next()
	}

	var raw rawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, err
	}

	return p.parseMessage(&raw)
}

// parseMessage converts a rawMessage to a typed Message.
func (p *parser) parseMessage(raw *rawMessage) (Message, error) {
	meta := p.makeMeta()

	switch raw.Type {
	case "system":
		return p.parseSystemMessage(raw, meta)
	case "assistant":
		return p.parseAssistantMessage(raw, meta)
	case "result":
		return p.parseResultMessage(raw, meta)
	case "permission", "control":
		return p.parseControlRequest(raw, meta)
	default:
		// Unknown message type - return as Text with the raw type info
		p.sequence++
		return &Text{
			MessageMeta: meta,
			Text:        string(raw.Content),
		}, nil
	}
}

// parseControlRequest handles permission/control request messages.
func (p *parser) parseControlRequest(raw *rawMessage, meta MessageMeta) (Message, error) {
	return &ControlRequestMsg{
		MessageMeta: meta,
		RequestID:   raw.RequestID,
		Type:        raw.Subtype,
		ToolName:    raw.ToolName,
		ToolInput:   raw.ToolInput,
	}, nil
}

// parseSystemMessage handles system-type messages.
func (p *parser) parseSystemMessage(raw *rawMessage, meta MessageMeta) (Message, error) {
	if raw.Subtype == "init" {
		// Extract session ID for future messages
		if raw.SessionID != "" {
			p.sessionID = raw.SessionID
			meta.SessionID = raw.SessionID
		}

		// Convert tool strings to ToolInfo
		tools := make([]ToolInfo, len(raw.Tools))
		for i, name := range raw.Tools {
			tools[i] = ToolInfo{Name: name}
		}

		// Convert MCP servers
		mcpServers := make([]MCPStatus, len(raw.MCPServers))
		for i, srv := range raw.MCPServers {
			mcpServers[i] = MCPStatus{Name: srv.Name, Status: srv.Status}
		}

		return &SystemInit{
			MessageMeta:    meta,
			TranscriptPath: raw.TranscriptPath,
			Tools:          tools,
			MCPServers:     mcpServers,
		}, nil
	}

	// Unknown system subtype
	return &Text{
		MessageMeta: meta,
		Text:        string(raw.Content),
	}, nil
}

// messageContent holds the parsed message structure.
type messageContent struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

// parseAssistantMessage handles assistant-type messages with content blocks.
func (p *parser) parseAssistantMessage(raw *rawMessage, meta MessageMeta) (Message, error) {
	// Parse the message wrapper
	var msgContent messageContent
	if len(raw.Message) > 0 {
		if err := json.Unmarshal(raw.Message, &msgContent); err != nil {
			// Fall back to raw content
			return &Text{
				MessageMeta: meta,
				Text:        string(raw.Message),
			}, nil
		}
	} else if len(raw.Content) > 0 {
		// Legacy format with content at root
		if err := json.Unmarshal(raw.Content, &msgContent.Content); err != nil {
			return &Text{
				MessageMeta: meta,
				Text:        string(raw.Content),
			}, nil
		}
	}

	// Process first content block
	// In a full implementation, you might return multiple messages or aggregate
	if len(msgContent.Content) == 0 {
		return &Text{
			MessageMeta: meta,
			Text:        "",
		}, nil
	}

	block := msgContent.Content[0]

	switch block.Type {
	case "text":
		return &Text{
			MessageMeta: meta,
			Text:        block.Text,
		}, nil
	case "thinking":
		return &Thinking{
			MessageMeta: meta,
			Thinking:    block.Thinking,
			Signature:   block.Signature,
		}, nil
	case "tool_use":
		return &ToolUse{
			MessageMeta: meta,
			ID:          block.ID,
			Name:        block.Name,
			Input:       block.Input,
		}, nil
	default:
		return &Text{
			MessageMeta: meta,
			Text:        block.Text,
		}, nil
	}
}

// parseResultMessage handles result-type messages.
func (p *parser) parseResultMessage(raw *rawMessage, meta MessageMeta) (Message, error) {
	p.turn++ // Result typically ends a turn

	usage := Usage{}
	if raw.Usage != nil {
		usage = *raw.Usage
	}

	return &Result{
		MessageMeta:   meta,
		DurationTotal: time.Duration(raw.DurationMS * float64(time.Millisecond)),
		DurationAPI:   time.Duration(raw.DurationAPIMS * float64(time.Millisecond)),
		NumTurns:      raw.NumTurns,
		CostUSD:       raw.TotalCostUSD, // Use TotalCostUSD
		Usage:         usage,
		ResultText:    raw.Result,
		IsError:       raw.IsError,
	}, nil
}

// makeMeta creates a MessageMeta with current state.
// It increments sequence for each message.
func (p *parser) makeMeta() MessageMeta {
	p.sequence++
	return MessageMeta{
		Timestamp: time.Now(),
		SessionID: p.sessionID,
		Turn:      p.turn,
		Sequence:  p.sequence,
	}
}
