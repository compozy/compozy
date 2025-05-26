package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/nats-io/nats.go/jetstream"
)

type HandleEvents struct {
	store core.Store
	repo  task.Repository
}

func NewHandleEvents(store core.Store, repo task.Repository) *HandleEvents {
	return &HandleEvents{store: store, repo: repo}
}

func (h *HandleEvents) Handler(ctx context.Context, msg jetstream.Msg) error {
	return h.Execute(ctx, msg)
}

func (h *HandleEvents) Execute(ctx context.Context, msg jetstream.Msg) error {
	subject := msg.Subject()
	data := msg.Data()
	parsed, err := core.ParseEvtSubject(subject)
	if err != nil {
		return fmt.Errorf("failed to parse event subject: %w", err)
	}
	event, err := nats.ParseEvent(parsed.Component, parsed.EventType, data)
	if err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}
	workflowExecID := parsed.WorkflowExecID
	taskExecID := parsed.ComponentExecID
	execution, err := h.repo.LoadExecution(ctx, workflowExecID, taskExecID)
	if err != nil {
		return err
	}
	if err := h.UpdateExecutionFromEvent(ctx, execution, event); err != nil {
		return fmt.Errorf("failed to update execution from event: %w", err)
	}
	execution.RequestData.Logger().Debug(fmt.Sprintf("Received: %T", event))
	return nil
}

func (h *HandleEvents) UpdateExecutionFromEvent(
	ctx context.Context,
	execution *task.Execution,
	event any,
) error {
	if err := h.handleEvent(execution, event); err != nil {
		return err
	}
	return h.store.UpsertJSON(ctx, execution.StoreKey(), execution)
}

func (h *HandleEvents) handleEvent(execution *task.Execution, event any) error {
	switch evt := event.(type) {
	// Task Events
	case *pb.EventTaskStarted, *pb.EventTaskWaiting, *pb.EventTaskWaitingEnded,
		*pb.EventTaskWaitingTimedOut, *pb.EventTaskSuccess, *pb.EventTaskFailed:
		return h.handleTaskEvent(execution, evt)
	// Agent Events
	case *pb.EventAgentStarted, *pb.EventAgentSuccess, *pb.EventAgentFailed:
		return h.handleAgentEvent(execution, evt)
	// Tool Events
	case *pb.EventToolStarted, *pb.EventToolSuccess, *pb.EventToolFailed:
		return h.handleToolEvent(execution, evt)
	default:
		return fmt.Errorf("unsupported event type for task state update: %T", evt)
	}
}

func (h *HandleEvents) handleTaskEvent(execution *task.Execution, event any) error {
	switch evt := event.(type) {
	case *pb.EventTaskStarted:
		execution.Status = core.StatusRunning
	case *pb.EventTaskWaiting:
		execution.Status = core.StatusWaiting
	case *pb.EventTaskWaitingEnded:
		execution.Status = core.StatusRunning
	case *pb.EventTaskWaitingTimedOut:
		execution.Status = core.StatusCanceled
		core.SetExecutionError(execution.BaseExecution, evt.GetDetails().GetError())
		execution.SetDuration()
	case *pb.EventTaskSuccess:
		execution.Status = core.StatusSuccess
		core.SetExecutionResult(execution.BaseExecution, evt.GetDetails())
		execution.SetDuration()
	case *pb.EventTaskFailed:
		execution.Status = core.StatusFailed
		core.SetExecutionError(execution.BaseExecution, evt.GetDetails().GetError())
		execution.SetDuration()
	}
	return nil
}

func (h *HandleEvents) handleAgentEvent(execution *task.Execution, event any) error {
	switch evt := event.(type) {
	case *pb.EventAgentStarted:
		execution.Status = core.StatusRunning
	case *pb.EventAgentSuccess:
		execution.Status = core.StatusSuccess
		core.SetExecutionResult(execution.BaseExecution, evt.GetDetails())
		execution.SetDuration()
	case *pb.EventAgentFailed:
		execution.Status = core.StatusFailed
		core.SetExecutionError(execution.BaseExecution, evt.GetDetails().GetError())
		execution.SetDuration()
	}
	return nil
}

func (h *HandleEvents) handleToolEvent(execution *task.Execution, event any) error {
	switch evt := event.(type) {
	case *pb.EventToolStarted:
		execution.Status = core.StatusRunning
	case *pb.EventToolSuccess:
		execution.Status = core.StatusSuccess
		core.SetExecutionResult(execution.BaseExecution, evt.GetDetails())
		execution.SetDuration()
	case *pb.EventToolFailed:
		execution.Status = core.StatusFailed
		core.SetExecutionError(execution.BaseExecution, evt.GetDetails().GetError())
		execution.SetDuration()
	}
	return nil
}
