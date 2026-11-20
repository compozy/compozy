package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// deriveSnapshot is one immutable read view of a derive source.
type deriveSnapshot struct {
	timing      *deriveTiming
	includeOpen bool
	meta        store.SessionMeta
	epoch       int64
	generation  int64
	maxSequence int64
	messages    []transcript.Message
	bodyStart   int
	budget      replayBudget
	imported    *store.SessionImportedContext
	// wholeMessageCount counts the source's own messages through its last settled turn
	// (or the cut, when later), independent of the fork cut: the preview's total.
	wholeMessageCount    int
	visibleMessageCount  int
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
	if _, err := m.resolveStoredSessionOwner(ctx, &meta); err != nil {
		return store.SessionMeta{}, fmt.Errorf("session: prove catalog owner for %q: %w", target, err)
	}
	return meta, nil
}

// readDeriveSnapshot reads at a fixed high-water mark and rechecks its generation under the commit lock.
func (m *Manager) readDeriveSnapshot(
	ctx context.Context,
	spec deriveSpec,
	timing *deriveTiming,
) (_ deriveSnapshot, _ func(), retErr error) {
	release, err := m.acquireDeriveSnapshotLock(ctx, spec.sourceID, timing)
	if err != nil {
		return deriveSnapshot{}, nil, err
	}
	started := time.Now()
	initialLockWait := timing.lock
	defer func() {
		if retErr != nil {
			release()
		}
		timing.read = time.Since(started) - (timing.lock - initialLockWait) - timing.bound
	}()
	snapshot, err := m.prepareDeriveSnapshot(ctx, spec, timing)
	if err != nil {
		return deriveSnapshot{}, nil, err
	}
	if snapshot.includeOpen {
		err = m.readDeriveSnapshotEvents(ctx, &snapshot, spec.messageID)
		return snapshot, release, err
	}
	if err := m.readDeriveHead(ctx, &snapshot); err != nil {
		return deriveSnapshot{}, nil, err
	}
	release()
	readErr := m.readDeriveSnapshotEvents(ctx, &snapshot, spec.messageID)
	if readErr == nil {
		snapshot.imported, readErr = m.buildImportedContext(ctx, &snapshot, spec, snapshot.budget)
	}
	commitRelease, err := m.acquireDeriveSnapshotLock(ctx, spec.sourceID, timing)
	if err != nil {
		return deriveSnapshot{}, nil, err
	}
	release = commitRelease
	if err := m.recheckDeriveSnapshot(ctx, &snapshot); err != nil {
		return deriveSnapshot{}, nil, err
	}
	if readErr != nil {
		return deriveSnapshot{}, nil, readErr
	}
	return snapshot, release, nil
}

func (m *Manager) acquireDeriveSnapshotLock(ctx context.Context, id string, timing *deriveTiming) (func(), error) {
	started := time.Now()
	release, err := m.lockDeriveConversation(ctx, id)
	timing.lock += time.Since(started)
	if err != nil {
		return nil, err
	}
	return sync.OnceFunc(release), nil
}

func (m *Manager) prepareDeriveSnapshot(
	ctx context.Context,
	spec deriveSpec,
	timing *deriveTiming,
) (deriveSnapshot, error) {
	meta, err := m.readSessionMetaReadOnly(ctx, spec.sourceID)
	if err != nil {
		return deriveSnapshot{}, err
	}
	if meta.WorkspaceID != spec.workspaceID || (spec.sourceProfileID != "" && meta.ProfileID != spec.sourceProfileID) {
		return deriveSnapshot{}, ErrSessionNotFound
	}
	snapshot := deriveSnapshot{meta: meta, includeOpen: spec.kind == store.LineageKindSide, timing: timing}
	if snapshot.includeOpen {
		m.snapshotSideRuntime(&snapshot)
	} else {
		workspace, err := m.resolveResumeWorkspace(ctx, deriveResolutionMeta(spec, &meta))
		if err != nil {
			return deriveSnapshot{}, err
		}
		snapshot.budget = m.deriveBudget(&workspace)
	}
	snapshot.epoch, snapshot.sourceActive, err = m.deriveTranscriptEpoch(ctx, meta.ID)
	return snapshot, err
}

