package toolruntime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"strings"

	"time"
)

func (r *Registry) validateRecovered(record ProcessRecord) bool {
	if record.PID <= 0 || record.StartedAt.IsZero() {
		return false
	}
	return r.verifier(record.PID, record.StartedAt)
}

func (r *Registry) markStale(ctx context.Context, id string, message string) error {
	completedAt := r.now().UTC()
	return r.updateState(ctx, id, ProcessStateStale, nil, message, &completedAt)
}

func (r *Registry) updateState(
	ctx context.Context,
	id string,
	state ProcessState,
	exitCode *int,
	errText string,
	completedAt *time.Time,
) error {
	if err := state.Validate(); err != nil {
		return err
	}
	update := ProcessStateUpdate{
		ID:          strings.TrimSpace(id),
		State:       state,
		ExitCode:    exitCode,
		Error:       trimBounded(errText),
		UpdatedAt:   r.now().UTC(),
		CompletedAt: completedAt,
	}
	if update.ID == "" {
		return errors.New("toolruntime: update process id is required")
	}

	if r.store != nil {
		if err := r.store.UpdateProcessRecordState(ctx, update); err != nil {
			return fmt.Errorf("toolruntime: update process %q state: %w", update.ID, err)
		}
	}

	r.mu.Lock()
	if active, ok := r.active[update.ID]; ok {
		active.record.State = update.State
		active.record.ExitCode = update.ExitCode
		active.record.Error = update.Error
		active.record.UpdatedAt = update.UpdatedAt
		active.record.CompletedAt = update.CompletedAt
		r.active[update.ID] = active
	}
	r.mu.Unlock()
	return nil
}

func (r *Registry) upsert(ctx context.Context, record ProcessRecord) error {
	if r.store == nil {
		return nil
	}
	if err := r.store.UpsertProcessRecord(ctx, record); err != nil {
		return fmt.Errorf("toolruntime: checkpoint process %q: %w", record.ID, err)
	}
	return nil
}

func newProcessID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("toolruntime: generate process id: %w", err)
	}
	return "proc_" + hex.EncodeToString(raw[:]), nil
}
