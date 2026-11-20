package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// ErrInheritedTranscriptUnavailable conceals the reason the source range cannot be read.
var ErrInheritedTranscriptUnavailable = errors.New("the source conversation is no longer available")

// InheritedTranscript returns a bounded page from the authorized child's direct source.
func (m *Manager) InheritedTranscript(
	ctx context.Context, child *Info, query transcript.PageQuery,
) (transcript.Page, int64, error) {
	meta, err := m.readSessionMetaReadOnly(ctx, child.ID)
	if err != nil {
		return transcript.Page{}, 0, err
	}
	if !sameTranscriptOwner(&meta, child.WorkspaceID, child.ProfileID) {
		return transcript.Page{}, 0, ErrSessionNotFound
	}
	imported := meta.ImportedContext
	if imported == nil || meta.Derivation == nil || imported.SourceSessionID == child.ID ||
		meta.Derivation.SourceSessionID != imported.SourceSessionID ||
		(imported.Kind != store.LineageKindFork && imported.Kind != store.LineageKindContinue) {
		return transcript.Page{}, 0, ErrInheritedTranscriptUnavailable
	}
	page, err := m.readInheritedTranscript(ctx, &meta, query)
	if errors.Is(err, ErrSessionNotFound) || errors.Is(err, store.ErrSessionNotFound) ||
		errors.Is(err, ErrSessionArchived) {
		return transcript.Page{}, 0, ErrInheritedTranscriptUnavailable
	}
	return page, imported.SourceEpoch, err
}

func sameTranscriptOwner(meta *store.SessionMeta, workspaceID, profileID string) bool {
	return meta.WorkspaceID == workspaceID &&
		normalizeCreateProfileID(meta.ProfileID) == normalizeCreateProfileID(profileID)
}

func (m *Manager) readInheritedTranscript(
	ctx context.Context, child *store.SessionMeta, query transcript.PageQuery,
) (_ transcript.Page, retErr error) {
	imported := child.ImportedContext
	sourceID := imported.SourceSessionID
	ctx, release, err := m.lockConversationOperation(ctx, sourceID)
	if err != nil {
		return transcript.Page{}, err
	}
	defer release()
	source, err := m.readSessionMetaReadOnly(ctx, sourceID)
	if err != nil {
		return transcript.Page{}, err
	}
	if !sameTranscriptOwner(&source, child.WorkspaceID, child.ProfileID) {
		return transcript.Page{}, ErrInheritedTranscriptUnavailable
	}
	if err := m.requireSessionUnarchived(ctx, source.WorkspaceID, sourceID); err != nil {
		return transcript.Page{}, err
	}
	epoch, active, err := m.deriveTranscriptEpoch(ctx, sourceID)
	if err != nil {
		return transcript.Page{}, err
	}
	if epoch != imported.SourceEpoch {
		return transcript.Page{}, ErrInheritedTranscriptUnavailable
	}
	recorder, cleanup, err := m.openDeriveQueryRecorder(ctx, &deriveSnapshot{meta: source, sourceActive: active})
	if err != nil {
		return transcript.Page{}, err
	}
	defer func() { retErr = errors.Join(retErr, cleanup()) }()
	reader, ok := recorder.(transcript.Reader)
	if !ok {
		return transcript.Page{}, errTranscriptProjectionUnavailable
	}
	through, err := inheritedTranscriptCut(ctx, child, recorder)
	if err != nil {
		return transcript.Page{}, err
	}
	query.ThroughSequence = &through
	page, err := reader.TranscriptPage(ctx, query)
	if err != nil {
		return transcript.Page{}, err
	}
	if page.Generation != imported.SourceGeneration || page.MaxSequence != through {
		return transcript.Page{}, ErrInheritedTranscriptUnavailable
	}
	return page, m.requireSessionUnarchived(ctx, source.WorkspaceID, sourceID)
}

func inheritedTranscriptCut(ctx context.Context, child *store.SessionMeta, recorder EventReadCloser) (int64, error) {
	if history := child.Derivation.History; history != nil {
		return history.ThroughSequence, nil
	}
	imported := child.ImportedContext
	if imported.ThroughTurnID == "" {
		return 0, nil
	}
	rows, err := recorder.QueryEventMetadata(ctx, store.EventQuery{
		TurnID: imported.ThroughTurnID, BeforeSequence: imported.SourceMaxSequence + 1,
		Archive: store.EventArchiveUnarchived, Limit: 1,
	})
	if err != nil {
		return 0, fmt.Errorf("session: recover inherited transcript cut: %w", err)
	}
	if len(rows) == 0 {
		return 0, ErrInheritedTranscriptUnavailable
	}
	return rows[0].Sequence, nil
}
