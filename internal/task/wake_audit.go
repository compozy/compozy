package task

import (
	"context"
	"log/slog"
	"strings"
)

type wakeTaskPayload struct {
	WakeEventID        string     `json:"wake_event_id"`
	WorkspaceID        string     `json:"workspace_id,omitempty"`
	Reason             WakeReason `json:"reason"`
	Status             Status     `json:"status,omitempty"`
	RunID              string     `json:"run_id,omitempty"`
	CreatorSessionID   string     `json:"creator_session_id,omitempty"`
	ExecutingSessionID string     `json:"executing_session_id,omitempty"`
	Summary            string     `json:"summary,omitempty"`
	SuppressionReason  string     `json:"suppression_reason,omitempty"`
	Error              string     `json:"error,omitempty"`
}

func (m *Service) recordWakeDelivered(
	ctx context.Context,
	auditEventID string,
	taskRecord Task,
	event WakeEvent,
	req *wakeDispatchRequest,
) {
	if err := m.recordTaskEventWithID(
		ctx,
		auditEventID,
		event.TaskID,
		event.RunID,
		taskEventWakeDelivered,
		req.actor,
		wakePayload(taskRecord, event, req, "", nil),
	); err != nil {
		m.logWakeDispatchError("task: wake delivered audit failed", err, event, taskRecord.ID)
	}
}

func (m *Service) recordWakeSuppressed(
	ctx context.Context,
	auditEventID string,
	taskRecord Task,
	event WakeEvent,
	req *wakeDispatchRequest,
	reason string,
	deliveryErr error,
) {
	if err := m.recordTaskEventWithID(
		ctx,
		auditEventID,
		event.TaskID,
		event.RunID,
		taskEventWakeSuppressed,
		req.actor,
		wakePayload(taskRecord, event, req, reason, deliveryErr),
	); err != nil {
		m.logWakeDispatchError("task: wake suppressed audit failed", err, event, taskRecord.ID)
	}
}

func wakePayload(
	taskRecord Task,
	event WakeEvent,
	req *wakeDispatchRequest,
	suppressionReason string,
	deliveryErr error,
) wakeTaskPayload {
	payload := wakeTaskPayload{
		WakeEventID:        event.WakeEventID,
		WorkspaceID:        event.WorkspaceID,
		Reason:             event.Reason,
		Status:             taskRecord.Status.Normalize(),
		RunID:              event.RunID,
		CreatorSessionID:   strings.TrimSpace(taskRecord.CreatedBy.Ref),
		ExecutingSessionID: strings.TrimSpace(req.executingSessionID),
		Summary:            event.Summary,
		SuppressionReason:  strings.TrimSpace(suppressionReason),
	}
	if deliveryErr != nil {
		payload.Error = boundWakeSummary(deliveryErr.Error())
	}
	return payload
}

func (m *Service) logWakeDispatchError(message string, err error, event WakeEvent, taskID string) {
	if err == nil {
		return
	}
	slog.Error(
		message,
		"error", err,
		"task_id", strings.TrimSpace(taskID),
		"run_id", strings.TrimSpace(event.RunID),
		"wake_event_id", strings.TrimSpace(event.WakeEventID),
		"reason", event.Reason.Normalize(),
	)
}
