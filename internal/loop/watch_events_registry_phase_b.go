package loop

import "github.com/compozy/compozy/internal/hooks"

var phaseBWatchEvents = []WatchEventsContract{
	// automation_runs is rowid-cursored and workspace-scoped by joining the run's job/trigger
	// plus loop_run fallback; the run row has no workspace_id column.
	{
		Kind:        hooks.HookAutomationRunCompleted,
		Stream:      watchEventsAutomationStream,
		LedgerTypes: []string{string(hooks.HookAutomationRunCompleted)},
		PayloadFields: []string{
			watchEventsPayloadJobID,
			watchEventsPayloadTriggerID,
			watchEventsPayloadAgentName,
			watchEventsFieldSessionID,
			watchEventsPayloadAttempt,
			watchEventsPayloadDurationMS,
		},
	},
	{
		Kind:        hooks.HookAutomationRunFailed,
		Stream:      watchEventsAutomationStream,
		LedgerTypes: []string{string(hooks.HookAutomationRunFailed)},
		PayloadFields: []string{
			watchEventsPayloadJobID,
			watchEventsPayloadTriggerID,
			watchEventsPayloadAgentName,
			watchEventsFieldSessionID,
			watchEventsPayloadAttempt,
			watchEventsPayloadError,
			watchEventsPayloadWillRetry,
		},
	},
}
