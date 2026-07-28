package acp

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// PromptJudgeRoleAgent identifies one daemon-owned agent judge prompt.
	PromptJudgeRoleAgent = "agent-judge"
)

// PromptJudgeMeta carries stable routing identity for one daemon-owned judge prompt.
type PromptJudgeMeta struct {
	Role          string `json:"role"`
	Attempt       int    `json:"attempt"`
	CorrelationID string `json:"correlation_id"`
	GateID        string `json:"gate_id"`
	CriterionID   string `json:"criterion_id"`
}

// Normalize returns a trimmed copy.
func (m PromptJudgeMeta) Normalize() PromptJudgeMeta {
	m.Role = strings.TrimSpace(m.Role)
	m.CorrelationID = strings.TrimSpace(m.CorrelationID)
	m.GateID = strings.TrimSpace(m.GateID)
	m.CriterionID = strings.TrimSpace(m.CriterionID)
	return m
}

// IsZero reports whether the metadata carries no judge identity.
func (m PromptJudgeMeta) IsZero() bool {
	normalized := m.Normalize()
	return normalized.Role == "" && normalized.Attempt == 0 && normalized.CorrelationID == "" &&
		normalized.GateID == "" && normalized.CriterionID == ""
}

// Validate enforces the closed judge prompt metadata contract.
func (m PromptJudgeMeta) Validate() error {
	normalized := m.Normalize()
	if normalized.Role != PromptJudgeRoleAgent {
		return fmt.Errorf("acp: invalid judge prompt role %q", normalized.Role)
	}
	if normalized.Attempt < 1 {
		return errors.New("acp: judge prompt attempt must be positive")
	}
	if normalized.CorrelationID == "" || normalized.GateID == "" || normalized.CriterionID == "" {
		return errors.New("acp: judge prompt metadata identity is incomplete")
	}
	return nil
}
