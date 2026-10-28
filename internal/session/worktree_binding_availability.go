package session

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

// WorktreeBindingAvailability is advisory; mutation repeats admission atomically.
type WorktreeBindingAvailability struct {
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

func (m *Manager) WorktreeBindingAvailability(ctx context.Context, id string) WorktreeBindingAvailability {
	err := m.worktreeBindingAvailabilityError(ctx, id)
	if err != nil {
		return WorktreeBindingAvailability{UnavailableReason: err.Error()}
	}
	return WorktreeBindingAvailability{Available: true}
}

func (m *Manager) worktreeBindingAvailabilityError(ctx context.Context, id string) error {
	if _, held := m.restartInputsHeld.Load(id); held || m.sessionStartRun(id) != nil {
		return ErrPromptInProgress
	}
	var meta store.SessionMeta
	if target, ok := m.Get(id); ok {
		target.mu.RLock()
		err := target.worktreeBindingRefusalLocked()
		meta = target.metaLocked()
		target.mu.RUnlock()
		if err != nil {
			return err
		}
	} else {
		var err error
		meta, err = m.readMetaWithContext(ctx, id)
		if err != nil {
			return err
		}
		if err := validateStoppedWorktreeBinding(&meta); err != nil {
			return err
		}
	}
	if meta.WorktreeTransition != nil && meta.WorktreeTransition.Pending {
		return ErrRecoveryPersistence
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, id); err != nil {
		return err
	}
	if m.inputQueue != nil {
		inputs, err := m.inputQueue.List(ctx, id)
		if err != nil {
			return err
		}
		if len(inputs) != 0 {
			return ErrPromptInProgress
		}
	}
	if m.workSignals.hasExternalSources() {
		state := m.workSignals.Inspect(ctx, id, m.now())
		if worktreeBindingOwned(state) || worktreeBindingSourceUnknown(state) {
			return ErrPromptInProgress
		}
	}
	return nil
}
