package toolruntime

import (
	"context"

	"errors"
	"fmt"

	"strings"
)

func (r *Registry) checkpoint(ctx context.Context, id string, checkpoint ProcessCheckpoint) error {
	if ctx == nil {
		return errors.New("toolruntime: checkpoint context is required")
	}
	r.mu.RLock()
	active, ok := r.active[id]
	if !ok {
		r.mu.RUnlock()
		return nil
	}
	record := active.record
	r.mu.RUnlock()

	if checkpoint.Owner != nil {
		record.Owner = normalizeOwner(*checkpoint.Owner)
	}
	if checkpoint.PID != nil {
		record.PID = *checkpoint.PID
	}
	if checkpoint.ProcessGroupID != nil {
		record.ProcessGroupID = *checkpoint.ProcessGroupID
	}
	if checkpoint.StartedAt != nil {
		record.StartedAt = checkpoint.StartedAt.UTC()
	}
	if checkpoint.State != "" {
		record.State = checkpoint.State
	}
	if checkpoint.Error != "" {
		record.Error = trimBounded(checkpoint.Error)
	}
	if checkpoint.UpdatedAt.IsZero() {
		record.UpdatedAt = r.now().UTC()
	} else {
		record.UpdatedAt = checkpoint.UpdatedAt.UTC()
	}

	if err := validateRecord(record); err != nil {
		return err
	}
	if err := r.upsert(ctx, record); err != nil {
		return err
	}

	r.mu.Lock()
	if current, ok := r.active[id]; ok {
		current.record = record
		r.active[id] = current
	}
	r.mu.Unlock()
	return nil
}

func (r *Registry) complete(ctx context.Context, id string, completion ProcessCompletion) error {
	if ctx == nil {
		return errors.New("toolruntime: complete context is required")
	}
	r.mu.RLock()
	_, ok := r.active[id]
	r.mu.RUnlock()
	if !ok {
		return nil
	}

	state := ProcessStateCompleted
	errText := strings.TrimSpace(completion.Error)
	if completion.Err != nil {
		errText = completion.Err.Error()
		state = ProcessStateFailed
	}
	completedAt := r.now().UTC()
	if err := r.updateState(ctx, id, state, completion.ExitCode, errText, &completedAt); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.active, id)
	r.mu.Unlock()
	return nil
}

func (r *Registry) interruptCandidates(
	ctx context.Context,
	scope InterruptScope,
) ([]activeProcess, error) {
	byID := make(map[string]activeProcess)

	r.mu.RLock()
	for id, active := range r.active {
		if matchesScope(active.record, scope) && !isTerminalState(active.record.State) {
			byID[id] = active
		}
	}
	r.mu.RUnlock()

	if r.store != nil {
		records, err := r.store.ListProcessRecords(ctx, ProcessQuery{
			States: activeStates(),
			Scope:  scope,
		})
		if err != nil {
			return nil, fmt.Errorf("toolruntime: list process records for interrupt: %w", err)
		}
		for _, record := range records {
			if _, exists := byID[record.ID]; exists {
				continue
			}
			byID[record.ID] = activeProcess{record: record}
		}
	}

	candidates := make([]activeProcess, 0, len(byID))
	for _, candidate := range byID {
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func matchesScope(record ProcessRecord, scope InterruptScope) bool {
	if scope.ProcessID != "" && record.ID != scope.ProcessID {
		return false
	}
	if scope.SessionID != "" && record.Owner.SessionID != scope.SessionID {
		return false
	}
	if scope.TurnID != "" && record.Owner.TurnID != scope.TurnID {
		return false
	}
	if scope.ToolCallID != "" && record.Owner.ToolCallID != scope.ToolCallID {
		return false
	}
	if scope.TerminalID != "" && record.Owner.TerminalID != scope.TerminalID {
		return false
	}
	if scope.ExtensionName != "" && record.Owner.ExtensionName != scope.ExtensionName {
		return false
	}
	if scope.HookName != "" && record.Owner.HookName != scope.HookName {
		return false
	}
	if scope.Source != "" && record.Source != scope.Source {
		return false
	}
	return true
}
