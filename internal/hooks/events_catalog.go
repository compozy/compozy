package hooks

import "fmt"

var allHookEvents = append([]HookEvent{
	HookSessionPreCreate,
	HookSessionPostCreate,
	HookSessionPreResume,
	HookSessionPostResume,
	HookSessionPreStop,
	HookSessionPostStop,
	HookSessionMessagePersisted,
	HookSandboxPrepare,
	HookSandboxReady,
	HookSandboxSyncBefore,
	HookSandboxSyncAfter,
	HookSandboxStop,
	HookInputPreSubmit,
	HookPromptPostAssemble,
	HookEventPreRecord,
	HookEventPostRecord,
	HookAutomationJobPreFire,
	HookAutomationJobPostFire,
	HookAutomationTriggerPreFire,
	HookAutomationTriggerPostFire,
	HookAutomationRunCompleted,
	HookAutomationRunFailed,
	HookAgentPreStart,
	HookAgentSpawned,
	HookAgentCrashed,
	HookAgentStopped,
	HookAgentSoulSnapshotResolved,
	HookAgentSoulMutationAfter,
	HookAgentHeartbeatPolicyResolved,
	HookAgentHeartbeatWakeBefore,
	HookAgentHeartbeatWakeAfter,
	HookSessionHealthUpdateAfter,
	HookTurnStart,
	HookTurnEnd,
	HookMessageStart,
	HookMessageDelta,
	HookMessageEnd,
	HookToolPreCall,
	HookToolPostCall,
	HookToolPostError,
	HookPermissionRequest,
	HookPermissionResolved,
	HookPermissionDenied,
	HookContextPreCompact,
	HookContextPostCompact,
	HookCoordinatorPreSpawn,
	HookCoordinatorSpawned,
	HookCoordinatorDecision,
	HookCoordinatorStopped,
	HookCoordinatorFailed,
	HookTaskBlocked,
	HookTaskUnblocked,
	HookTaskNeedsAttention,
	HookTaskRecovered,
	HookTaskStatusChanged,
	HookTaskRunEnqueued,
	HookTaskRunPreClaim,
	HookTaskRunPostClaim,
	HookTaskRunLeaseExtended,
	HookTaskRunLeaseExpired,
	HookTaskRunLeaseRecovered,
	HookTaskRunReleased,
	HookTaskRunCompleted,
	HookTaskRunFailed,
	HookLoopStarted,
	HookLoopGenerationPre,
	HookLoopGenerationPost,
	HookLoopGatePre,
	HookLoopGatePost,
	HookLoopNodeTerminal,
	HookLoopTerminal,
	HookSpawnPreCreate,
	HookSpawnCreated,
	HookSpawnParentStopped,
	HookSpawnTTLExpired,
	HookSpawnReaped,
}, append(networkHookEvents(), windowManagerHookEvents()...)...)

var _ = func() bool {
	if err := validateHookEventSpecsConsistency(); err != nil {
		panic(err)
	}
	return true
}()

// AllHookEvents returns the full taxonomy in deterministic order.
func AllHookEvents() []HookEvent {
	events := make([]HookEvent, len(allHookEvents))
	copy(events, allHookEvents)
	return events
}

// String returns the literal hook event value.
func (e HookEvent) String() string {
	return string(e)
}

// Family reports the taxonomy family for the event.
func (e HookEvent) Family() HookEventFamily {
	spec, ok := hookEventSpecs[e]
	if !ok {
		return ""
	}
	return spec.family
}

// SyncEligible reports whether the event accepts sync hooks.
func (e HookEvent) SyncEligible() bool {
	spec, ok := hookEventSpecs[e]
	return ok && spec.syncEligible
}

// Validate ensures the event is part of the supported taxonomy.
func (e HookEvent) Validate() error {
	if _, ok := hookEventSpecs[e]; !ok {
		return fmt.Errorf("hooks: invalid hook event %q", e)
	}
	return nil
}

func validateHookEventSpecsConsistency() error {
	eventsFromList := make(map[HookEvent]struct{}, len(allHookEvents))
	for _, event := range allHookEvents {
		eventsFromList[event] = struct{}{}
		if _, ok := hookEventSpecs[event]; !ok {
			return fmt.Errorf("hooks: event %q exists in allHookEvents but is missing from hookEventSpecs", event)
		}
	}
	for event := range hookEventSpecs {
		if _, ok := eventsFromList[event]; !ok {
			return fmt.Errorf("hooks: event %q exists in hookEventSpecs but is missing from allHookEvents", event)
		}
	}
	return nil
}
