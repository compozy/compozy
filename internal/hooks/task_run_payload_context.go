package hooks

func (p TaskRunEnqueuedPayload) hookSessionContext() SessionContext {
	return taskRunSessionContext(p.TaskRunContext)
}

func (p TaskRunPreClaimPayload) hookSessionContext() SessionContext {
	return taskRunSessionContext(p.taskRunContextSnapshot())
}

func (p TaskRunPreClaimPayload) taskRunContextSnapshot() TaskRunContext {
	if p.TaskRunContext == nil {
		return TaskRunContext{}
	}
	return *p.TaskRunContext
}

func (p TaskRunPostClaimPayload) hookSessionContext() SessionContext {
	return taskRunSessionContext(p.TaskRunContext)
}

func (p TaskRunLeasePayload) hookSessionContext() SessionContext {
	return taskRunSessionContext(p.TaskRunContext)
}

func taskRunSessionContext(ctx TaskRunContext) SessionContext {
	return SessionContext{
		SessionID:          ctx.SessionID,
		AgentName:          ctx.AgentName,
		WorkspaceID:        ctx.WorkspaceID,
		SessionSoulContext: optionalSessionSoulContext(ctx.SoulSnapshotID, ctx.SoulDigest),
	}
}
