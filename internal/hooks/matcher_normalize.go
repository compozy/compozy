package hooks

import "strings"

func (m HookMatcher) matchSessionContext(payload SessionContext, includeSessionType bool) bool {
	if !matchStringField(m.AgentName, payload.AgentName) {
		return false
	}
	if !matchStringField(m.WorkspaceID, payload.WorkspaceID) {
		return false
	}
	if !matchStringField(m.WorkspaceRoot, payload.Workspace) {
		return false
	}
	if includeSessionType && !matchStringField(m.SessionType, payload.SessionType) {
		return false
	}
	return true
}

func (m HookMatcher) matchSandbox(
	session SessionContext,
	sandboxID string,
	backend string,
	profile string,
	direction string,
) bool {
	return m.matchSessionContext(session, false) &&
		matchStringField(m.SandboxID, sandboxID) &&
		matchStringField(m.SandboxBackend, backend) &&
		matchStringField(m.SandboxProfile, profile) &&
		matchStringField(m.SyncDirection, direction)
}

func (m HookMatcher) matchToolCall(payload ToolCallRef) bool {
	if !matchStringField(m.ToolID, payload.ToolID) {
		return false
	}
	if m.ToolReadOnly != nil && payload.ReadOnly != *m.ToolReadOnly {
		return false
	}
	return true
}

func (m HookMatcher) matchPermission(toolName string, decisionClass string) bool {
	return matchStringField(m.ToolName, toolName) &&
		matchStringField(m.DecisionClass, decisionClass)
}

func normalizeHookMatcher(matcher HookMatcher) HookMatcher {
	normalized := HookMatcher{
		AgentName:        strings.TrimSpace(matcher.AgentName),
		AgentType:        strings.TrimSpace(matcher.AgentType),
		WorkspaceID:      strings.TrimSpace(matcher.WorkspaceID),
		WorkspaceRoot:    strings.TrimSpace(matcher.WorkspaceRoot),
		SessionType:      strings.TrimSpace(matcher.SessionType),
		SandboxID:        strings.TrimSpace(matcher.SandboxID),
		SandboxBackend:   strings.TrimSpace(matcher.SandboxBackend),
		SandboxProfile:   strings.TrimSpace(matcher.SandboxProfile),
		SyncDirection:    strings.TrimSpace(matcher.SyncDirection),
		InputClass:       strings.TrimSpace(matcher.InputClass),
		ACPEventType:     strings.TrimSpace(matcher.ACPEventType),
		TurnID:           strings.TrimSpace(matcher.TurnID),
		ToolID:           strings.TrimSpace(matcher.ToolID),
		ToolName:         strings.TrimSpace(matcher.ToolName),
		DecisionClass:    strings.TrimSpace(matcher.DecisionClass),
		MessageRole:      strings.TrimSpace(matcher.MessageRole),
		MessageDeltaType: strings.TrimSpace(matcher.MessageDeltaType),
	}
	normalized.NetworkMatcher = normalizeNetworkMatcher(matcher.NetworkMatcher)
	normalized.CompactionMatcher = normalizeCompactionMatcher(matcher.CompactionMatcher)
	normalized.Autonomy = normalizeAutonomyMatcher(matcher.Autonomy)
	if matcher.ToolReadOnly != nil {
		value := *matcher.ToolReadOnly
		normalized.ToolReadOnly = &value
	}
	return normalized
}

func normalizeCompactionMatcher(matcher *CompactionMatcher) *CompactionMatcher {
	if matcher == nil {
		return nil
	}
	normalized := CompactionMatcher{
		Reason:   strings.TrimSpace(matcher.Reason),
		Strategy: strings.TrimSpace(matcher.Strategy),
	}
	if normalized.empty() {
		return nil
	}
	return &normalized
}

func normalizeAutonomyMatcher(matcher *AutonomyMatcher) *AutonomyMatcher {
	if matcher == nil {
		return nil
	}
	normalized := AutonomyMatcher{
		TaskID:               strings.TrimSpace(matcher.TaskID),
		RunID:                strings.TrimSpace(matcher.RunID),
		LoopRunID:            strings.TrimSpace(matcher.LoopRunID),
		LoopName:             strings.TrimSpace(matcher.LoopName),
		NodeID:               strings.TrimSpace(matcher.NodeID),
		WorkflowID:           strings.TrimSpace(matcher.WorkflowID),
		ParticipationChannel: strings.TrimSpace(matcher.ParticipationChannel),
		CoordinatorSessionID: strings.TrimSpace(matcher.CoordinatorSessionID),
		ParentSessionID:      strings.TrimSpace(matcher.ParentSessionID),
		RootSessionID:        strings.TrimSpace(matcher.RootSessionID),
		ChildSessionID:       strings.TrimSpace(matcher.ChildSessionID),
		SpawnRole:            strings.TrimSpace(matcher.SpawnRole),
		ReleaseReason:        strings.TrimSpace(matcher.ReleaseReason),
	}
	if (&normalized).empty() {
		return nil
	}
	return &normalized
}

func (m *CompactionMatcher) empty() bool {
	return m.Reason == "" &&
		m.Strategy == ""
}

func (m *AutonomyMatcher) empty() bool {
	return m.TaskID == "" &&
		m.RunID == "" &&
		m.LoopRunID == "" &&
		m.LoopName == "" &&
		m.NodeID == "" &&
		m.WorkflowID == "" &&
		m.ParticipationChannel == "" &&
		m.CoordinatorSessionID == "" &&
		m.ParentSessionID == "" &&
		m.RootSessionID == "" &&
		m.ChildSessionID == "" &&
		m.SpawnRole == "" &&
		m.ReleaseReason == ""
}
