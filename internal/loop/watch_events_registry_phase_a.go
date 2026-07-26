package loop

import "github.com/compozy/agh/internal/hooks"

var phaseAWatchEvents = []WatchEventsContract{
	{
		Kind:          hooks.HookTaskStatusChanged,
		Stream:        watchEventsTaskStream,
		LedgerTypes:   []string{string(hooks.HookTaskStatusChanged)},
		PayloadFields: []string{"from_status", "to_status", watchEventsPayloadParentTaskID},
	},
	{
		Kind:        hooks.HookTaskBlocked,
		Stream:      watchEventsTaskStream,
		LedgerTypes: []string{string(hooks.HookTaskBlocked)},
		PayloadFields: []string{
			"block_id",
			actionKindMetaKey,
			watchEventsPayloadReason,
			watchEventsPayloadDetails,
			watchEventsPayloadParentTaskID,
		},
	},
	{
		Kind:        hooks.HookTaskUnblocked,
		Stream:      watchEventsTaskStream,
		LedgerTypes: []string{string(hooks.HookTaskUnblocked)},
		PayloadFields: []string{
			"block_id",
			actionKindMetaKey,
			watchEventsPayloadReason,
			watchEventsPayloadDetails,
			"cleared_at",
			"clear_note",
			watchEventsPayloadParentTaskID,
		},
	},
	{
		Kind:        hooks.HookTaskNeedsAttention,
		Stream:      watchEventsTaskStream,
		LedgerTypes: []string{string(hooks.HookTaskNeedsAttention)},
		PayloadFields: []string{
			watchEventsPayloadReason,
			"note",
			"at",
			watchEventsPayloadParentTaskID,
		},
	},
	{
		Kind:        hooks.HookTaskRecovered,
		Stream:      watchEventsTaskStream,
		LedgerTypes: []string{string(hooks.HookTaskRecovered)},
		PayloadFields: []string{
			watchEventsPayloadReason,
			"note",
			"at",
			watchEventsPayloadParentTaskID,
		},
	},
	{
		Kind:        hooks.HookTaskRunCompleted,
		Stream:      watchEventsTaskStream,
		LedgerTypes: []string{string(hooks.HookTaskRunCompleted)},
		PayloadFields: []string{
			"previous_run_status",
			"previous_session_id",
			"recovery_action",
			"recovery_reason",
		},
	},
	{
		Kind:        hooks.HookTaskRunFailed,
		Stream:      watchEventsTaskStream,
		LedgerTypes: []string{string(hooks.HookTaskRunFailed)},
		PayloadFields: []string{
			"previous_run_status",
			"previous_session_id",
			"recovery_action",
			"recovery_reason",
			"error",
		},
	},
	{
		Kind:        hooks.HookLoopTerminal,
		Stream:      watchEventsLoopStream,
		LedgerTypes: []string{loopRunEventLedgerStatusChanged},
		PayloadFields: []string{
			reasonMetaStatus,
			"from",
			"to",
			"cause",
			"reason_code",
			watchEventsPayloadDetails,
		},
	},
	{
		Kind:        hooks.HookLoopNodeTerminal,
		Stream:      watchEventsLoopStream,
		LedgerTypes: []string{loopRunEventLedgerNodeSucceeded, loopRunEventLedgerNodeFailed},
		PayloadFields: []string{
			"node_id",
			"generation",
			"item_index",
			"task_id",
			"task_run_id",
			reasonMetaStatus,
			"run_status",
			"error",
			watchEventsPayloadDetails,
			"output_ref",
		},
	},
}
