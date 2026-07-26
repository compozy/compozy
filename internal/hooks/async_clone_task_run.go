package hooks

import "github.com/compozy/agh/internal/network/participation"

func cloneTaskRunEnqueuedPayload(payload TaskRunEnqueuedPayload) TaskRunEnqueuedPayload {
	payload.TaskRunContext = cloneTaskRunContext(payload.TaskRunContext)
	return payload
}

func cloneTaskRunPreClaimPayload(payload TaskRunPreClaimPayload) TaskRunPreClaimPayload {
	if payload.TaskRunContext != nil {
		contextSnapshot := cloneTaskRunContext(*payload.TaskRunContext)
		payload.TaskRunContext = &contextSnapshot
	}
	payload.Criteria.RequiredCapabilities = cloneStringSlice(payload.Criteria.RequiredCapabilities)
	return payload
}

func cloneTaskRunContext(payload TaskRunContext) TaskRunContext {
	if payload.RunKind != nil {
		runKind := *payload.RunKind
		payload.RunKind = &runKind
	}
	if payload.ResolvedNetworkParticipation != nil {
		payload.ResolvedNetworkParticipation = participation.CloneSpec(*payload.ResolvedNetworkParticipation)
	}
	return payload
}
