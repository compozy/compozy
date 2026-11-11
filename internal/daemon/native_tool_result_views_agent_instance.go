package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
)

type compactAgentInstanceRecord struct {
	ID                   string                          `json:"id"`
	AgentName            string                          `json:"agent_name"`
	Label                string                          `json:"label"`
	WorkspaceID          string                          `json:"workspace_id"`
	Scope                string                          `json:"scope"`
	ProfileID            string                          `json:"profile_id"`
	ProfileName          string                          `json:"profile_name"`
	MainSessionID        string                          `json:"main_session_id"`
	State                string                          `json:"state"`
	PauseReason          string                          `json:"pause_reason"`
	Activity             string                          `json:"activity"`
	NeedsAttention       bool                            `json:"needs_attention"`
	AttentionDismissible bool                            `json:"attention_dismissible"`
	AuthorityStatus      string                          `json:"authority_status"`
	ChecksEnabled        bool                            `json:"checks_enabled"`
	Created              bool                            `json:"created"`
	NextWake             *contract.AgentInstanceNextWake `json:"next_wake,omitempty"`
	HeldWakes            int64                           `json:"held_wakes"`
	FailureStreak        int64                           `json:"failure_streak"`
	UnattendedTurns      int64                           `json:"unattended_turns"`
	UnattendedTurnLimit  int64                           `json:"unattended_turn_limit"`
	FatigueLimit         int                             `json:"fatigue_limit"`
	Sources              []compactAgentInstanceSource    `json:"sources"`
}

func compactAgentInstance(p contract.AgentInstancePayload) compactAgentInstanceRecord {
	result := compactAgentInstanceRecord{
		ID:                   p.ID,
		AgentName:            p.AgentName,
		Label:                p.Label,
		WorkspaceID:          p.WorkspaceID,
		Scope:                p.Scope,
		ProfileID:            p.ProfileID,
		ProfileName:          p.ProfileName,
		MainSessionID:        p.MainSessionID,
		State:                p.State,
		PauseReason:          p.PauseReason,
		Activity:             p.Activity,
		NeedsAttention:       p.NeedsAttention,
		AttentionDismissible: p.AttentionDismissible,
		AuthorityStatus:      p.AuthorityStatus,
		ChecksEnabled:        p.ChecksEnabled,
		Created:              p.Created,
		NextWake:             p.NextWake,
		HeldWakes:            p.HeldWakes,
		FailureStreak:        p.FailureStreak,
		UnattendedTurns:      p.UnattendedTurns,
		UnattendedTurnLimit:  p.UnattendedTurnLimit,
		FatigueLimit:         p.FatigueLimit,
	}
	if p.Sources != nil {
		result.Sources = make([]compactAgentInstanceSource, len(p.Sources))
		for i, value := range p.Sources {
			result.Sources[i] = compactAgentSource(value)
		}
	}
	return result
}

type compactAgentInstanceSource struct {
	ID               string        `json:"id"`
	Kind             string        `json:"kind,omitempty"`
	Script           string        `json:"script,omitempty"`
	Status           string        `json:"status"`
	FailureStreak    int64         `json:"failure_streak"`
	Error            *source.Error `json:"error,omitempty"`
	AwaitingApproval bool          `json:"awaiting_approval,omitzero"`
	NextPollAt       string        `json:"next_poll_at,omitempty"`
}

func compactAgentSource(p heartbeat.HeartbeatSourceStatus) compactAgentInstanceSource {
	return compactAgentInstanceSource{
		ID:               p.ID,
		Kind:             p.Kind,
		Script:           p.Script,
		Status:           p.Status,
		FailureStreak:    p.FailureStreak,
		Error:            p.Error,
		AwaitingApproval: p.AwaitingApproval,
		NextPollAt:       p.NextPollAt,
	}
}

var agentInstanceRecordViews = resultViews[contract.AgentInstancePayload]{
	Compact: func(p contract.AgentInstancePayload) any { return map[string]any{"instance": compactAgentInstance(p)} },
	Full:    func(p contract.AgentInstancePayload) any { return contract.AgentInstanceResponse{Instance: p} },
	Preview: func(contract.AgentInstancePayload) string { return "Instance updated" },
}

var agentInstanceCreateViews = func() resultViews[contract.AgentInstancePayload] {
	views := agentInstanceRecordViews
	views.Preview = func(contract.AgentInstancePayload) string { return "Instance created" }
	return views
}()

type compactAgentInstanceList struct {
	Peers              []contract.AgentInstancePeer `json:"peers,omitempty"`
	MaxActiveInstances int                          `json:"max_active_instances"`
	ActiveCount        int64                        `json:"active_count"`
	Instances          []compactAgentInstanceRecord `json:"instances"`
	NextCursor         string                       `json:"next_cursor"`
}

var agentInstanceListViews = resultViews[contract.AgentInstanceListResponse]{
	Compact: func(p contract.AgentInstanceListResponse) any {
		result := compactAgentInstanceList{
			Peers:              p.Peers,
			MaxActiveInstances: p.MaxActiveInstances,
			ActiveCount:        p.ActiveCount,
			NextCursor:         p.NextCursor,
		}
		if p.Instances != nil {
			result.Instances = make([]compactAgentInstanceRecord, len(p.Instances))
			for i, value := range p.Instances {
				result.Instances[i] = compactAgentInstance(value)
			}
		}
		return result
	},
	Preview: func(contract.AgentInstanceListResponse) string { return "Agent instances" },
}

var agentInstancePeersViews = func() resultViews[contract.AgentInstanceListResponse] {
	views := agentInstanceListViews
	views.Preview = func(contract.AgentInstanceListResponse) string { return "Reachable instance peers" }
	return views
}()
