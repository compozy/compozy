package acp

import (
	"strings"
)

// PromptSyntheticMeta captures stable daemon-owned metadata for one synthetic prompt turn.
type PromptSyntheticMeta struct {
	TaskID               string          `json:"task_id,omitempty"`
	TaskRunID            string          `json:"task_run_id,omitempty"`
	WorkflowID           string          `json:"workflow_id,omitempty"`
	ClaimTokenHash       string          `json:"claim_token_hash,omitempty"`
	CoordinatorSessionID string          `json:"coordinator_session_id,omitempty"`
	ChildSessionID       string          `json:"child_session_id,omitempty"`
	ChildAgentName       string          `json:"child_agent_name,omitempty"`
	Badge                string          `json:"badge,omitempty"`
	CallID               string          `json:"call_id,omitempty"`
	CallState            string          `json:"call_state,omitempty"`
	ResultRef            string          `json:"result_ref,omitempty"`
	ResultBytes          int             `json:"result_bytes,omitempty"`
	ContractDigest       string          `json:"contract_digest,omitempty"`
	MessageID            string          `json:"message_id,omitempty"`
	DeliveryKind         string          `json:"delivery_kind,omitempty"`
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
		ChildSessionID:       strings.TrimSpace(m.ChildSessionID),
		ChildAgentName:       strings.TrimSpace(m.ChildAgentName),
		Badge:                strings.TrimSpace(m.Badge),
		CallID:               strings.TrimSpace(m.CallID),
		CallState:            strings.TrimSpace(m.CallState),
		ResultRef:            strings.TrimSpace(m.ResultRef),
		ResultBytes:          m.ResultBytes,
		ContractDigest:       strings.TrimSpace(m.ContractDigest),
		MessageID:            strings.TrimSpace(m.MessageID),
		DeliveryKind:         strings.TrimSpace(m.DeliveryKind),
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
	return normalized.identityFieldsZero() && normalized.deliveryFieldsZero() && normalized.Goal == nil
}

func (m PromptSyntheticMeta) identityFieldsZero() bool {
	return m.TaskID == "" && m.TaskRunID == "" && m.WorkflowID == "" &&
		m.ClaimTokenHash == "" && m.CoordinatorSessionID == "" &&
		m.ChildSessionID == "" && m.ChildAgentName == "" && m.Badge == "" &&
		m.PolicySnapshotID == "" && m.PolicyDigest == "" && m.ConfigDigest == ""
}

func (m PromptSyntheticMeta) deliveryFieldsZero() bool {
	return m.CallID == "" && m.CallState == "" && m.ResultRef == "" &&
		m.ResultBytes == 0 && m.ContractDigest == "" && m.MessageID == "" &&
		m.DeliveryKind == "" && m.Reason == "" && m.Summary == "" && m.WakeEventID == ""
}

// Validate ensures the synthetic metadata carries the minimum wake-up identity.
func (m PromptSyntheticMeta) Validate() error {
	normalized := m.Normalize()
	if normalized.Reason == "" {
		return invalidPromptMetadata("acp: synthetic prompt metadata requires a reason")
	}
	if normalized.Goal != nil {
		return normalized.Goal.Validate()
	}
	return nil
}
