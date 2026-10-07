package hooks

func cloneTaskRunEnqueuedPayload(payload TaskRunEnqueuedPayload) TaskRunEnqueuedPayload {
	payload.TaskRunContext = cloneTaskRunContext(payload.TaskRunContext)
	return payload
}

func cloneTaskRunPreClaimPayload(payload TaskRunPreClaimPayload) TaskRunPreClaimPayload {
	if payload.TaskRunContext != nil {
		payload.TaskRunContext = new(cloneTaskRunContext(*payload.TaskRunContext))
	}
	payload.Criteria.RequiredCapabilities = cloneStringSlice(payload.Criteria.RequiredCapabilities)
	return payload
}

func cloneTaskRunContext(payload TaskRunContext) TaskRunContext {
	if payload.RunKind != nil {
		payload.RunKind = new(*payload.RunKind)
	}
	return payload
}
