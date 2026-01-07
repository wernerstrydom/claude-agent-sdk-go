package agent

import (
	"io"
	"sync"
)

// bridge pumps messages from a parser to a channel.
type bridge struct {
	parser    *parser
	messages  chan Message
	err       error
	done      chan struct{}
	closeOnce sync.Once
}

// newBridge creates a new bridge that reads from the given reader.
func newBridge(r io.Reader) *bridge {
	b := &bridge{
		parser:   newParser(r),
		messages: make(chan Message, 32),
		done:     make(chan struct{}),
	}
	go b.pump()
	return b
}

// pump reads messages from the parser and sends them to the channel.
func (b *bridge) pump() {
	defer close(b.messages)

	for {
		msg, err := b.parser.next()
		if err != nil {
			if err != io.EOF {
				b.err = err
			}
			return
		}

		select {
		case b.messages <- msg:
		case <-b.done:
			return
		}
	}
}

// recv returns the read-only message channel.
func (b *bridge) recv() <-chan Message {
	return b.messages
}

// error returns any error that occurred during parsing.
func (b *bridge) error() error {
	return b.err
}

// close signals the pump to stop.
func (b *bridge) close() {
	b.closeOnce.Do(func() {
		close(b.done)
	})
}
