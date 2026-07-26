package hooks

// MatchesSession matches session-family hooks.
func (m HookMatcher) MatchesSession(payload SessionContext) bool {
	return m.matchSessionContext(payload, true)
}

// MatchesSandboxPrepare matches sandbox prepare hooks.
func (m HookMatcher) MatchesSandboxPrepare(payload SandboxPreparePayload) bool {
	return m.matchSandbox(
		payload.SessionContext,
		payload.SandboxID,
		payload.Backend,
		payload.Profile.Profile,
		"",
	)
}

// MatchesSandboxReady matches sandbox ready hooks.
func (m HookMatcher) MatchesSandboxReady(payload SandboxReadyPayload) bool {
	return m.matchSandbox(payload.SessionContext, payload.SandboxID, payload.Backend, payload.Profile, "")
}

// MatchesSandboxSyncBefore matches sandbox pre-sync hooks.
func (m HookMatcher) MatchesSandboxSyncBefore(payload SandboxSyncBeforePayload) bool {
	return m.matchSandbox(
		payload.SessionContext,
		payload.SandboxID,
		payload.Backend,
		payload.Profile,
		payload.Direction,
	)
}

// MatchesSandboxSyncAfter matches sandbox post-sync hooks.
func (m HookMatcher) MatchesSandboxSyncAfter(payload SandboxSyncAfterPayload) bool {
	return m.matchSandbox(
		payload.SessionContext,
		payload.SandboxID,
		payload.Backend,
		payload.Profile,
		payload.Direction,
	)
}

// MatchesSandboxStop matches sandbox stop hooks.
func (m HookMatcher) MatchesSandboxStop(payload SandboxStopPayload) bool {
	return m.matchSandbox(payload.SessionContext, payload.SandboxID, payload.Backend, payload.Profile, "")
}

// MatchesInput matches input-family hooks.
func (m HookMatcher) MatchesInput(payload InputPreSubmitPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false) &&
		matchStringField(m.InputClass, payload.InputClass)
}

// MatchesPrompt matches prompt-family hooks.
func (m HookMatcher) MatchesPrompt(payload PromptPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false) &&
		matchStringField(m.InputClass, payload.InputClass)
}

// MatchesEvent matches event-record-family hooks.
func (m HookMatcher) MatchesEvent(payload EventRecordPayload) bool {
	return matchStringField(m.AgentName, payload.AgentName) &&
		matchStringField(m.ACPEventType, payload.RecordType) &&
		matchStringField(m.TurnID, payload.TurnID)
}

// MatchesAutomation matches automation lifecycle hooks.
func (m HookMatcher) MatchesAutomation(agentName string, workspaceID string) bool {
	return matchStringField(m.AgentName, agentName) &&
		matchStringField(m.WorkspaceID, workspaceID)
}

// MatchesAgentPreStart matches pre-start agent hooks.
func (m HookMatcher) MatchesAgentPreStart(payload AgentPreStartPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false)
}

// MatchesAgentLifecycle matches spawned, crashed, and stopped agent hooks.
func (m HookMatcher) MatchesAgentLifecycle(payload AgentLifecyclePayload) bool {
	return m.matchSessionContext(payload.SessionContext, false)
}

// MatchesTurn matches turn-family hooks.
func (m HookMatcher) MatchesTurn(payload TurnPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false) &&
		matchStringField(m.InputClass, payload.InputClass)
}

// MatchesMessage matches message-family hooks.
func (m HookMatcher) MatchesMessage(payload MessagePayload) bool {
	return matchStringField(m.MessageRole, payload.Role) &&
		matchStringField(m.MessageDeltaType, payload.DeltaType)
}

// MatchesToolPreCall matches tool pre-call hooks.
func (m HookMatcher) MatchesToolPreCall(payload ToolPreCallPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false) &&
		m.matchToolCall(payload.ToolCallRef)
}

// MatchesToolPostCall matches tool post-call hooks.
func (m HookMatcher) MatchesToolPostCall(payload ToolPostCallPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false) &&
		m.matchToolCall(payload.ToolCallRef)
}

// MatchesToolPostError matches tool post-error hooks.
func (m HookMatcher) MatchesToolPostError(payload ToolPostErrorPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false) &&
		m.matchToolCall(payload.ToolCallRef)
}

// MatchesPermissionRequest matches permission-request hooks.
func (m HookMatcher) MatchesPermissionRequest(payload PermissionRequestPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false) &&
		m.matchPermission(payload.ToolCall.Kind, payload.DecisionClass)
}

// MatchesPermissionResolution matches resolved and denied permission hooks.
func (m HookMatcher) MatchesPermissionResolution(payload PermissionResolutionPayload) bool {
	return m.matchSessionContext(payload.SessionContext, false) &&
		m.matchPermission(payload.ToolCall.Kind, payload.DecisionClass)
}