func (m *Manager) deriveTranscriptEpoch(ctx context.Context, id string) (int64, bool, error) {
	if live, ok := m.Get(id); ok {
		return live.Info().TranscriptEpoch, true, nil
	}
	if m.transcriptEpochStore == nil {
		return 0, false, nil
	}
	epoch, err := m.transcriptEpochStore.SessionTranscriptEpoch(ctx, id)
	if err != nil {
		return 0, false, fmt.Errorf("session: read transcript epoch of %q: %w", id, err)
	}
	return epoch, false, nil
}

func (m *Manager) readDeriveHead(ctx context.Context, snapshot *deriveSnapshot) (retErr error) {
	recorder, cleanup, err := m.openDeriveQueryRecorder(ctx, snapshot)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, cleanup()) }()
	if reader, ok := recorder.(transcript.Reader); ok {
		head, err := reader.TranscriptPage(ctx, transcript.PageQuery{MetadataOnly: true})
		if err != nil {
			return fmt.Errorf("session: read derive source transcript fences: %w", err)
		}
		snapshot.generation, snapshot.maxSequence = head.Generation, head.MaxSequence
		return nil
	}
	rows, err := recorder.QueryEventMetadata(ctx, store.EventQuery{Limit: 1, Archive: store.EventArchiveUnarchived})
	if err == nil && len(rows) > 0 {
		snapshot.maxSequence = rows[0].Sequence
	}
	return err
}

func (m *Manager) recheckDeriveSnapshot(ctx context.Context, snapshot *deriveSnapshot) error {
	current := deriveSnapshot{meta: snapshot.meta}
	var err error
	current.epoch, current.sourceActive, err = m.deriveTranscriptEpoch(ctx, snapshot.meta.ID)
	if err != nil {
		return err
	}
	if current.epoch != snapshot.epoch {
		return ErrDeriveFenceConflict
	}
	if err := m.readDeriveHead(ctx, &current); err != nil {
		return err
	}
	if current.generation != snapshot.generation {
		return ErrDeriveFenceConflict
	}
	return nil
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
	query := store.EventQuery{
		Archive: store.EventArchiveUnarchived, ReplayShape: !snapshot.includeOpen,
	}
	if !snapshot.includeOpen {
		query.BeforeSequence = snapshot.maxSequence + 1
	}
	events, err := recorder.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("session: query derive source events of %q: %w", snapshot.meta.ID, err)
	}
	if snapshot.includeOpen {
		if err := m.readDeriveHead(ctx, snapshot); err != nil {
			return err
		}
		if len(events) > 0 {
			snapshot.maxSequence = events[len(events)-1].Sequence
		}
	}
	baseline, coveredThrough, err := deriveRewindBaseline(ctx, recorder)
	if err != nil {
		return err
	}
	if messageID == "" {
		turnID, through, laterOpen := lastSettledTurn(events)
		snapshot.cut = DeriveCut{TurnID: turnID, ThroughSequence: through, TurnSettled: true}
		// The event cut owns turn state: a prompt can start after reading the live metadata.
		// Retained open turns on stopped side sources remain replayable without a working marker.
		snapshot.sourceTurnInProgress = laterOpen && (!snapshot.includeOpen || snapshot.sourceActive)
	} else {
		cut, cutErr := deriveAnchorCut(ctx, recorder, snapshot.meta.ID, messageID, events)
		if cutErr != nil && !errors.Is(cutErr, ErrDeriveTurnInProgress) {
			return cutErr
		}
		snapshot.cut, snapshot.cutErr = cut, cutErr
	}
	if err := snapshot.assembleReplay(ctx, recorder, events, baseline, coveredThrough); err != nil {
		return fmt.Errorf("session: assemble derive source transcript of %q: %w", snapshot.meta.ID, err)
	}
	snapshot.wholeMessageCount = len(snapshot.messages)
	if messageID != "" {
		_, settledThrough, _ := lastSettledTurn(events)
		if settledThrough > snapshot.cut.ThroughSequence {
			whole, wholeErr := assembleDeriveMessages(baseline, events, coveredThrough, settledThrough)
			if wholeErr != nil {
				return fmt.Errorf("session: assemble derive source transcript of %q: %w", snapshot.meta.ID, wholeErr)
			}
			snapshot.wholeMessageCount = len(whole)
		}
	}
	return snapshot.countVisibleMessages(ctx, recorder)
}

