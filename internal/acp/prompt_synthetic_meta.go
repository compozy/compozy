package acp

import (
	"strings"
)

const PromptSyntheticKindSessionReply = "session_reply"

// PromptSyntheticMeta captures stable daemon-owned metadata for one synthetic prompt turn.
type PromptSyntheticMeta struct {
	Hop                  int             `json:"hop,omitzero"`
	ChildWorkspaceID     string          `json:"child_workspace_id,omitempty"`
	ReplyTruncated       bool            `json:"reply_truncated,omitzero"`
	Kind                 string          `json:"kind,omitempty"`
	SubagentIDs          []string        `json:"subagent_ids,omitempty"`
	TaskID               string          `json:"task_id,omitempty"`
	TaskRunID            string          `json:"task_run_id,omitempty"`
	WorkflowID           string          `json:"workflow_id,omitempty"`
	ClaimTokenHash       string          `json:"claim_token_hash,omitempty"`
	CoordinatorSessionID string          `json:"coordinator_session_id,omitempty"`
	ChildSessionID       string          `json:"child_session_id,omitempty"`
	ChildAgentName       string          `json:"child_agent_name,omitempty"`
	Badge                string          `json:"badge,omitempty"`
	Reason               string          `json:"reason,omitempty"`
	Summary              string          `json:"summary,omitempty"`
	WakeEventID          string          `json:"wake_event_id,omitempty"`
	PolicySnapshotID     string          `json:"policy_snapshot_id,omitempty"`
	PolicyDigest         string          `json:"policy_digest,omitempty"`
	ConfigDigest         string          `json:"config_digest,omitempty"`
	Goal                 *GoalPromptMeta `json:"goal,omitempty"`
}

// Normalize returns a trimmed copy of the synthetic metadata.
func (m PromptSyntheticMeta) Normalize() PromptSyntheticMeta {
	return PromptSyntheticMeta{
		Hop: m.Hop, ChildWorkspaceID: strings.TrimSpace(m.ChildWorkspaceID), ReplyTruncated: m.ReplyTruncated,
		Kind:                 strings.TrimSpace(m.Kind),
		SubagentIDs:          append([]string(nil), m.SubagentIDs...),
		TaskID:               strings.TrimSpace(m.TaskID),
		TaskRunID:            strings.TrimSpace(m.TaskRunID),
		WorkflowID:           strings.TrimSpace(m.WorkflowID),
		ClaimTokenHash:       strings.TrimSpace(m.ClaimTokenHash),
		CoordinatorSessionID: strings.TrimSpace(m.CoordinatorSessionID),
		ChildSessionID:       strings.TrimSpace(m.ChildSessionID),
		ChildAgentName:       strings.TrimSpace(m.ChildAgentName),
		Badge:                strings.TrimSpace(m.Badge),
		Reason:               strings.TrimSpace(m.Reason),
		Summary:              strings.TrimSpace(m.Summary),
		WakeEventID:          strings.TrimSpace(m.WakeEventID),
		PolicySnapshotID:     strings.TrimSpace(m.PolicySnapshotID),
		PolicyDigest:         strings.TrimSpace(m.PolicyDigest),
		ConfigDigest:         strings.TrimSpace(m.ConfigDigest),
		Goal:                 CloneGoalPromptMeta(m.Goal),
	}
}

// IsZero reports whether the synthetic metadata carries any fields.
func (m PromptSyntheticMeta) IsZero() bool {
	normalized := m.Normalize()
	return normalized.ChildWorkspaceID == "" && !normalized.ReplyTruncated && normalized.Hop == 0 &&
		normalized.Kind == "" && len(normalized.SubagentIDs) == 0 &&
		normalized.TaskID == "" && normalized.TaskRunID == "" && normalized.WorkflowID == "" &&
		normalized.ClaimTokenHash == "" &&
		normalized.CoordinatorSessionID == "" &&
		normalized.ChildSessionID == "" &&
		normalized.ChildAgentName == "" &&
		normalized.Badge == "" &&
		normalized.Reason == "" &&
		normalized.Summary == "" &&
		normalized.WakeEventID == "" &&
		normalized.PolicySnapshotID == "" &&
		normalized.PolicyDigest == "" &&
		normalized.ConfigDigest == "" &&
		normalized.Goal == nil
}

// Validate ensures the synthetic metadata carries the minimum wake-up identity.
func (m PromptSyntheticMeta) Validate() error {
	normalized := m.Normalize()
	if normalized.Hop < 0 || normalized.Hop > MaxSessionMessageHops {
		return invalidPromptMetadata("acp: invalid synthetic prompt hop")
	}
	if normalized.Kind != PromptSyntheticKindSessionReply && normalized.Hop != 0 {
		return invalidPromptMetadata("acp: only session replies carry a synthetic hop")
	}
	if normalized.Reason == "" {
		return invalidPromptMetadata("acp: synthetic prompt metadata requires a reason")
	}
	if normalized.Goal != nil {
		return normalized.Goal.Validate()
	}
	return nil
}
