package acp

import (
	"errors"
	"fmt"
	"strings"
)

const (
	GoalPromptKindWork         = "goal-work"
	GoalPromptKindContinuation = "goal-continuation"
	GoalPromptKindCompaction   = "goal-compaction"
)

// GoalPromptMeta classifies one engine-owned Goal prompt without importing Loop.
type GoalPromptMeta struct {
	Kind          string `json:"kind"`
	RunID         string `json:"run_id"`
	NodeID        string `json:"node_id"`
	Generation    int64  `json:"generation"`
	ItemIndex     int    `json:"item_index"`
	Turn          *int   `json:"turn"`
	PromptAttempt int    `json:"prompt_attempt"`
	PromptID      string `json:"prompt_id"`
}

// Normalize returns a trimmed, isolated copy.
func (m GoalPromptMeta) Normalize() GoalPromptMeta {
	normalized := m
	normalized.Kind = strings.TrimSpace(m.Kind)
	normalized.RunID = strings.TrimSpace(m.RunID)
	normalized.NodeID = strings.TrimSpace(m.NodeID)
	normalized.PromptID = strings.TrimSpace(m.PromptID)
	if m.Turn != nil {
		turn := *m.Turn
		normalized.Turn = &turn
	}
	return normalized
}

// IsZero reports whether the metadata carries no Goal identity.
func (m GoalPromptMeta) IsZero() bool {
	normalized := m.Normalize()
	return normalized.Kind == "" && normalized.RunID == "" && normalized.NodeID == "" &&
		normalized.Generation == 0 && normalized.ItemIndex == 0 && normalized.Turn == nil &&
		normalized.PromptAttempt == 0 && normalized.PromptID == ""
}

// Validate enforces the closed public Goal prompt metadata contract.
func (m GoalPromptMeta) Validate() error {
	normalized := m.Normalize()
	if normalized.RunID == "" || normalized.NodeID == "" || normalized.PromptID == "" {
		return errors.New("acp: Goal prompt metadata identity is incomplete")
	}
	if normalized.Generation < 1 {
		return errors.New("acp: Goal prompt metadata generation must be positive")
	}
	if normalized.ItemIndex < 0 || normalized.PromptAttempt < 0 {
		return errors.New("acp: Goal prompt metadata indices must be non-negative")
	}
	switch normalized.Kind {
	case GoalPromptKindWork, GoalPromptKindContinuation:
		if normalized.Turn == nil || *normalized.Turn < 1 {
			return errors.New("acp: Goal work prompt metadata requires a positive turn")
		}
	case GoalPromptKindCompaction:
		if normalized.Turn != nil {
			return errors.New("acp: Goal compaction prompt metadata requires a null turn")
		}
	default:
		return fmt.Errorf("acp: invalid Goal prompt kind %q", normalized.Kind)
	}
	return nil
}

// CloneGoalPromptMeta returns an isolated normalized pointer.
func CloneGoalPromptMeta(meta *GoalPromptMeta) *GoalPromptMeta {
	if meta == nil {
		return nil
	}
	cloned := meta.Normalize()
	if cloned.IsZero() {
		return nil
	}
	return &cloned
}