func (snapshot *deriveSnapshot) assembleReplay(
	ctx context.Context, recorder EventReadCloser, events []store.SessionEvent,
	baseline []transcript.Message, coveredThrough int64,
) (err error) {
	if snapshot.includeOpen {
		var messages []transcript.Message
		messages, err = transcript.Assemble(eventsAfter(events, coveredThrough))
		snapshot.messages = transcript.Prune(append(baseline, messages...), transcript.PruneOptions{Dedup: true})
		return err
	}
	snapshot.messages, snapshot.bodyStart, err = readDeriveReplayWindow(ctx, recorder, baseline, events, coveredThrough,
		snapshot.cut.ThroughSequence, snapshot.budget.MaxBytes)
	return err
}

func (snapshot *deriveSnapshot) countVisibleMessages(ctx context.Context, recorder EventReadCloser) error {
	counter, ok := recorder.(transcript.RangeCounter)
	if !ok || snapshot.includeOpen {
		return nil
	}
	count, err := counter.TranscriptCountThrough(ctx, snapshot.cut.ThroughSequence)
	if err != nil {
		return fmt.Errorf("session: count visible derive source messages: %w", err)
	}
	snapshot.visibleMessageCount = count
	return nil
}

// deriveSourceMoved reports whether the source recorded any unarchived event after the
// snapshot's cut. The derive holds the source's conversation operation lock, so its epoch
// and generation cannot change; only appended events can move it.
func (m *Manager) deriveSourceMoved(ctx context.Context, snapshot *deriveSnapshot) (_ bool, retErr error) {
	recorder, cleanup, err := m.openDeriveQueryRecorder(ctx, snapshot)
	if err != nil {
		return false, err
	}
	defer func() { retErr = errors.Join(retErr, cleanup()) }()
	later, err := recorder.QueryEventMetadata(ctx, store.EventQuery{
		Forward: true, AfterSequence: snapshot.maxSequence, Limit: 1, Archive: store.EventArchiveUnarchived,
	})
	if err != nil {
		return false, fmt.Errorf("session: query derive source events of %q: %w", snapshot.meta.ID, err)
	}
	return len(later) > 0, nil
}

// assembleDeriveMessages assembles the rewind baseline plus the carried events in
// (afterSequence, throughSequence], deduplicated the way a derive carries them. A cut
// inside the baseline (a retained pre-rewind turn) also cuts the baseline there.
func assembleDeriveMessages(
	baseline []transcript.Message,
	events []store.SessionEvent,
	afterSequence int64,
	throughSequence int64,
) ([]transcript.Message, error) {
	if throughSequence < afterSequence {
		cut, err := cutDeriveBaseline(baseline, events, throughSequence, afterSequence)
		if err != nil {
			return nil, err
		}
		return transcript.Prune(cut, transcript.PruneOptions{Dedup: true}), nil
	}
	own, err := transcript.Assemble(carriedEvents(events, afterSequence, throughSequence))
	if err != nil {
		return nil, err
	}
	merged := append(append([]transcript.Message(nil), baseline...), own...)
	return transcript.Prune(merged, transcript.PruneOptions{Dedup: true}), nil
}

// cutDeriveBaseline keeps the baseline messages before the first one the retained events
// in (throughSequence, coveredThrough] assemble: those are the turns after the cut. The
// baseline was assembled from the same events, so their message ids match.
func cutDeriveBaseline(
	baseline []transcript.Message,
	events []store.SessionEvent,
	throughSequence int64,
	coveredThrough int64,
) ([]transcript.Message, error) {
	later := make([]store.SessionEvent, 0)
	for _, event := range events {
		if event.Sequence > throughSequence && event.Sequence <= coveredThrough {
			later = append(later, event)
		}
	}
	excluded, err := transcript.Assemble(later)
	if err != nil {
		return nil, err
	}
	afterCut := make(map[string]struct{}, len(excluded))
	for _, message := range excluded {
		afterCut[message.ID] = struct{}{}
	}
	for index, message := range baseline {
		if _, ok := afterCut[message.ID]; ok {
			return append([]transcript.Message(nil), baseline[:index]...), nil
		}
	}
	return append([]transcript.Message(nil), baseline...), nil
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
	if state.BaselineStale {
		messages, err := assembleConversationRewindPrefix(ctx, recorder, state.CoveredThroughSequence)
		return messages, state.CoveredThroughSequence, err
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
	owner, err := m.resolveStoredSessionOwner(ctx, &snapshot.meta)
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
