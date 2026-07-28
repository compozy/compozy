package windowmanager

import (
	"context"
	"errors"
	"fmt"
)

func (m *Manager) executeDurable(
	ctx context.Context,
	snapshot Snapshot,
	request CommandRequest,
	commandID CommandID,
	rebasedFrom *Revision,
) (Result, error) {
	working, reduced, workspaceDefaults, err := m.reduceDurable(ctx, snapshot, request, commandID)
	if err != nil {
		return Result{}, err
	}
	if !reduced.changed {
		return Result{
			Snapshot:    cloneSnapshot(snapshot),
			Changes:     reduced.changes,
			RebasedFrom: rebasedFrom,
		}, nil
	}
	nextRevision, err := NextTopologyRevision(snapshot.Revision)
	if err != nil {
		return Result{}, err
	}
	working = NormalizeSnapshot(working)
	if err := ValidateSnapshot(working); err != nil {
		return Result{}, err
	}
	working.Revision = nextRevision
	working.UpdatedAt = m.now().UTC()
	if err := m.recordCommandHistory(&working, snapshot, request, commandID, workspaceDefaults); err != nil {
		return Result{}, err
	}
	projection, err := m.projectClientViews(request.WorkspaceID, snapshot, working, request)
	if err != nil {
		return Result{}, err
	}
	event := newCommandEvent(working, request, commandID, reduced.changes)
	if err := m.commitCommand(ctx, snapshot.Revision, request, working, event); err != nil {
		return Result{}, err
	}
	m.publishEvent(event)
	client := m.applyClientProjection(request.WorkspaceID, projection)
	for _, view := range projection.changed {
		m.publishClient(view)
	}
	return Result{
		Snapshot: cloneSnapshot(working), Applied: true, Changes: reduced.changes,
		Client: client, RebasedFrom: rebasedFrom,
	}, nil
}

func (m *Manager) observeCommittedEvent(ctx context.Context, event Event) {
	if m.eventObserver == nil || !isObservableCommand(event.CommandID) {
		return
	}
	m.eventObserver(ctx, cloneEvent(event))
}

func isObservableCommand(commandID CommandID) bool {
	switch commandID {
	case CommandDesktopCreate,
		CommandDesktopDelete,
		CommandWindowMove,
		CommandLayoutArrange,
		CommandLayoutReplace:
		return true
	default:
		return false
	}
}

func (m *Manager) reduceDurable(
	ctx context.Context,
	snapshot Snapshot,
	request CommandRequest,
	commandID CommandID,
) (Snapshot, reduction, Config, error) {
	client, err := m.clientForRequest(request, commandID)
	if err != nil {
		return Snapshot{}, reduction{}, Config{}, err
	}
	workspaceDefaults, err := m.resolveWorkspaceDefaults(ctx, request.WorkspaceID)
	if err != nil {
		return Snapshot{}, reduction{}, Config{}, err
	}
	payload, err := m.resolveLayoutResourceCommand(
		ctx,
		request.WorkspaceID,
		request.Payload,
		workspaceDefaults,
	)
	if err != nil {
		return Snapshot{}, reduction{}, Config{}, err
	}
	config, err := effectiveConfig(workspaceDefaults, snapshot.Overrides)
	if err != nil {
		return Snapshot{}, reduction{}, Config{}, fmt.Errorf("resolve effective config: %w", err)
	}
	working := cloneSnapshot(snapshot)
	var focused *WindowID
	if client != nil {
		focused = clonePointer(client.FocusedWindowID)
	}
	reduced, err := (&reducer{
		generate: m.generate, config: config, focusedWindow: focused,
	}).reduce(&working, payload)
	if err != nil {
		return Snapshot{}, reduction{}, Config{}, err
	}
	return working, reduced, workspaceDefaults, nil
}

func (m *Manager) recordCommandHistory(
	working *Snapshot,
	before Snapshot,
	request CommandRequest,
	commandID CommandID,
	workspaceDefaults Config,
) error {
	if commandID == CommandLayoutUndo || commandID == CommandLayoutRedo || commandID == CommandWindowNavigate {
		return nil
	}
	config, err := effectiveConfig(workspaceDefaults, working.Overrides)
	if err != nil {
		return fmt.Errorf("resolve committed config: %w", err)
	}
	entry := HistoryEntry{
		CommandID: commandID,
		Before:    snapshotState(before), After: snapshotState(*working),
		Actor: request.Actor, Origin: request.Origin, CreatedAt: working.UpdatedAt,
	}
	appendHistory(working, entry, config.HistoryLimit)
	return nil
}

func newCommandEvent(
	snapshot Snapshot,
	request CommandRequest,
	commandID CommandID,
	changes ChangeSet,
) Event {
	return Event{
		WorkspaceID: request.WorkspaceID, Revision: snapshot.Revision,
		CommandID: commandID, Changes: changes, Actor: request.Actor,
		Origin: request.Origin, OccurredAt: snapshot.UpdatedAt,
	}
}

func (m *Manager) commitCommand(
	ctx context.Context,
	expected Revision,
	request CommandRequest,
	snapshot Snapshot,
	event Event,
) error {
	commit := Commit{
		WorkspaceID: request.WorkspaceID, ExpectedRevision: expected,
		Snapshot: snapshot, Event: event,
	}
	if err := m.repository.Commit(ctx, &commit); err != nil {
		if errors.Is(err, ErrRevisionConflict) {
			return m.repositoryConflict(ctx, request.WorkspaceID, request.ExpectedRevision)
		}
		return fmt.Errorf("commit window command %q: %w", event.CommandID, err)
	}
	return nil
}