// MatchesContextCompact matches context-compaction hooks.
func (m HookMatcher) MatchesContextCompact(payload ContextCompactPayload) bool {
	compaction := m.compaction()
	return matchStringField(compaction.Reason, payload.Reason) &&
		matchStringField(compaction.Strategy, payload.Strategy)
}

// MatchesCoordinator matches coordinator-family hooks.
func (m HookMatcher) MatchesCoordinator(payload CoordinatorContext) bool {
	autonomy := m.autonomy()
	return matchStringField(m.AgentName, payload.AgentName) &&
		matchStringField(m.WorkspaceID, payload.WorkspaceID) &&
		matchStringField(m.WorkspaceRoot, payload.Workspace) &&
		matchStringField(autonomy.TaskID, payload.TaskID) &&
		matchStringField(autonomy.RunID, payload.RunID) &&
		matchStringField(autonomy.WorkflowID, payload.WorkflowID) &&
		matchStringField(
			autonomy.ParticipationChannel,
			resolvedParticipationChannel(payload.ResolvedNetworkParticipation),
		) &&
		matchStringField(autonomy.CoordinatorSessionID, payload.CoordinatorSessionID)
}

// MatchesTask matches task-family hooks.
func (m HookMatcher) MatchesTask(payload TaskContext) bool {
	return m.matchTaskAutonomyFields(
		payload.AgentName,
		payload.WorkspaceID,
		payload.TaskID,
		payload.RunID,
		"",
		"",
		"",
		payload.WorkflowID,
		payload.NetworkSpecSnapshot().ChannelID,
		payload.ReleaseReason,
	)
}

// MatchesTaskRun matches task-run-family hooks.
func (m HookMatcher) MatchesTaskRun(payload TaskRunContext) bool {
	return m.matchTaskAutonomyFields(
		payload.AgentName,
		payload.WorkspaceID,
		payload.TaskID,
		payload.RunID,
		payload.LoopRunID,
		"",
		"",
		payload.WorkflowID,
		payload.NetworkSpecSnapshot().ChannelID,
		payload.ReleaseReason,
	)
}

// MatchesLoop matches loop-family hooks.
func (m HookMatcher) MatchesLoop(payload LoopContext) bool {
	return m.matchTaskAutonomyFields(
		payload.AgentName,
		payload.WorkspaceID,
		payload.TaskID,
		payload.RunID,
		payload.LoopRunID,
		payload.LoopName,
		payload.NodeID,
		payload.WorkflowID,
		resolvedParticipationChannel(payload.ResolvedNetworkParticipation),
		"",
	)
}

func (m HookMatcher) matchTaskAutonomyFields(
	agentName string,
	workspaceID string,
	taskID string,
	runID string,
	loopRunID string,
	loopName string,
	nodeID string,
	workflowID string,
	participationChannel string,
	releaseReason string,
) bool {
	autonomy := m.autonomy()
	return matchStringField(m.AgentName, agentName) &&
		matchStringField(m.WorkspaceID, workspaceID) &&
		matchStringField(autonomy.TaskID, taskID) &&
		matchStringField(autonomy.RunID, runID) &&
		matchStringField(autonomy.LoopRunID, loopRunID) &&
		matchStringField(autonomy.LoopName, loopName) &&
		matchStringField(autonomy.NodeID, nodeID) &&
		matchStringField(autonomy.WorkflowID, workflowID) &&
		matchStringField(autonomy.ParticipationChannel, participationChannel) &&
		matchStringField(autonomy.ReleaseReason, releaseReason)
}

// MatchesSpawn matches spawn-family hooks.
func (m HookMatcher) MatchesSpawn(payload SpawnContext) bool {
	autonomy := m.autonomy()
	return matchStringField(m.AgentName, payload.AgentName) &&
		matchStringField(m.WorkspaceID, payload.WorkspaceID) &&
		matchStringField(m.WorkspaceRoot, payload.Workspace) &&
		matchStringField(autonomy.TaskID, payload.TaskID) &&
		matchStringField(autonomy.RunID, payload.RunID) &&
		matchStringField(autonomy.WorkflowID, payload.WorkflowID) &&
		matchStringField(
			autonomy.ParticipationChannel,
			resolvedParticipationChannel(payload.ResolvedNetworkParticipation),
		) &&
		matchStringField(autonomy.ParentSessionID, payload.ParentSessionID) &&
		matchStringField(autonomy.RootSessionID, payload.RootSessionID) &&
		matchStringField(autonomy.ChildSessionID, payload.ChildSessionID) &&
		matchStringField(autonomy.SpawnRole, payload.SpawnRole)
}

var emptyAutonomyMatcher = &AutonomyMatcher{}
var emptyCompactionMatcher = &CompactionMatcher{}

func (m HookMatcher) compaction() *CompactionMatcher {
	if m.CompactionMatcher == nil {
		return emptyCompactionMatcher
	}
	return m.CompactionMatcher
}

func (m HookMatcher) autonomy() *AutonomyMatcher {
	if m.Autonomy == nil {
		return emptyAutonomyMatcher
	}
	return m.Autonomy
}
