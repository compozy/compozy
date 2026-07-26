package daemon

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	watchEventsPayloadAttemptKey                      = "attempt"
	watchEventsPayloadAgentNameKey                    = "agent_name"
	watchEventsPayloadCausationIDKey                  = "causation_id"
	watchEventsPayloadChannelKey                      = "channel"
	watchEventsPayloadDetailsKey                      = "details"
	watchEventsPayloadDirectIDKey                     = "direct_id"
	watchEventsPayloadDirectionKey                    = "direction"
	watchEventsPayloadDurationMSKey                   = "duration_ms"
	watchEventsPayloadErrorKey                        = "error"
	watchEventsPayloadKindKey                         = "kind"
	watchEventsPayloadMessageIDKey                    = "message_id"
	watchEventsPayloadNodeIDKey                       = "node_id"
	watchEventsPayloadParentTaskIDKey                 = "parent_task_id"
	watchEventsPayloadPeerFromKey                     = "peer_from"
	watchEventsPayloadPeerToKey                       = "peer_to"
	watchEventsPayloadReasonKey                       = "reason"
	watchEventsPayloadRecordTypeKey                   = "record_type"
	watchEventsPayloadResolvedNetworkParticipationKey = "resolved_network_participation"
	watchEventsPayloadSequenceKey                     = "sequence"
	watchEventsPayloadSessionIDKey                    = "session_id"
	watchEventsPayloadSurfaceKey                      = "surface"
	watchEventsPayloadThreadIDKey                     = "thread_id"
	watchEventsPayloadTraceIDKey                      = "trace_id"
	watchEventsPayloadTurnIDKey                       = "turn_id"
	watchEventsPayloadWillRetryKey                    = "will_retry"
	watchEventsPayloadWorkIDKey                       = "work_id"
	watchEventsPayloadWorkStateKey                    = "work_state"

	watchEventsPayloadCoordinatorSessionIDKey = "coordinator_session_id"
	watchEventsPayloadDecisionKey             = "decision"
	watchEventsPayloadDecisionKindKey         = "decision_kind"
	watchEventsPayloadModelKey                = "model"
	watchEventsPayloadProviderKey             = "provider"
	watchEventsPayloadStopReasonKey           = "stop_reason"
	watchEventsPayloadWorkflowIDKey           = "workflow_id"
)

func watchEventsTaskStatusChangedEvent(
	payload hookspkg.TaskStatusChangedPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	return looppkg.WatchEvent{
		Kind:        string(hookspkg.HookTaskStatusChanged),
		Stream:      looppkg.WatchEventsTaskStream,
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		TaskID:      strings.TrimSpace(payload.TaskID),
		RunID:       strings.TrimSpace(payload.RunID),
		Payload: map[string]any{
			"from_status":                     strings.TrimSpace(payload.FromStatus),
			"to_status":                       strings.TrimSpace(payload.ToStatus),
			watchEventsPayloadParentTaskIDKey: strings.TrimSpace(payload.ParentTaskID),
		},
		LedgerKind: string(hookspkg.HookTaskStatusChanged),
	}
}

func watchEventsTaskBlockEvent(
	kind hookspkg.HookEvent,
	payload hookspkg.TaskBlockPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	return looppkg.WatchEvent{
		Kind:        string(kind),
		Stream:      looppkg.WatchEventsTaskStream,
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		TaskID:      strings.TrimSpace(payload.TaskID),
		RunID:       strings.TrimSpace(payload.RunID),
		Payload: map[string]any{
			"block_id":                        strings.TrimSpace(payload.BlockID),
			watchEventsPayloadKindKey:         strings.TrimSpace(payload.Kind),
			watchEventsPayloadReasonKey:       strings.TrimSpace(payload.Reason),
			watchEventsPayloadDetailsKey:      watchEventsRawJSONValue(payload.Details),
			"cleared_at":                      watchEventsOptionalTime(payload.ClearedAt),
			"clear_note":                      strings.TrimSpace(payload.ClearNote),
			watchEventsPayloadParentTaskIDKey: strings.TrimSpace(payload.ParentTaskID),
		},
		LedgerKind: string(kind),
	}
}

