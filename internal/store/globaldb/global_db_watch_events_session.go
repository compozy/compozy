package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
)

func (g *WatchEventsRepo) readSessionWatchEventsCursor(
	ctx context.Context,
	query normalizedWatchEventsQuery,
	stream string,
) (int64, error) {
	if !sessionWatchEventRequested(query.kinds) {
		return 0, nil
	}
	reader, ok, err := g.openSessionWatchEventsReader(ctx, query, stream)
	if err != nil || !ok {
		return 0, err
	}
	cursor, err := reader.MaxEventSequence(ctx)
	if err != nil {
		closeErr := reader.Close(ctx)
		if closeErr != nil {
			return 0, errors.Join(fmt.Errorf("store: read session watch-events cursor: %w", err), closeErr)
		}
		return 0, fmt.Errorf("store: read session watch-events cursor: %w", err)
	}
	if err := reader.Close(ctx); err != nil {
		return 0, err
	}
	return cursor, nil
}

func (g *WatchEventsRepo) readSessionWatchEvents(
	ctx context.Context,
	query normalizedWatchEventsQuery,
	stream string,
) ([]looppkg.WatchEvent, error) {
	if !sessionWatchEventRequested(query.kinds) {
		return nil, nil
	}
	reader, ok, err := g.openSessionWatchEventsReader(ctx, query, stream)
	if err != nil || !ok {
		return nil, err
	}
	sessionID, ok := looppkg.WatchEventsSessionIDFromStream(stream)
	if !ok {
		return nil, fmt.Errorf("%w: watch-events session stream is invalid: %q", looppkg.ErrValidation, stream)
	}
	events, err := reader.QueryEventMetadata(ctx, store.EventQuery{
		AfterSequence: query.streams[stream],
		Limit:         query.limit,
	})
	if err != nil {
		closeErr := reader.Close(ctx)
		if closeErr != nil {
			return nil, errors.Join(fmt.Errorf("store: read session watch-events: %w", err), closeErr)
		}
		return nil, fmt.Errorf("store: read session watch-events: %w", err)
	}
	if err := reader.Close(ctx); err != nil {
		return nil, err
	}
	watchEvents := make([]looppkg.WatchEvent, 0, len(events))
	for _, event := range events {
		watchEvents = append(watchEvents, sessionWatchEventFromMetadata(query.workspaceID, sessionID, stream, event))
	}
	return watchEvents, nil
}

func (g *WatchEventsRepo) openSessionWatchEventsReader(
	ctx context.Context,
	query normalizedWatchEventsQuery,
	stream string,
) (store.EventMetadataReadCloser, bool, error) {
	sessionID, ok := looppkg.WatchEventsSessionIDFromStream(stream)
	if !ok {
		return nil, false, fmt.Errorf("%w: watch-events session stream is invalid: %q", looppkg.ErrValidation, stream)
	}
	sessions, err := g.sessions.ListSessions(ctx, store.SessionListQuery{
		ID:          sessionID,
		WorkspaceID: query.workspaceID,
		Limit:       1,
	})
	if err != nil {
		return nil, false, err
	}
	if len(sessions) == 0 {
		return nil, false, nil
	}
	if g.openSessionEventMetadata == nil {
		return nil, false, errors.New("store: session watch-events metadata opener is required")
	}
	reader, err := g.openSessionEventMetadata(ctx, sessionID)
	if err != nil {
		return nil, false, fmt.Errorf("store: open session watch-events db %q: %w", sessionID, err)
	}
	return reader, true, nil
}

func sessionWatchEventRequested(kinds []string) bool {
	for _, kind := range kinds {
		if strings.TrimSpace(kind) == string(hookspkg.HookEventPostRecord) {
			return true
		}
	}
	return false
}

func sessionWatchEventFromMetadata(
	workspaceID string,
	sessionID string,
	stream string,
	row store.EventMetadata,
) looppkg.WatchEvent {
	payload := map[string]any{
		watchEventsPayloadRecordTypeKey: strings.TrimSpace(row.Type),
		watchEventsPayloadSequenceKey:   row.Sequence,
		watchEventsPayloadTurnIDKey:     strings.TrimSpace(row.TurnID),
		watchEventsPayloadAgentNameKey:  strings.TrimSpace(row.AgentName),
		watchEventsPayloadSessionIDKey:  strings.TrimSpace(sessionID),
	}
	return looppkg.WatchEvent{
		Kind:        string(hookspkg.HookEventPostRecord),
		Seq:         row.Sequence,
		Stream:      strings.TrimSpace(stream),
		At:          formatWatchEventAt(row.Timestamp),
		WorkspaceID: strings.TrimSpace(workspaceID),
		SessionID:   strings.TrimSpace(sessionID),
		Payload:     payload,
		LedgerKind:  string(hookspkg.HookEventPostRecord),
	}
}
