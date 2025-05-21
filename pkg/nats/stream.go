package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type Streams struct {
	WorkflowCmds jetstream.Stream
	TaskCmds     jetstream.Stream
	AgentCmds    jetstream.Stream
	ToolCmds     jetstream.Stream
	Events       jetstream.Stream
	Logs         jetstream.Stream
}

func NewStreams(ctx context.Context, js jetstream.JetStream) (*Streams, error) {
	wfCmds, err := NewWorkflowCmdStream(ctx, js)
	if err != nil {
		return nil, err
	}

	taskCmds, err := NewTaskCmdStream(ctx, js)
	if err != nil {
		return nil, err
	}

	agCmds, err := NewAgentCmdStream(ctx, js)
	if err != nil {
		return nil, err
	}

	toolCmds, err := NewToolCmdStream(ctx, js)
	if err != nil {
		return nil, err
	}

	events, err := NewEventStream(ctx, js)
	if err != nil {
		return nil, err
	}

	logs, err := NewLogStream(ctx, js)
	if err != nil {
		return nil, err
	}

	return &Streams{
		WorkflowCmds: wfCmds,
		TaskCmds:     taskCmds,
		AgentCmds:    agCmds,
		ToolCmds:     toolCmds,
		Events:       events,
		Logs:         logs,
	}, nil
}

// NewWorkflowCmdStream creates the workflow commands stream
func NewWorkflowCmdStream(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, StreamWorkflowCmds,
		[]string{"compozy.*.workflow.cmds.*.*"},
		24*time.Hour)
}

// NewTaskCmdStream creates the task commands stream
func NewTaskCmdStream(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, StreamTaskCmds,
		[]string{"compozy.*.task.cmds.*.*"},
		12*time.Hour)
}

// NewAgentCmdStream creates the agent commands stream
func NewAgentCmdStream(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, StreamAgentCmds,
		[]string{"compozy.*.agent.cmds.*.*"},
		12*time.Hour)
}

// NewToolCmdStream creates the tool commands stream
func NewToolCmdStream(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, StreamToolCmds,
		[]string{"compozy.*.tool.cmds.*.*"},
		12*time.Hour)
}

// NewEventStream creates the events stream
func NewEventStream(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, StreamEvents,
		[]string{
			"compozy.*.workflow.evts.*.*",
			"compozy.*.task.evts.*.*",
			"compozy.*.agent.evts.*.*",
			"compozy.*.tool.evts.*.*",
		},
		7*24*time.Hour) // Longer retention for state events
}

// NewLogStream creates the logs stream
func NewLogStream(ctx context.Context, js jetstream.JetStream) (jetstream.Stream, error) {
	return createStream(ctx, js, StreamLogs,
		[]string{"compozy.logs.*.*.*"},
		3*24*time.Hour)
}

func createStream(ctx context.Context, js jetstream.JetStream, name StreamName, subjects []string, maxAge time.Duration) (jetstream.Stream, error) {
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