func watchEventsTaskAttentionEvent(
	kind hookspkg.HookEvent,
	payload hookspkg.TaskAttentionPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	return looppkg.WatchEvent{
		Kind:        string(kind),
		Stream:      looppkg.WatchEventsTaskStream,
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		TaskID:      strings.TrimSpace(payload.TaskID),
		RunID:       strings.TrimSpace(payload.RunID),
		Payload: map[string]any{
			watchEventsPayloadReasonKey:       strings.TrimSpace(payload.Reason),
			"note":                            strings.TrimSpace(payload.Note),
			"at":                              watchEventsOptionalTime(payload.At),
			watchEventsPayloadParentTaskIDKey: strings.TrimSpace(payload.ParentTaskID),
		},
		LedgerKind: string(kind),
	}
}

func watchEventsTaskRunTerminalEvent(
	payload hookspkg.TaskRunLeasePayload,
	now func() time.Time,
) looppkg.WatchEvent {
	kind := payload.Event
	if strings.TrimSpace(string(kind)) == "" {
		kind = hookspkg.HookTaskRunCompleted
		if taskpkg.ParseRunStatus(payload.RunStatus).Normalize() == taskpkg.TaskRunStatusFailed {
			kind = hookspkg.HookTaskRunFailed
		}
	}
	return looppkg.WatchEvent{
		Kind:        string(kind),
		Stream:      looppkg.WatchEventsTaskStream,
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		TaskID:      strings.TrimSpace(payload.TaskID),
		RunID:       strings.TrimSpace(payload.RunID),
		LoopRunID:   strings.TrimSpace(payload.LoopRunID),
		SessionID:   strings.TrimSpace(payload.SessionID),
		Channel:     strings.TrimSpace(payload.NetworkSpecSnapshot().ChannelID),
		Payload: map[string]any{
			"previous_run_status":      strings.TrimSpace(payload.PreviousRunStatus),
			"previous_session_id":      strings.TrimSpace(payload.PreviousSessionID),
			"recovery_action":          strings.TrimSpace(payload.RecoveryAction),
			"recovery_reason":          strings.TrimSpace(payload.RecoveryReason),
			watchEventsPayloadErrorKey: strings.TrimSpace(payload.Error),
		},
		LedgerKind: string(kind),
	}
}

