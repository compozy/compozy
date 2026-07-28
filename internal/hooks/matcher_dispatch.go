package hooks

func selectMatchingHooks[P any](
	snapshot []*ResolvedHook,
	payload P,
	match matcherFunc[P],
) ([]*ResolvedHook, []*ResolvedHook) {
	syncHooks := make([]*ResolvedHook, 0, len(snapshot))
	asyncHooks := make([]*ResolvedHook, 0, len(snapshot))

	for _, hook := range snapshot {
		if hook == nil {
			continue
		}
		if match != nil && !match(hook.Matcher, payload) {
			continue
		}
		switch hook.Mode {
		case HookModeAsync:
			asyncHooks = append(asyncHooks, hook)
		case HookModeSync:
			syncHooks = append(syncHooks, hook)
		}
	}

	return syncHooks, asyncHooks
}

func matchSessionPreCreate(matcher HookMatcher, payload SessionPreCreatePayload) bool {
	return matcher.MatchesSession(payload.SessionContext)
}

func matchSessionLifecycle(matcher HookMatcher, payload SessionLifecyclePayload) bool {
	return matcher.MatchesSession(payload.SessionContext)
}

func matchSandboxPrepare(matcher HookMatcher, payload SandboxPreparePayload) bool {
	return matcher.MatchesSandboxPrepare(payload)
}

func matchSandboxReady(matcher HookMatcher, payload SandboxReadyPayload) bool {
	return matcher.MatchesSandboxReady(payload)
}

func matchSandboxSyncBefore(matcher HookMatcher, payload SandboxSyncBeforePayload) bool {
	return matcher.MatchesSandboxSyncBefore(payload)
}

func matchSandboxSyncAfter(matcher HookMatcher, payload SandboxSyncAfterPayload) bool {
	return matcher.MatchesSandboxSyncAfter(payload)
}

func matchSandboxStop(matcher HookMatcher, payload SandboxStopPayload) bool {
	return matcher.MatchesSandboxStop(payload)
}

func matchInputPreSubmit(matcher HookMatcher, payload InputPreSubmitPayload) bool {
	return matcher.MatchesInput(payload)
}

func matchPrompt(matcher HookMatcher, payload PromptPayload) bool {
	return matcher.MatchesPrompt(payload)
}

func matchEventRecord(matcher HookMatcher, payload EventRecordPayload) bool {
	return matcher.MatchesEvent(payload)
}

