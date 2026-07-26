package loop

import "github.com/compozy/agh/internal/hooks"

var phaseCWatchEvents = []WatchEventsContract{
	// event_summaries is rowid-cursored and workspace-scoped directly by workspace_id.
	// Phase C uses dedicated coordinator lifecycle rows because hook.dispatch rows only
	// exist when a matching public hook is configured.
	{
		Kind:        hooks.HookCoordinatorSpawned,
		Stream:      watchEventsObserveStream,
		LedgerTypes: []string{string(hooks.HookCoordinatorSpawned)},
		PayloadFields: []string{
			watchEventsPayloadAgentName,
			watchEventsPayloadCoordinatorSessionID,
			watchEventsPayloadResolvedNetworkParticipation,
			watchEventsPayloadProvider,
			watchEventsPayloadModel,
			watchEventsPayloadWorkflowID,
			watchEventsPayloadDecisionKind,
			watchEventsPayloadDecision,
		},
	},
	{
		Kind:        hooks.HookCoordinatorDecision,
		Stream:      watchEventsObserveStream,
		LedgerTypes: []string{string(hooks.HookCoordinatorDecision)},
		PayloadFields: []string{
			watchEventsPayloadAgentName,
			watchEventsPayloadCoordinatorSessionID,
			watchEventsPayloadResolvedNetworkParticipation,
			watchEventsPayloadProvider,
			watchEventsPayloadModel,
			watchEventsPayloadWorkflowID,
			watchEventsPayloadDecisionKind,
			watchEventsPayloadDecision,
		},
	},
	{
		Kind:        hooks.HookCoordinatorStopped,
		Stream:      watchEventsObserveStream,
		LedgerTypes: []string{string(hooks.HookCoordinatorStopped)},
		PayloadFields: []string{
			watchEventsPayloadAgentName,
			watchEventsPayloadCoordinatorSessionID,
			watchEventsPayloadResolvedNetworkParticipation,
			watchEventsPayloadProvider,
			watchEventsPayloadDecisionKind,
			watchEventsPayloadDecision,
			watchEventsPayloadStopReason,
		},
	},
	{
		Kind:        hooks.HookCoordinatorFailed,
		Stream:      watchEventsObserveStream,
		LedgerTypes: []string{string(hooks.HookCoordinatorFailed)},
		PayloadFields: []string{
			watchEventsPayloadAgentName,
			watchEventsPayloadCoordinatorSessionID,
			watchEventsPayloadResolvedNetworkParticipation,
			watchEventsPayloadWorkflowID,
			watchEventsPayloadDecisionKind,
			watchEventsPayloadDecision,
			watchEventsPayloadError,
		},
	},
	// session_events streams are sequence-cursored per session. The linter requires
	// a session_id equality constraint so evaluation never scans all session DBs.
	{
		Kind:        hooks.HookEventPostRecord,
		Stream:      watchEventsSessionStream,
		LedgerTypes: []string{string(hooks.HookEventPostRecord)},
		PayloadFields: []string{
			watchEventsPayloadRecordType,
			watchEventsPayloadSequence,
			watchEventsPayloadTurnID,
			watchEventsPayloadAgentName,
			watchEventsFieldSessionID,
		},
		RequiredVars: []string{watchEventsFieldSessionID},
	},
}
