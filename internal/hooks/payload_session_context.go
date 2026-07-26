package hooks

import "strings"

func (p SessionPreCreatePayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p SessionLifecyclePayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p SessionMessagePersistedPayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p SandboxPreparePayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p SandboxReadyPayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p SandboxSyncBeforePayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p SandboxSyncAfterPayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p SandboxStopPayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p InputPreSubmitPayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p PromptPayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p EventRecordPayload) hookSessionContext() SessionContext { return p.SessionContext }

func (p AgentPreStartPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p AgentLifecyclePayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p AgentHeartbeatWakeBeforePayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p AgentHeartbeatWakeAfterPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p SessionHealthUpdateAfterPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p TurnPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p MessagePayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p ToolPreCallPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p ToolPostCallPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p ToolPostErrorPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p PermissionRequestPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p PermissionResolutionPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p ContextCompactPayload) hookSessionContext() SessionContext {
	return p.SessionContext
}

func (p CoordinatorPreSpawnPayload) hookSessionContext() SessionContext {
	return SessionContext{
		SessionID:   p.CoordinatorSessionID,
		AgentName:   p.AgentName,
		WorkspaceID: p.WorkspaceID,
		Workspace:   p.Workspace,
	}
}

func (p CoordinatorLifecyclePayload) hookSessionContext() SessionContext {
	return SessionContext{
		SessionID:   p.CoordinatorSessionID,
		AgentName:   p.AgentName,
		WorkspaceID: p.WorkspaceID,
		Workspace:   p.Workspace,
	}
}

func (p TaskBlockPayload) hookSessionContext() SessionContext {
	return taskSessionContext(p.TaskContext)
}

func (p TaskAttentionPayload) hookSessionContext() SessionContext {
	return taskSessionContext(p.TaskContext)
}

func (p TaskStatusChangedPayload) hookSessionContext() SessionContext {
	return taskSessionContext(p.TaskContext)
}

func (p LoopLifecyclePayload) hookSessionContext() SessionContext {
	return loopSessionContext(p.LoopContext)
}

func (p LoopGenerationPayload) hookSessionContext() SessionContext {
	return loopSessionContext(p.LoopContext)
}

func (p LoopGatePayload) hookSessionContext() SessionContext {
	return loopSessionContext(p.LoopContext)
}

func (p LoopNodeTerminalPayload) hookSessionContext() SessionContext {
	return loopSessionContext(p.LoopContext)
}

func (p SpawnPreCreatePayload) hookSessionContext() SessionContext {
	return spawnSessionContext(p.SpawnContext)
}

func (p SpawnLifecyclePayload) hookSessionContext() SessionContext {
	return spawnSessionContext(p.SpawnContext)
}

func taskSessionContext(ctx TaskContext) SessionContext {
	return SessionContext{
		AgentName:   ctx.AgentName,
		WorkspaceID: ctx.WorkspaceID,
	}
}

func loopSessionContext(ctx LoopContext) SessionContext {
	return SessionContext{
		SessionID:   ctx.SessionID,
		AgentName:   ctx.AgentName,
		WorkspaceID: ctx.WorkspaceID,
	}
}

func spawnSessionContext(ctx SpawnContext) SessionContext {
	return SessionContext{
		SessionID:          ctx.ChildSessionID,
		AgentName:          ctx.AgentName,
		WorkspaceID:        ctx.WorkspaceID,
		Workspace:          ctx.Workspace,
		SessionSoulContext: optionalSessionSoulContext(ctx.SoulSnapshotID, ctx.SoulDigest),
	}
}

func optionalSessionSoulContext(snapshotID string, digest string) *SessionSoulContext {
	trimmedSnapshotID := strings.TrimSpace(snapshotID)
	trimmedDigest := strings.TrimSpace(digest)
	if trimmedSnapshotID == "" && trimmedDigest == "" {
		return nil
	}
	return &SessionSoulContext{
		SoulSnapshotID: trimmedSnapshotID,
		SoulDigest:     trimmedDigest,
	}
}
