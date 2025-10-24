package services

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/streaming"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
)

// PublishComplete emits a completion event for the provided task state.
func PublishComplete(ctx context.Context, publisher streaming.Publisher, state *task.State, payload any) {
	publish(ctx, publisher, state, streaming.EventTypeComplete, map[string]any{
		"status": stateStatus(state),
		"output": stateOutput(state),
		"usage":  stateUsage(state),
		"result": payload,
	})
}

// PublishError emits an error event with a redacted message.
func PublishError(ctx context.Context, publisher streaming.Publisher, state *task.State, err error) {
	publish(ctx, publisher, state, streaming.EventTypeError, map[string]any{
		"status":  stateStatus(state),
		"message": core.RedactError(err),
	})
}

func publish(
	ctx context.Context,
	publisher streaming.Publisher,
	state *task.State,
	eventType streaming.EventType,
	data map[string]any,
) {
	if publisher == nil || state == nil || state.TaskExecID.IsZero() {
		return
	}
	payload := baseData(state)
	for k, v := range data {
		payload[k] = v
	}
	event := streaming.Event{Type: eventType, Data: payload}
	if _, err := publisher.Publish(ctx, state.TaskExecID, event); err != nil {
		logger.FromContext(ctx).Warn(
			"Failed to publish execution event",
			"event_type", eventType,
			"task_exec_id", state.TaskExecID.String(),
			"error", core.RedactError(err),
		)
	}
}

func baseData(state *task.State) map[string]any {
	data := map[string]any{
		"component": state.Component,
	}
	if state.TaskID != "" {
		data["task_id"] = state.TaskID
	}
	if !state.WorkflowExecID.IsZero() {
		data["workflow_exec_id"] = state.WorkflowExecID.String()
	}
	if state.AgentID != nil {
		data["agent_id"] = *state.AgentID
	}
	return data
}

func stateStatus(state *task.State) core.StatusType {
	if state == nil {
		return ""
	}
	return state.Status
}

func stateOutput(state *task.State) *core.Output {
	if state == nil {
		return nil
	}
	return state.Output
}

func stateUsage(state *task.State) any {
	if state == nil {
		return nil
	}
	return state.Usage
}
