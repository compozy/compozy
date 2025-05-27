package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/nats-io/nats.go/jetstream"
)

type Streams struct {
	Commands jetstream.Stream
	Events   jetstream.Stream
	Logs     jetstream.Stream
}

func NewStreams(ctx context.Context, js jetstream.JetStream) (*Streams, error) {
	wfCmds, err := NewStreamCommands(ctx, js)
	if err != nil {
		return nil, err
	}

	events, err := NewStreamEvents(ctx, js)
	if err != nil {
		return nil, err
	}

	logs, err := NewStreamLogs(ctx, js)
	if err != nil {
		return nil, err
	}

	return &Streams{
		Commands: wfCmds,
		Events:   events,
		Logs:     logs,
	}, nil
}

// NewStreamCommands creates the workflow commands stream
func NewStreamCommands(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, core.StreamCommands,
		[]string{core.StreamCmdWildcard()},
		24*time.Hour)
}

// NewStreamEvents creates the events stream
func NewStreamEvents(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, core.StreamEvents,
		[]string{core.StreamEventWildcard()},
		7*24*time.Hour) // Longer retention for state events
}

// NewStreamLogs creates the logs stream
func NewStreamLogs(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, core.StreamLogs,
		[]string{core.StreamLogWidcard()},
		3*24*time.Hour)
}

func createStream(
	ctx context.Context,
	js jetstream.JetStream,
	name core.StreamName,
	subjects []string,
	maxAge time.Duration,
) (jetstream.Stream, error) {
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     string(name),
		Subjects: subjects,
		Storage:  jetstream.FileStorage,
		MaxAge:   maxAge,
	})
	if err != nil && err != jetstream.ErrStreamNameAlreadyInUse {
		return nil, fmt.Errorf("failed to create %s stream: %w", name, err)
	}
	return stream, nil
}

func (c *Client) GetStream(ctx context.Context, name core.StreamName) (jetstream.Stream, error) {
	switch name {
	case core.StreamCommands:
		return NewStreamCommands(ctx, c.js)
	case core.StreamEvents:
		return NewStreamEvents(ctx, c.js)
	case core.StreamLogs:
		return NewStreamLogs(ctx, c.js)
	}
	return nil, fmt.Errorf("stream not found: %s", name)
}