func watchEventsAutomationRunCompletedEvent(
	payload hookspkg.AutomationRunCompletedPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	return looppkg.WatchEvent{
		Kind:        string(hookspkg.HookAutomationRunCompleted),
		Stream:      looppkg.WatchEventsAutomationStream,
		At:          watchEventsHookAt(time.Time{}, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		RunID:       strings.TrimSpace(payload.RunID),
		SessionID:   strings.TrimSpace(payload.SessionID),
		Payload: map[string]any{
			daemonPayloadJobIDKey:           strings.TrimSpace(payload.JobID),
			daemonPayloadTriggerIDKey:       strings.TrimSpace(payload.TriggerID),
			daemonPayloadAgentNameKey:       strings.TrimSpace(payload.AgentName),
			daemonPayloadSessionIDKey:       strings.TrimSpace(payload.SessionID),
			watchEventsPayloadAttemptKey:    payload.Attempt,
			watchEventsPayloadDurationMSKey: payload.DurationMS,
		},
		LedgerKind: string(hookspkg.HookAutomationRunCompleted),
	}
}

func watchEventsAutomationRunFailedEvent(
	payload hookspkg.AutomationRunFailedPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	return looppkg.WatchEvent{
		Kind:        string(hookspkg.HookAutomationRunFailed),
		Stream:      looppkg.WatchEventsAutomationStream,
		At:          watchEventsHookAt(time.Time{}, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		RunID:       strings.TrimSpace(payload.RunID),
		SessionID:   strings.TrimSpace(payload.SessionID),
		Payload: map[string]any{
			daemonPayloadJobIDKey:          strings.TrimSpace(payload.JobID),
			daemonPayloadTriggerIDKey:      strings.TrimSpace(payload.TriggerID),
			daemonPayloadAgentNameKey:      strings.TrimSpace(payload.AgentName),
			daemonPayloadSessionIDKey:      strings.TrimSpace(payload.SessionID),
			watchEventsPayloadAttemptKey:   payload.Attempt,
			watchEventsPayloadErrorKey:     strings.TrimSpace(payload.Error),
			watchEventsPayloadWillRetryKey: payload.WillRetry,
		},
		LedgerKind: string(hookspkg.HookAutomationRunFailed),
	}
}

func watchEventsNetworkEvent(
	kind hookspkg.HookEvent,
	payload hookspkg.NetworkPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	return looppkg.WatchEvent{
		Kind:        string(kind),
		Stream:      looppkg.WatchEventsNetworkStream,
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		SessionID:   strings.TrimSpace(payload.SessionID),
		Channel:     strings.TrimSpace(payload.Channel),
		WorkID:      strings.TrimSpace(payload.WorkID),
		Payload: map[string]any{
			daemonPayloadSessionIDKey:        strings.TrimSpace(payload.SessionID),
			watchEventsPayloadChannelKey:     strings.TrimSpace(payload.Channel),
			watchEventsPayloadSurfaceKey:     strings.TrimSpace(payload.Surface),
			watchEventsPayloadThreadIDKey:    strings.TrimSpace(payload.ThreadID),
			watchEventsPayloadDirectIDKey:    strings.TrimSpace(payload.DirectID),
			watchEventsPayloadMessageIDKey:   strings.TrimSpace(payload.MessageID),
			watchEventsPayloadKindKey:        strings.TrimSpace(payload.Kind),
			watchEventsPayloadDirectionKey:   strings.TrimSpace(payload.Direction),
			watchEventsPayloadWorkIDKey:      strings.TrimSpace(payload.WorkID),
			watchEventsPayloadWorkStateKey:   strings.TrimSpace(payload.WorkState),
			watchEventsPayloadPeerFromKey:    strings.TrimSpace(payload.PeerFrom),
			watchEventsPayloadPeerToKey:      strings.TrimSpace(payload.PeerTo),
			watchEventsPayloadTraceIDKey:     strings.TrimSpace(payload.TraceID),
			watchEventsPayloadCausationIDKey: strings.TrimSpace(payload.CausationID),
		},
		LedgerKind: string(kind),
	}
}

func watchEventsCoordinatorLifecycleEvent(
	kind hookspkg.HookEvent,
	payload hookspkg.CoordinatorLifecyclePayload,
	now func() time.Time,
) looppkg.WatchEvent {
	coordinatorSessionID := strings.TrimSpace(payload.CoordinatorSessionID)
	return looppkg.WatchEvent{
		Kind:        string(kind),
		Stream:      looppkg.WatchEventsObserveStream,
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		TaskID:      strings.TrimSpace(payload.TaskID),
		RunID:       strings.TrimSpace(payload.RunID),
		SessionID:   coordinatorSessionID,
		Payload: map[string]any{
			watchEventsPayloadAgentNameKey:                    strings.TrimSpace(payload.AgentName),
			watchEventsPayloadCoordinatorSessionIDKey:         coordinatorSessionID,
			watchEventsPayloadResolvedNetworkParticipationKey: payload.ResolvedNetworkParticipation,
			watchEventsPayloadWorkflowIDKey:                   strings.TrimSpace(payload.WorkflowID),
			watchEventsPayloadProviderKey:                     strings.TrimSpace(payload.Provider),
			watchEventsPayloadModelKey:                        strings.TrimSpace(payload.Model),
			watchEventsPayloadDecisionKindKey:                 strings.TrimSpace(payload.DecisionKind),
			watchEventsPayloadDecisionKey:                     strings.TrimSpace(payload.Decision),
			watchEventsPayloadStopReasonKey:                   strings.TrimSpace(payload.StopReason),
			watchEventsPayloadErrorKey:                        strings.TrimSpace(payload.Error),
		},
		LedgerKind: string(kind),
	}
}

func watchEventsEventPostRecordEvent(
	payload hookspkg.EventPostRecordPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	sessionID := strings.TrimSpace(payload.SessionID)
	sequence := payload.Sequence
	return looppkg.WatchEvent{
		Kind:        string(hookspkg.HookEventPostRecord),
		Seq:         sequence,
		Stream:      looppkg.WatchEventsSessionStreamForSession(sessionID),
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		SessionID:   sessionID,
		Payload: map[string]any{
			watchEventsPayloadRecordTypeKey: strings.TrimSpace(payload.RecordType),
			watchEventsPayloadSequenceKey:   sequence,
			watchEventsPayloadTurnIDKey:     strings.TrimSpace(payload.TurnID),
			watchEventsPayloadAgentNameKey:  strings.TrimSpace(payload.AgentName),
			watchEventsPayloadSessionIDKey:  sessionID,
		},
		LedgerKind: string(hookspkg.HookEventPostRecord),
	}
}

func watchEventsLoopTerminalEvent(
	payload hookspkg.LoopTerminalPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	return looppkg.WatchEvent{
		Kind:        string(hookspkg.HookLoopTerminal),
		Stream:      looppkg.WatchEventsLoopStream,
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		TaskID:      strings.TrimSpace(payload.TaskID),
		RunID:       strings.TrimSpace(payload.RunID),
		LoopRunID:   strings.TrimSpace(payload.LoopRunID),
		LoopName:    strings.TrimSpace(payload.LoopName),
		SessionID:   strings.TrimSpace(payload.SessionID),
		Channel:     watchEventsParticipationChannel(payload.ResolvedNetworkParticipation),
		Payload: map[string]any{
			daemonStatusField:            strings.TrimSpace(payload.Status),
			"to":                         strings.TrimSpace(payload.Status),
			"cause":                      strings.TrimSpace(payload.Cause),
			"reason_code":                strings.TrimSpace(payload.ReasonCode),
			watchEventsPayloadDetailsKey: watchEventsRawJSONValue(payload.Details),
		},
		LedgerKind: "status_changed",
	}
}

func watchEventsLoopNodeTerminalEvent(
	payload hookspkg.LoopNodeTerminalPayload,
	now func() time.Time,
) looppkg.WatchEvent {
	return looppkg.WatchEvent{
		Kind:        string(hookspkg.HookLoopNodeTerminal),
		Stream:      looppkg.WatchEventsLoopStream,
		At:          watchEventsHookAt(payload.Timestamp, now),
		WorkspaceID: strings.TrimSpace(payload.WorkspaceID),
		TaskID:      strings.TrimSpace(payload.TaskID),
		RunID:       strings.TrimSpace(payload.RunID),
		LoopRunID:   strings.TrimSpace(payload.LoopRunID),
		LoopName:    strings.TrimSpace(payload.LoopName),
		SessionID:   strings.TrimSpace(payload.SessionID),
		Channel:     watchEventsParticipationChannel(payload.ResolvedNetworkParticipation),
		Payload: map[string]any{
			watchEventsPayloadNodeIDKey:  strings.TrimSpace(payload.NodeID),
			"generation":                 payload.Generation,
			coordinatorRuntimeTaskIDKey:  strings.TrimSpace(payload.TaskID),
			"task_run_id":                strings.TrimSpace(payload.RunID),
			daemonStatusField:            strings.TrimSpace(payload.TaskStatus),
			"run_status":                 strings.TrimSpace(payload.RunStatus),
			watchEventsPayloadErrorKey:   strings.TrimSpace(payload.Error),
			watchEventsPayloadDetailsKey: watchEventsRawJSONValue(payload.Details),
		},
		LedgerKind: watchEventsLoopNodeLedgerKind(payload),
	}
}

func watchEventsParticipationChannel(spec *participation.Spec) string {
	if spec == nil {
		return ""
	}
	return strings.TrimSpace(spec.ChannelID)
}

func watchEventsLoopNodeLedgerKind(payload hookspkg.LoopNodeTerminalPayload) string {
	if strings.TrimSpace(payload.Error) != "" ||
		taskpkg.ParseRunStatus(payload.RunStatus).Normalize() == taskpkg.TaskRunStatusFailed {
		return "node_failed"
	}
	return "node_succeeded"
}

func watchEventsHookAt(timestamp time.Time, now func() time.Time) string {
	if timestamp.IsZero() {
		if now == nil {
			timestamp = time.Now().UTC()
		} else {
			timestamp = now().UTC()
		}
	}
	return timestamp.UTC().Format(time.RFC3339Nano)
}

func watchEventsOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func watchEventsRawJSONValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return string(raw)
	}
	return value
}
