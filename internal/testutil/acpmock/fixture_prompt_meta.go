package acpmock

import (
	"errors"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

// Normalize returns an isolated, trimmed Goal matcher.
func (m TurnMatchGoal) Normalize() TurnMatchGoal {
	normalized := m
	normalized.Kind = strings.TrimSpace(m.Kind)
	normalized.RunID = strings.TrimSpace(m.RunID)
	normalized.NodeID = strings.TrimSpace(m.NodeID)
	normalized.PromptID = strings.TrimSpace(m.PromptID)
	normalized.ItemIndex = cloneInt(m.ItemIndex)
	normalized.Turn = cloneInt(m.Turn)
	normalized.PromptAttempt = cloneInt(m.PromptAttempt)
	return normalized
}

// IsZero reports whether the matcher selects no Goal fields.
func (m TurnMatchGoal) IsZero() bool {
	normalized := m.Normalize()
	return normalized.Kind == "" && normalized.RunID == "" && normalized.NodeID == "" &&
		normalized.Generation == 0 && normalized.ItemIndex == nil && normalized.Turn == nil &&
		normalized.PromptAttempt == nil && normalized.PromptID == ""
}

// Validate ensures at least one exact Goal selector is present.
func (m TurnMatchGoal) Validate(_ string) error {
	if m.IsZero() {
		return errors.New("acpmock: Goal matcher requires at least one exact selector")
	}
	return nil
}

func (m TurnMatchGoal) matches(meta acp.GoalPromptMeta) bool {
	want := m.Normalize()
	got := meta.Normalize()
	return matchesString(want.Kind, got.Kind) &&
		matchesString(want.RunID, got.RunID) &&
		matchesString(want.NodeID, got.NodeID) &&
		(want.Generation == 0 || want.Generation == got.Generation) &&
		matchesOptionalInt(want.ItemIndex, got.ItemIndex) &&
		matchesOptionalIntPointer(want.Turn, got.Turn) &&
		matchesOptionalInt(want.PromptAttempt, got.PromptAttempt) &&
		matchesString(want.PromptID, got.PromptID)
}

// Normalize returns a trimmed judge matcher.
func (m TurnMatchJudge) Normalize() TurnMatchJudge {
	m.Role = strings.TrimSpace(m.Role)
	m.CorrelationID = strings.TrimSpace(m.CorrelationID)
	m.GateID = strings.TrimSpace(m.GateID)
	m.CriterionID = strings.TrimSpace(m.CriterionID)
	return m
}

// IsZero reports whether the matcher selects no judge fields.
func (m TurnMatchJudge) IsZero() bool {
	normalized := m.Normalize()
	return normalized.Role == "" && normalized.Attempt == 0 && normalized.CorrelationID == "" &&
		normalized.GateID == "" && normalized.CriterionID == ""
}

// Validate ensures at least one exact judge selector is present.
func (m TurnMatchJudge) Validate(_ string) error {
	if m.Attempt < 0 {
		return errors.New("acpmock: judge matcher attempt must be non-negative")
	}
	if m.IsZero() {
		return errors.New("acpmock: judge matcher requires at least one exact selector")
	}
	return nil
}

func (m TurnMatchJudge) matches(meta acp.PromptJudgeMeta) bool {
	want := m.Normalize()
	got := meta.Normalize()
	return matchesString(want.Role, got.Role) &&
		(want.Attempt == 0 || want.Attempt == got.Attempt) &&
		matchesString(want.CorrelationID, got.CorrelationID) &&
		matchesString(want.GateID, got.GateID) &&
		matchesString(want.CriterionID, got.CriterionID)
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func matchesString(want, got string) bool {
	return want == "" || want == got
}

func matchesOptionalInt(want *int, got int) bool {
	return want == nil || *want == got
}

func matchesOptionalIntPointer(want, got *int) bool {
	if want == nil {
		return true
	}
	return got != nil && *want == *got
}
