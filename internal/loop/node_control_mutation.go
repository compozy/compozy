package loop

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// NodeControlMutationKind identifies one coordinator-owned control transition.
type NodeControlMutationKind string

const (
	NodeControlMutationPause      NodeControlMutationKind = "pause"
	NodeControlMutationQuarantine NodeControlMutationKind = "quarantine"
	NodeControlMutationAttention  NodeControlMutationKind = "attention"
	NodeControlMutationCancel     NodeControlMutationKind = "cancel"
)

// NodeControlMutation is persisted with the generation snapshot that caused it.
type NodeControlMutation struct {
	Kind             NodeControlMutationKind `json:"kind"`
	NodeID           NodeID                  `json:"node_id"`
	ExpectedRevision int64                   `json:"expected_revision"`
	ExpectExisting   bool                    `json:"expect_existing,omitempty"`
	QuarantineEntry  json.RawMessage         `json:"quarantine_entry,omitempty"`
	AttentionFlag    string                  `json:"attention_flag,omitempty"`
	AttentionReason  string                  `json:"attention_reason,omitempty"`
	CancelState      CancelState             `json:"cancel_state,omitempty"`
	PauseActorKind   string                  `json:"pause_actor_kind,omitempty"`
	PauseActorID     string                  `json:"pause_actor_id,omitempty"`
	PauseReason      string                  `json:"pause_reason,omitempty"`
	PauseRuleID      string                  `json:"pause_rule_id,omitempty"`
	At               time.Time               `json:"at"`
}

func (m NodeControlMutation) normalized() NodeControlMutation {
	m.Kind = NodeControlMutationKind(strings.TrimSpace(string(m.Kind)))
	m.NodeID = NodeID(strings.TrimSpace(string(m.NodeID)))
	m.AttentionFlag = strings.TrimSpace(m.AttentionFlag)
	m.AttentionReason = strings.TrimSpace(m.AttentionReason)
	m.CancelState = CancelState(strings.TrimSpace(string(m.CancelState)))
	m.PauseActorKind = strings.TrimSpace(m.PauseActorKind)
	m.PauseActorID = strings.TrimSpace(m.PauseActorID)
	m.PauseReason = strings.TrimSpace(m.PauseReason)
	m.PauseRuleID = strings.TrimSpace(m.PauseRuleID)
	if len(m.QuarantineEntry) > 0 {
		m.QuarantineEntry = append(json.RawMessage(nil), m.QuarantineEntry...)
	}
	if !m.At.IsZero() {
		m.At = m.At.UTC()
	}
	return m
}

func (m NodeControlMutation) validate() error {
	if m.NodeID == "" || m.ExpectedRevision < 0 || m.At.IsZero() {
		return fmt.Errorf("%w: node control mutation identity is incomplete", ErrValidation)
	}
	switch m.Kind {
	case NodeControlMutationPause:
		if m.PauseActorKind == "" || m.PauseActorID == "" || m.PauseReason == "" || m.PauseRuleID == "" {
			return fmt.Errorf("%w: pause mutation provenance is incomplete", ErrValidation)
		}
	case NodeControlMutationQuarantine:
		if len(m.QuarantineEntry) == 0 || !json.Valid(m.QuarantineEntry) {
			return fmt.Errorf("%w: quarantine mutation entry must be valid JSON", ErrValidation)
		}
	case NodeControlMutationAttention:
		if m.AttentionFlag != attentionDependencyFlag || m.AttentionReason == "" {
			return fmt.Errorf("%w: attention mutation requires a quarantined dependency", ErrValidation)
		}
	case NodeControlMutationCancel:
		if !m.ExpectExisting || m.CancelState != CancelStateCanceled {
			return fmt.Errorf("%w: cancel mutation must close an existing node control", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: node control mutation kind is invalid: %q", ErrValidation, m.Kind)
	}
	return nil
}
