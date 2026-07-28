package acp

import (
	"errors"
	"strings"
)

// PromptSyntheticMeta captures stable daemon-owned metadata for one synthetic prompt turn.
type PromptSyntheticMeta struct {
	TaskID               string          `json:"task_id,omitempty"`
	TaskRunID            string          `json:"task_run_id,omitempty"`
	WorkflowID           string          `json:"workflow_id,omitempty"`
	ClaimTokenHash       string          `json:"claim_token_hash,omitempty"`
	CoordinatorSessionID string          `json:"coordinator_session_id,omitempty"`
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
		TaskID:               strings.TrimSpace(m.TaskID),
		TaskRunID:            strings.TrimSpace(m.TaskRunID),
		WorkflowID:           strings.TrimSpace(m.WorkflowID),
		ClaimTokenHash:       strings.TrimSpace(m.ClaimTokenHash),
		CoordinatorSessionID: strings.TrimSpace(m.CoordinatorSessionID),
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
	return normalized.TaskID == "" && normalized.TaskRunID == "" && normalized.WorkflowID == "" &&
		normalized.ClaimTokenHash == "" && normalized.CoordinatorSessionID == "" && normalized.Reason == "" &&
		normalized.Summary == "" && normalized.WakeEventID == "" && normalized.PolicySnapshotID == "" &&
		normalized.PolicyDigest == "" && normalized.ConfigDigest == "" && normalized.Goal == nil
}

// Validate ensures the synthetic metadata carries the minimum wake-up identity.
func (m PromptSyntheticMeta) Validate() error {
	normalized := m.Normalize()
	if normalized.Reason == "" {
		return errors.New("acp: synthetic prompt metadata requires a reason")
	}
	if normalized.Goal != nil {
		return normalized.Goal.Validate()
	}
	return nil
}
