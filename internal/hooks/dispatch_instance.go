package hooks

import "context"

type AgentInstancePayload struct {
	Actor         string `json:"actor,omitempty"`
	ActionID      string `json:"action_id,omitempty"`
	ReportID      string `json:"report_id,omitempty"`
	ProposalIndex *int   `json:"proposal_index,omitempty"`
	PayloadBase
	SessionContext
	InstanceID    string `json:"instance_id"`
	MainSessionID string `json:"main_session_id,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type AgentInstanceObservationPatch struct{}

func (h *Hooks) DispatchAgentInstance(ctx context.Context, payload AgentInstancePayload) (AgentInstancePayload, error) {
	return h.executeDispatch(
		ctx,
		payload.Event,
		payload,
		dispatchConfig[AgentInstancePayload, AgentInstanceObservationPatch]{
			match: func(matcher HookMatcher, payload AgentInstancePayload) bool {
				return matcher.MatchesSession(payload.SessionContext)
			},
			apply: applyNoop[AgentInstancePayload, AgentInstanceObservationPatch],
		},
	)
}

func instanceHookEventDescriptors() map[HookEvent]EventDescriptor {
	result := make(map[HookEvent]EventDescriptor)
	for _, event := range []HookEvent{
		HookAgentInstanceCreated, HookAgentInstancePaused, HookAgentInstanceResumed,
		HookAgentInstanceArchived, HookAgentInstanceAdopted, HookAgentInstanceProposalPicked,
	} {
		result[event] = EventDescriptor{
			Event:         event,
			Family:        HookEventFamilyAgent,
			PayloadSchema: "AgentInstancePayload",
			PatchSchema:   "AgentInstanceObservationPatch",
		}
	}
	return result
}
