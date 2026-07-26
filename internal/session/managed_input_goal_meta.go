package session

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

func goalPromptMetaFromManagedInput(meta ManagedInputPromptMeta) (*acp.GoalPromptMeta, error) {
	var kind string
	switch strings.TrimSpace(meta.Kind) {
	case managedInputPromptKindWork:
		kind = acp.GoalPromptKindWork
	case managedInputPromptKindContinuation:
		kind = acp.GoalPromptKindContinuation
	case managedInputPromptKindCompact:
		kind = acp.GoalPromptKindCompaction
	default:
		return nil, fmt.Errorf("session: unsupported managed Goal prompt kind %q", meta.Kind)
	}
	goalMeta := acp.GoalPromptMeta{
		Kind:          kind,
		RunID:         meta.LoopRunID,
		NodeID:        meta.NodeID,
		Generation:    int64(meta.Generation),
		ItemIndex:     meta.ItemIndex,
		Turn:          meta.Turn,
		PromptAttempt: meta.PromptAttempt,
		PromptID:      meta.PromptID,
	}.Normalize()
	if err := goalMeta.Validate(); err != nil {
		return nil, fmt.Errorf("session: invalid managed Goal prompt metadata: %w", err)
	}
	return &goalMeta, nil
}

func goalPromptMetaFromPromptMeta(meta acp.PromptMeta) *acp.GoalPromptMeta {
	normalized := meta.Normalize()
	if normalized.Synthetic == nil {
		return nil
	}
	return acp.CloneGoalPromptMeta(normalized.Synthetic.Goal)
}
