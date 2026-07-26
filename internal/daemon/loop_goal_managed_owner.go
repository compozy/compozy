package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func managedGoalTaskMatchesOwner(
	run taskpkg.Run,
	metadata managedGoalTaskRunMetadata,
	owner session.ManagedInputOwner,
) bool {
	return run.LoopRunID == owner.LoopRunID &&
		metadata.Generation == owner.RunGeneration &&
		metadata.GoalSegmentEpoch == owner.ControlEpoch
}

func managedGoalCheckpointMatchesOwner(
	checkpoint goalpkg.Checkpoint,
	owner session.ManagedInputOwner,
) bool {
	return checkpoint.ControlEpoch == owner.ControlEpoch &&
		checkpoint.BindingEpoch == owner.BindingEpoch &&
		checkpoint.TaskRunID == owner.TaskRunID &&
		checkpoint.QueueEntryID == owner.QueueEntryID &&
		checkpoint.PromptID == owner.PromptID &&
		checkpoint.PromptAttempt == owner.PromptAttempt &&
		checkpoint.SessionID == owner.SessionID &&
		checkpoint.PromptKind == owner.PromptKind
}

func operationOwner(operation *managedGoalOperation) session.ManagedInputOwner {
	return session.ManagedInputOwner{
		QueueEntryID:  operation.entry.ID,
		SessionID:     operation.entry.SessionID,
		OwnerKind:     operation.entry.OwnerKind,
		LoopRunID:     operation.entry.LoopRunID,
		TaskRunID:     operation.entry.TaskRunID,
		RunGeneration: int(*operation.entry.RunGeneration),
		PromptAttempt: operation.entry.PromptAttempt,
		ControlEpoch:  *operation.entry.OwnerEpoch,
		BindingEpoch:  *operation.entry.BindingEpoch,
		PromptID:      operation.entry.PromptID,
		PromptKind:    operation.entry.PromptKind,
	}
}

func managedInputOwnerFromQueueEntry(entry store.SessionInputQueueEntry) (session.ManagedInputOwner, error) {
	if entry.RunGeneration == nil || entry.OwnerEpoch == nil || entry.BindingEpoch == nil {
		return session.ManagedInputOwner{}, errors.New("daemon: managed Goal queue epochs are incomplete")
	}
	return session.ManagedInputOwner{
		QueueEntryID:  entry.ID,
		SessionID:     entry.SessionID,
		OwnerKind:     entry.OwnerKind,
		LoopRunID:     entry.LoopRunID,
		TaskRunID:     entry.TaskRunID,
		RunGeneration: int(*entry.RunGeneration),
		PromptAttempt: entry.PromptAttempt,
		ControlEpoch:  *entry.OwnerEpoch,
		BindingEpoch:  *entry.BindingEpoch,
		PromptID:      entry.PromptID,
		PromptKind:    entry.PromptKind,
	}, nil
}

func validateManagedGoalOwner(owner session.ManagedInputOwner) error {
	if owner.OwnerKind != loopManagedGoalKind || strings.TrimSpace(owner.QueueEntryID) == "" ||
		strings.TrimSpace(owner.SessionID) == "" || strings.TrimSpace(owner.LoopRunID) == "" ||
		strings.TrimSpace(owner.TaskRunID) == "" || owner.RunGeneration < 1 || owner.PromptAttempt < 0 ||
		owner.ControlEpoch < 1 || owner.BindingEpoch < 1 || strings.TrimSpace(owner.PromptID) == "" ||
		strings.TrimSpace(owner.PromptKind) == "" {
		return fmt.Errorf("%w: managed Goal owner is incomplete", looppkg.ErrValidation)
	}
	return nil
}

func managedGoalEntryMatchesOwner(entry store.SessionInputQueueEntry, owner session.ManagedInputOwner) bool {
	loaded, err := managedInputOwnerFromQueueEntry(entry)
	return err == nil && loaded == owner
}

func decodeManagedGoalTaskMetadata(raw json.RawMessage) (managedGoalTaskRunMetadata, error) {
	var metadata managedGoalTaskRunMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return managedGoalTaskRunMetadata{}, fmt.Errorf("daemon: decode managed Goal task metadata: %w", err)
	}
	metadata.NodeID = strings.TrimSpace(metadata.NodeID)
	if metadata.Generation < 1 || metadata.NodeID == "" || metadata.ItemIndex < 0 ||
		metadata.GoalSegmentEpoch < 1 {
		return managedGoalTaskRunMetadata{}, fmt.Errorf(
			"%w: managed Goal task metadata is invalid",
			looppkg.ErrValidation,
		)
	}
	return metadata, nil
}
