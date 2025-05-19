package nats

import (
	"github.com/nats-io/nats.go"
)

// Client defines the interface for NATS client operations
type Client interface {
	// JetStreamContext returns a JetStream context
	JetStreamContext() (nats.JetStreamContext, error)

	// Conn returns the underlying NATS connection
	Conn() *nats.Conn

	// Close closes the NATS connection
	Close() error
}

// ClientImpl implements the Client interface
type ClientImpl struct {
	conn *nats.Conn
}

// NewClient creates a new NATS client
func NewClient(conn *nats.Conn) Client {
	return &ClientImpl{
		conn: conn,
	}
}

// JetStreamContext returns a JetStream context
func (c *ClientImpl) JetStreamContext() (nats.JetStreamContext, error) {
	return c.conn.JetStream()
}

// Conn returns the underlying NATS connection
func (c *ClientImpl) Conn() *nats.Conn {
	return c.conn
}

// Close closes the NATS connection
func (c *ClientImpl) Close() error {
	c.conn.Close()
	return nil
}
