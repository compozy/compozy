package hooks

import "context"

type SubagentSpawnPayload struct {
	Isolation  string `json:"isolation"`
	WorktreeID string `json:"worktree_id,omitempty"`
	Title      string `json:"title"`
	Role       string `json:"role"`
	TaskChars  int    `json:"task_chars"`
}

type SubagentRuntimePayload struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type SubagentSettledPayload struct {
	PayloadBase
	ProfileID       string                 `json:"profile_id,omitempty"`
	WorkspaceID     string                 `json:"workspace_id"`
	SubagentID      string                 `json:"subagent_id"`
	ParentSessionID string                 `json:"parent_session_id"`
	ChildSessionID  *string                `json:"child_session_id"`
	Origin          string                 `json:"origin"`
	Status          string                 `json:"status"`
	Runtime         SubagentRuntimePayload `json:"runtime"`
	DurationMS      int64                  `json:"duration_ms"`
}

func (p SubagentSettledPayload) HookProfileID() string { return p.ProfileID }

type SubagentObservationPatch = AutonomyObservationPatch

func subagentHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookSubagentSettled: {
		Event: HookSubagentSettled, Family: HookEventFamilySubagent,
		PayloadSchema: "SubagentSettledPayload", PatchSchema: "SubagentObservationPatch",
	}}
}

func (h *Hooks) DispatchSubagentSettled(
	ctx context.Context,
	payload SubagentSettledPayload,
) (SubagentSettledPayload, error) {
	return h.executeDispatch(ctx, HookSubagentSettled, payload,
		dispatchConfig[SubagentSettledPayload, SubagentObservationPatch]{
			match: func(m HookMatcher, p SubagentSettledPayload) bool {
				return m.MatchesSpawn(
					SpawnContext{
						ParentSessionID: p.ParentSessionID,
						ChildSessionID:  subagentChildID(p),
						WorkspaceID:     p.WorkspaceID,
						SpawnRole:       "subagent",
					},
				)
			},
			apply: applyNoop[SubagentSettledPayload, SubagentObservationPatch],
		})
}

func subagentChildID(p SubagentSettledPayload) string {
	if p.ChildSessionID == nil {
		return ""
	}
	return *p.ChildSessionID
}

func (p SubagentSettledPayload) cloneForAsync() SubagentSettledPayload {
	if p.ChildSessionID != nil {
		p.ChildSessionID = new(*p.ChildSessionID)
	}
	return p
}

func (p SubagentSettledPayload) hookSessionContext() SessionContext {
	return SessionContext{ProfileID: p.ProfileID, SessionID: p.ParentSessionID, WorkspaceID: p.WorkspaceID}
}
