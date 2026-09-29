package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// deriveSnapshot is one immutable read view of a derive source.
type deriveSnapshot struct {
	meta                 store.SessionMeta
	epoch                int64
	generation           int64
	maxSequence          int64
	messages             []transcript.Message
	cut                  DeriveCut
	sourceTurnInProgress bool
	sourceActive         bool
	// cutErr is ErrDeriveTurnInProgress for an unsettled anchor turn: a preview reports
	// the unsettled cut, a derive refuses it.
	cutErr error
}

// readSessionMetaReadOnly proves the catalog owner and decodes the metadata document
// without restoring stop receipts or repairing inactive metadata: a derive or preview
// never reclassifies, repairs, or reprojects the session it reads.
func (m *Manager) readSessionMetaReadOnly(ctx context.Context, id string) (store.SessionMeta, error) {
	target, err := normalizeStoredSessionID(id)
	if err != nil {
		return store.SessionMeta{}, err
	}
	if live, ok := m.Get(target); ok {
		return live.Meta(), nil
	}
	reader := m.readSessionMeta
	if reader == nil {
		reader = store.ReadSessionMeta
	}
	meta, err := reader(store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.SessionMeta{}, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
		}
		return store.SessionMeta{}, fmt.Errorf("session: read metadata for %q: %w", target, err)
	}
	if metaID, idErr := normalizeStoredSessionID(meta.ID); idErr != nil || metaID != target {
		return store.SessionMeta{}, fmt.Errorf(
			"%w: metadata identity %q does not match directory %q", ErrSessionNotFound, meta.ID, target,
		)
	}
	if m.isPending(target) {
		return meta, nil
	}
	if _, err := m.resolveStoredSessionOwner(ctx, target, meta.WorkspaceID); err != nil {
		return store.SessionMeta{}, fmt.Errorf("session: prove catalog owner for %q: %w", target, err)
	}
	return meta, nil
}

// readDeriveSnapshot reads one immutable view of the source under its conversation
// operation lock (rewind, clear, and delete wait on it). The returned release must be
// called: previews release immediately, derives keep the lock until the child commits.
// The events are read in one query, and the fences are computed from that same read, so
// concurrent appends cannot tear the view.
func (m *Manager) readDeriveSnapshot(
	ctx context.Context,
	sourceID string,
	messageID string,
) (_ deriveSnapshot, _ func(), retErr error) {
	_, unlockConversation, err := m.lockConversationOperation(ctx, sourceID)
	if err != nil {
		return deriveSnapshot{}, nil, err
	}
	var once sync.Once
	unlock := func() { once.Do(unlockConversation) }
	defer func() {
		if retErr != nil {
			unlock()
		}
	}()
	meta, err := m.readSessionMetaReadOnly(ctx, sourceID)
	if err != nil {
		return deriveSnapshot{}, nil, err
	}
	snapshot := deriveSnapshot{meta: meta}
	if live, ok := m.Get(meta.ID); ok {
		snapshot.sourceActive = true
		snapshot.epoch = live.Info().TranscriptEpoch
	} else if m.transcriptEpochStore != nil {
		if snapshot.epoch, err = m.transcriptEpochStore.SessionTranscriptEpoch(ctx, meta.ID); err != nil {
			return deriveSnapshot{}, nil, fmt.Errorf("session: read transcript epoch of %q: %w", meta.ID, err)
		}
	}
	if err := m.readDeriveSnapshotEvents(ctx, &snapshot, strings.TrimSpace(messageID)); err != nil {
		return deriveSnapshot{}, nil, err
	}
	return snapshot, unlock, nil
}

