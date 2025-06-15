package worker

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/gosimple/slug"
)

// Use the same constant as defined in mod.go to avoid mismatches

// TemporalSignalDispatcher implements SignalDispatcher using Temporal client
type TemporalSignalDispatcher struct {
	client       client.Client
	dispatcherID string
	taskQueue    string
}

// NewTemporalSignalDispatcher creates a new TemporalSignalDispatcher
func NewTemporalSignalDispatcher(
	client client.Client,
	dispatcherID string,
	taskQueue string,
) services.SignalDispatcher {
	return &TemporalSignalDispatcher{
		client:       client,
		dispatcherID: dispatcherID,
		taskQueue:    taskQueue,
	}
}

// DispatchSignal sends a signal using Temporal's SignalWithStartWorkflow
func (t *TemporalSignalDispatcher) DispatchSignal(
	ctx context.Context,
	signalName string,
	payload map[string]any,
	correlationID string,
) error {
	projectName, err := core.GetProjectName(ctx)
	if err != nil {
		return fmt.Errorf("failed to get project name: %w", err)
	}

	_, err = t.client.SignalWithStartWorkflow(
		ctx,
		t.dispatcherID,
		DispatcherEventChannel,
		EventSignal{
			Name:          signalName,
			Payload:       core.Input(payload),
			CorrelationID: correlationID,
		},
		client.StartWorkflowOptions{
			ID:        t.dispatcherID,
			TaskQueue: t.taskQueue,
		},
		DispatcherWorkflow,
		projectName,
	)
	return err
}

func GetTaskQueue(projectName string) string {
	return slug.Make(projectName)
}
