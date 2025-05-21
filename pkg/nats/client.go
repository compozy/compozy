package nats

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Client implements the Client interface
type Client struct {
	conn    *nats.Conn
	streams *Streams
	js      jetstream.JetStream
}

// NewClient creates a new NATS client
func NewClient(conn *nats.Conn) (*Client, error) {
	js, err := jetstream.New(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}
	return &Client{
		conn: conn,
		js:   js,
	}, nil
}

// JetStream returns a JetStream context
func (c *Client) JetStream() (jetstream.JetStream, error) {
	return c.js, nil
}

// Conn returns the underlying NATS connection
func (c *Client) Conn() *nats.Conn {
	return c.conn
}

// Close closes the NATS connection
func (c *Client) Close() error {
	c.conn.Close()
	return nil
}

// CloseWithContext closes the NATS connection with a context parameter
// This allows for context-aware operations with timeouts, cancellation, etc.
func (c *Client) CloseWithContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		c.conn.Close()
		return nil
	}
}

func (c *Client) SetupStreams(ctx context.Context) error {
	streams, err := NewStreams(ctx, c.js)
	if err != nil {
		return fmt.Errorf("failed to create Streams context: %w", err)
	}
	c.streams = streams
	return nil
}

func (c *Client) Setup(ctx context.Context) error {
	if err := c.SetupStreams(ctx); err != nil {
		return err
	}
	return nil
}

func (c *Client) GetStream(ctx context.Context, name StreamName) (jetstream.Stream, error) {
	switch name {
	case StreamWorkflowCmds:
		return NewWorkflowCmdStream(ctx, c.js)
	case StreamTaskCmds:
		return NewTaskCmdStream(ctx, c.js)
	case StreamAgentCmds:
		return NewAgentCmdStream(ctx, c.js)
	case StreamToolCmds:
		return NewToolCmdStream(ctx, c.js)
	case StreamEvents:
		return NewEventStream(ctx, c.js)
	case StreamLogs:
		return NewLogStream(ctx, c.js)
	}
	return nil, fmt.Errorf("stream not found: %s", name)
}