func matchAutomationJobPreFire(matcher HookMatcher, payload AutomationJobPreFirePayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAutomationJobPostFire(matcher HookMatcher, payload AutomationJobPostFirePayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAutomationTriggerPreFire(matcher HookMatcher, payload AutomationTriggerPreFirePayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAutomationTriggerPostFire(matcher HookMatcher, payload AutomationTriggerPostFirePayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAutomationRunCompleted(matcher HookMatcher, payload AutomationRunCompletedPayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAutomationRunFailed(matcher HookMatcher, payload AutomationRunFailedPayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAgentPreStart(matcher HookMatcher, payload AgentPreStartPayload) bool {
	return matcher.MatchesAgentPreStart(payload)
}

func matchAgentLifecycle(matcher HookMatcher, payload AgentLifecyclePayload) bool {
	return matcher.MatchesAgentLifecycle(payload)
}

func matchAgentSoulSnapshotResolved(matcher HookMatcher, payload AgentSoulSnapshotResolvedPayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAgentSoulMutationAfter(matcher HookMatcher, payload AgentSoulMutationAfterPayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAgentHeartbeatPolicyResolved(matcher HookMatcher, payload AgentHeartbeatPolicyResolvedPayload) bool {
	return matcher.MatchesAutomation(payload.AgentName, payload.WorkspaceID)
}

func matchAgentHeartbeatWakeBefore(matcher HookMatcher, payload AgentHeartbeatWakeBeforePayload) bool {
	return matcher.MatchesSession(payload.SessionContext)
}

func matchAgentHeartbeatWakeAfter(matcher HookMatcher, payload AgentHeartbeatWakeAfterPayload) bool {
	return matcher.MatchesSession(payload.SessionContext)
}

func matchSessionHealthUpdateAfter(matcher HookMatcher, payload SessionHealthUpdateAfterPayload) bool {
	return matcher.MatchesSession(payload.SessionContext)
}

func matchSessionMessagePersisted(matcher HookMatcher, payload SessionMessagePersistedPayload) bool {
	return matcher.MatchesSession(payload.SessionContext)
}

func matchTurn(matcher HookMatcher, payload TurnPayload) bool {
	return matcher.MatchesTurn(payload)
}

func matchMessage(matcher HookMatcher, payload MessagePayload) bool {
	return matcher.MatchesMessage(payload)
}

func matchNetwork(matcher HookMatcher, payload NetworkPayload) bool {
	return matcher.MatchesNetwork(payload)
}

func matchWindowManager(matcher HookMatcher, payload WindowManagerPayload) bool {
	return matchStringField(matcher.WorkspaceID, payload.WorkspaceID)
}

func matchToolPreCall(matcher HookMatcher, payload ToolPreCallPayload) bool {
	return matcher.MatchesToolPreCall(payload)
}

func matchToolPostCall(matcher HookMatcher, payload ToolPostCallPayload) bool {
	return matcher.MatchesToolPostCall(payload)
}

func matchToolPostError(matcher HookMatcher, payload ToolPostErrorPayload) bool {
	return matcher.MatchesToolPostError(payload)
}

func matchPermissionRequest(matcher HookMatcher, payload PermissionRequestPayload) bool {
	return matcher.MatchesPermissionRequest(payload)
}

func matchPermissionResolution(matcher HookMatcher, payload PermissionResolutionPayload) bool {
	return matcher.MatchesPermissionResolution(payload)
}

func matchContextCompact(matcher HookMatcher, payload ContextCompactPayload) bool {
	return matcher.MatchesContextCompact(payload)
}

func matchCoordinatorPreSpawn(matcher HookMatcher, payload CoordinatorPreSpawnPayload) bool {
	return matcher.MatchesCoordinator(payload.CoordinatorContext)
}

func matchCoordinatorLifecycle(matcher HookMatcher, payload CoordinatorLifecyclePayload) bool {
	return matcher.MatchesCoordinator(payload.CoordinatorContext)
}

func matchTaskBlock(matcher HookMatcher, payload TaskBlockPayload) bool {
	return matcher.MatchesTask(payload.TaskContext)
}

func matchTaskAttention(matcher HookMatcher, payload TaskAttentionPayload) bool {
	return matcher.MatchesTask(payload.TaskContext)
}

func matchTaskRunEnqueued(matcher HookMatcher, payload TaskRunEnqueuedPayload) bool {
	return matcher.MatchesTaskRun(payload.TaskRunContext)
}

func matchTaskRunPreClaim(matcher HookMatcher, payload TaskRunPreClaimPayload) bool {
	return matcher.MatchesTaskRun(payload.taskRunContextSnapshot())
}

func matchTaskRunPostClaim(matcher HookMatcher, payload TaskRunPostClaimPayload) bool {
	return matcher.MatchesTaskRun(payload.TaskRunContext)
}

func matchTaskRunLease(matcher HookMatcher, payload TaskRunLeasePayload) bool {
	return matcher.MatchesTaskRun(payload.TaskRunContext)
}

func matchLoopLifecycle(matcher HookMatcher, payload LoopLifecyclePayload) bool {
	return matcher.MatchesLoop(payload.LoopContext)
}

func matchLoopGeneration(matcher HookMatcher, payload LoopGenerationPayload) bool {
	return matcher.MatchesLoop(payload.LoopContext)
}

func matchLoopGate(matcher HookMatcher, payload LoopGatePayload) bool {
	return matcher.MatchesLoop(payload.LoopContext)
}

func matchLoopNodeTerminal(matcher HookMatcher, payload LoopNodeTerminalPayload) bool {
	return matcher.MatchesLoop(payload.LoopContext)
}

func matchSpawnPreCreate(matcher HookMatcher, payload SpawnPreCreatePayload) bool {
	return matcher.MatchesSpawn(payload.SpawnContext)
}

func matchSpawnLifecycle(matcher HookMatcher, payload SpawnLifecyclePayload) bool {
	return matcher.MatchesSpawn(payload.SpawnContext)
}
