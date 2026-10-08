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
	if !matchStringField(m.WorktreeID, payload.WorktreeIDValue()) {
		return false
	}
	if includeSessionType && !matchStringField(m.SessionType, payload.SessionType) {
		return false
	}
	return true
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
		WorktreeID:       strings.TrimSpace(matcher.WorktreeID),
		WorkspaceRoot:    strings.TrimSpace(matcher.WorkspaceRoot),
		SessionType:      strings.TrimSpace(matcher.SessionType),
		InputClass:       strings.TrimSpace(matcher.InputClass),
		ACPEventType:     strings.TrimSpace(matcher.ACPEventType),
		TurnID:           strings.TrimSpace(matcher.TurnID),
		ToolID:           strings.TrimSpace(matcher.ToolID),
		ToolName:         strings.TrimSpace(matcher.ToolName),
		DecisionClass:    strings.TrimSpace(matcher.DecisionClass),
		MessageRole:      strings.TrimSpace(matcher.MessageRole),
		MessageDeltaType: strings.TrimSpace(matcher.MessageDeltaType),

		CompactionMatcher: normalizeCompactionMatcher(matcher.CompactionMatcher),
		Autonomy:          normalizeAutonomyMatcher(matcher.Autonomy)}
	if matcher.ToolReadOnly != nil {
		normalized.ToolReadOnly = new(*matcher.ToolReadOnly)
	}
	return normalized
}

func normalizeCompactionMatcher(matcher *CompactionMatcher) *CompactionMatcher {
	if matcher == nil {
		return nil
	}
	normalized := CompactionMatcher{
		Trigger: strings.TrimSpace(matcher.Trigger),
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
	return m.Trigger == ""
}

func (m *AutonomyMatcher) empty() bool {
	return m.TaskID == "" &&
		m.RunID == "" &&
		m.LoopRunID == "" &&
		m.LoopName == "" &&
		m.NodeID == "" &&
		m.WorkflowID == "" &&
		m.CoordinatorSessionID == "" &&
		m.ParentSessionID == "" &&
		m.RootSessionID == "" &&
		m.ChildSessionID == "" &&
		m.SpawnRole == "" &&
		m.ReleaseReason == ""
}