func (m *Manager) readDeriveSnapshotEvents(
	ctx context.Context,
	snapshot *deriveSnapshot,
	messageID string,
) (retErr error) {
	recorder, cleanup, err := m.openDeriveQueryRecorder(ctx, snapshot)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, cleanup()) }()
	events, err := recorder.Query(ctx, store.EventQuery{Archive: store.EventArchiveUnarchived})
	if err != nil {
		return fmt.Errorf("session: query derive source events of %q: %w", snapshot.meta.ID, err)
	}
	for _, event := range events {
		snapshot.maxSequence = max(snapshot.maxSequence, event.Sequence)
	}
	if reader, ok := recorder.(transcript.Reader); ok {
		page, pageErr := reader.TranscriptPage(ctx, transcript.PageQuery{Limit: 1})
		if pageErr != nil {
			return fmt.Errorf("session: read derive source generation of %q: %w", snapshot.meta.ID, pageErr)
		}
		snapshot.generation = page.Generation
	}
	baseline, coveredThrough, err := deriveRewindBaseline(ctx, recorder)
	if err != nil {
		return err
	}
	if messageID == "" {
		turnID, through, laterOpen := lastSettledTurn(events)
		snapshot.cut = DeriveCut{TurnID: turnID, ThroughSequence: through, TurnSettled: true}
		snapshot.sourceTurnInProgress = laterOpen
	} else {
		cut, cutErr := deriveAnchorCut(ctx, recorder, snapshot.meta.ID, messageID, events)
		if cutErr != nil && !errors.Is(cutErr, ErrDeriveTurnInProgress) {
			return cutErr
		}
		snapshot.cut, snapshot.cutErr = cut, cutErr
	}
	own, err := transcript.Assemble(carriedEvents(events, coveredThrough, snapshot.cut.ThroughSequence))
	if err != nil {
		return fmt.Errorf("session: assemble derive source transcript of %q: %w", snapshot.meta.ID, err)
	}
	snapshot.messages = transcript.Prune(append(baseline, own...), transcript.PruneOptions{Dedup: true})
	return nil
}

// deriveRewindBaseline returns the rewind baseline messages and the sequence they cover.
func deriveRewindBaseline(ctx context.Context, recorder EventReadCloser) ([]transcript.Message, int64, error) {
	reader, ok := recorder.(store.ConversationRewindReader)
	if !ok {
		return nil, 0, nil
	}
	state, found, err := reader.ConversationRewindState(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("session: read derive source rewind state: %w", err)
	}
	if !found {
		return nil, 0, nil
	}
	var messages []transcript.Message
	if err := json.Unmarshal([]byte(state.MessagesJSON), &messages); err != nil {
		return nil, 0, fmt.Errorf("session: decode derive source rewind baseline: %w", err)
	}
	return messages, state.CoveredThroughSequence, nil
}

// deriveAnchorCut resolves a fork anchor through the durable transcript entry of the
// user message (TranscriptUserAnchor: complete user entry, active start event, no
// archived-prefix guard), then cuts through that turn's terminal event.
func deriveAnchorCut(
	ctx context.Context,
	recorder EventReadCloser,
	sourceID string,
	messageID string,
	events []store.SessionEvent,
) (DeriveCut, error) {
	reader, ok := recorder.(store.TranscriptAnchorReader)
	if !ok {
		return DeriveCut{}, errors.New("session: event recorder does not resolve transcript anchors")
	}
	anchor, err := reader.TranscriptUserAnchor(ctx, messageID)
	if errors.Is(err, store.ErrTranscriptAnchorNotFound) || errors.Is(err, store.ErrTranscriptAnchorInvalid) {
		return DeriveCut{}, deriveErr(
			ErrDeriveMessageNotFound, "message %s not found in session %s", messageID, sourceID,
		)
	}
	if err != nil {
		return DeriveCut{}, fmt.Errorf("session: resolve derive anchor %q: %w", messageID, err)
	}
	return resolveDeriveCut(anchor, events)
}

// openDeriveQueryRecorder opens the source's events without the repairing metadata read
// the general query path performs for stored sessions.
func (m *Manager) openDeriveQueryRecorder(
	ctx context.Context,
	snapshot *deriveSnapshot,
) (EventReadCloser, func() error, error) {
	if snapshot.sourceActive {
		return m.openQueryRecorder(ctx, snapshot.meta.ID)
	}
	if m.openQueryStore == nil {
		return nil, nil, errors.New("session: query recorder opener is required")
	}
	owner, err := m.resolveStoredSessionOwner(ctx, snapshot.meta.ID, snapshot.meta.WorkspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("session: resolve catalog owner for %q: %w", snapshot.meta.ID, err)
	}
	dbPath := store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, snapshot.meta.ID))
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("%w: %s", ErrSessionNotFound, snapshot.meta.ID)
		}
		return nil, nil, fmt.Errorf("session: stat events database for %q: %w", snapshot.meta.ID, err)
	}
	reader, err := m.openQueryStore(ctx, owner, dbPath)
	if err != nil {
		return nil, nil, normalizeRecorderOpenError(snapshot.meta.ID, err)
	}
	return reader, m.eventStoreCleanup(reader), nil
}
