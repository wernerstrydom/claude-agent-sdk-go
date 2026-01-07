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

	// System init fields
	SessionID      string      `json:"session_id,omitempty"`
	TranscriptPath string      `json:"transcript_path,omitempty"`
	Tools          []ToolInfo  `json:"tools,omitempty"`
	MCPServers     []MCPStatus `json:"mcp_servers,omitempty"`

	// Result fields
	DurationMS    float64 `json:"duration_ms,omitempty"`
	DurationAPIMS float64 `json:"duration_api_ms,omitempty"`
	NumTurns      int     `json:"num_turns,omitempty"`
	CostUSD       float64 `json:"cost_usd,omitempty"`
	IsError       bool    `json:"is_error,omitempty"`
	Result        string  `json:"result,omitempty"`
	Usage         *Usage  `json:"usage,omitempty"`
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
	default:
		// Unknown message type - return as Text with the raw type info
		p.sequence++
		return &Text{
			MessageMeta: meta,
			Text:        string(raw.Content),
		}, nil
	}
}

// parseSystemMessage handles system-type messages.
func (p *parser) parseSystemMessage(raw *rawMessage, meta MessageMeta) (Message, error) {
	if raw.Subtype == "init" {
		// Extract session ID for future messages
		if raw.SessionID != "" {
			p.sessionID = raw.SessionID
			meta.SessionID = raw.SessionID
		}
		return &SystemInit{
			MessageMeta:    meta,
			TranscriptPath: raw.TranscriptPath,
			Tools:          raw.Tools,
			MCPServers:     raw.MCPServers,
		}, nil
	}

	// Unknown system subtype
	return &Text{
		MessageMeta: meta,
		Text:        string(raw.Content),
	}, nil
}

// parseAssistantMessage handles assistant-type messages with content blocks.
func (p *parser) parseAssistantMessage(raw *rawMessage, meta MessageMeta) (Message, error) {
	// Parse content blocks
	var blocks []contentBlock
	if err := json.Unmarshal(raw.Content, &blocks); err != nil {
		// If content is not an array, treat it as text
		var text string
		if err := json.Unmarshal(raw.Content, &text); err != nil {
			// Return raw content as text
			return &Text{
				MessageMeta: meta,
				Text:        string(raw.Content),
			}, nil
		}
		return &Text{
			MessageMeta: meta,
			Text:        text,
		}, nil
	}

	// Process first content block
	// In a full implementation, you might return multiple messages or aggregate
	if len(blocks) == 0 {
		return &Text{
			MessageMeta: meta,
			Text:        "",
		}, nil
	}

	block := blocks[0]

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
		CostUSD:       raw.CostUSD,
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
