package sessiondb

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

// restoreCompactionTranscriptProjection retains the ledger's assigned keys instead of rerouting surviving entries.
func restoreCompactionTranscriptProjection(ctx context.Context, db *sql.DB, sessionID string) error {
	keys, err := sqlcgen.New(db).ListMissingUnarchivedCompactionEntries(ctx)
	if err != nil {
		return fmt.Errorf("store: find restored compaction entries: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}
	return store.ExecuteWrite(ctx, db, func(ctx context.Context, tx *store.WriteTx) error {
		return restoreCompactionTranscriptProjectionTx(ctx, tx, sessionID)
	})
}

func restoreCompactionTranscriptProjectionTx(ctx context.Context, tx *store.WriteTx, sessionID string) error {
	queries := sqlcgen.New(tx)
	keys, err := queries.ListMissingUnarchivedCompactionEntries(ctx)
	if err != nil {
		return fmt.Errorf("store: find restored compaction entries: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}
	state, err := queries.GetTranscriptProjectionState(ctx)
	if err != nil {
		return fmt.Errorf("store: read restored projection state: %w", err)
	}
	if state.ProjectionVersion != transcript.ProjectionVersion &&
		state.ProjectionVersion != whitespaceProjectionVersion {
		return fmt.Errorf("%w: stored version %d", transcript.ErrProjectionIncompatible, state.ProjectionVersion)
	}
	restored, err := rebuildCompactionEntryIdentities(ctx, tx, sessionID, keys)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := restoreCompactionTranscriptEntry(ctx, tx, sessionID, key, keys, restored[key]); err != nil {
			return err
		}
	}
	if err := queries.AdvanceTranscriptProjectionGeneration(ctx); err != nil {
		return fmt.Errorf("store: advance restored compaction generation: %w", err)
	}
	return nil
}

func restoreCompactionTranscriptEntry(
	ctx context.Context,
	tx *store.WriteTx,
	sessionID, key string,
	restoredKeys []string,
	rebuilt restoredEntryIdentity,
) error {
	events, err := loadAssignedEvents(ctx, tx, sessionID, key)
	if err != nil {
		return err
	}
	events = slices.DeleteFunc(events, func(event store.SessionEvent) bool { return event.Archived })
	identity, toolRoutes := rebuilt.identity, rebuilt.routes
	candidate := identity
	candidate.MessageID = candidate.BaseMessageID
	projected, err := transcript.ProjectAssignedEntry(events, candidate)
	if err != nil {
		return fmt.Errorf("store: inspect restored entry %q: %w", key, err)
	}
	if projected != nil {
		identity.MessageID, err = allocateProjectionMessageID(ctx, tx, key, identity.BaseMessageID)
		if err != nil {
			return err
		}
	}
	entry, err := transcript.ProjectAssignedEntry(events, identity)
	if err != nil {
		return fmt.Errorf("store: reproject restored entry %q: %w", key, err)
	}
	if err := persistProjectionEntry(ctx, tx, identity, entry); err != nil {
		return err
	}
	for toolKey := range toolRoutes {
		if routed, found, err := (projectionSQLResolver{db: tx}).ToolEntryIdentity(ctx, toolKey); err != nil {
			return err
		} else if found && !slices.Contains(restoredKeys, routed.Key) {
			continue
		}
		if err := sqlcgen.New(tx).UpsertTranscriptToolRoute(ctx, sqlcgen.UpsertTranscriptToolRouteParams{
			ToolKey: toolKey, EntryKey: key,
		}); err != nil {
			return fmt.Errorf("store: restore transcript tool route %q: %w", toolKey, err)
		}
	}
	return nil
}

type restoredEntryIdentity struct {
	identity transcript.EntryIdentity
	routes   map[string]string
}

// rebuildCompactionEntryIdentities replays the keyed ledger once in sequence order
// and snapshots each restored entry when the replay passes that entry's cutoff.
// The replay never depends on the target entry, so one pass serves every key.
func rebuildCompactionEntryIdentities(
	ctx context.Context,
	tx *store.WriteTx,
	sessionID string,
	keys []string,
) (map[string]restoredEntryIdentity, error) {
	queries := sqlcgen.New(tx)
	type target struct {
		key    string
		cutoff int64
	}
	targets := make([]target, 0, len(keys))
	for _, key := range keys {
		cutoff, err := queries.GetTranscriptEntryUpgradeCutoff(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("store: load restored entry cutoff: %w", err)
		}
		targets = append(targets, target{key: key, cutoff: cutoff})
	}
	slices.SortStableFunc(targets, func(a, b target) int { return cmp.Compare(a.cutoff, b.cutoff) })
	rows, err := queries.ListTranscriptKeyedEventsThrough(ctx, targets[len(targets)-1].cutoff)
	if err != nil {
		return nil, fmt.Errorf("store: load restored entry context: %w", err)
	}
	replay := compactionProjectionReplay{
		identities: make(map[string]transcript.EntryIdentity),
		routes:     make(map[string]string),
	}
	restored := make(map[string]restoredEntryIdentity, len(targets))
	next := 0
	snapshotThrough := func(sequence int64) error {
		for ; next < len(targets) && targets[next].cutoff < sequence; next++ {
			key := targets[next].key
			identity, found := replay.identities[key]
			if !found {
				return fmt.Errorf("%w: missing restored identity %q", transcript.ErrProjectionCorrupt, key)
			}
			routes := maps.Clone(replay.routes)
			maps.DeleteFunc(routes, func(_ string, entryKey string) bool { return entryKey != key })
			restored[key] = restoredEntryIdentity{identity: identity, routes: routes}
		}
		return nil
	}
	for _, row := range rows {
		if err := snapshotThrough(row.Sequence); err != nil {
			return nil, err
		}
		event, err := sessionEventFromSQLC(
			row.ID,
			row.Sequence,
			row.TurnID,
			row.Type,
			row.AgentName,
			row.Content,
			row.Archived,
			row.Timestamp,
			sessionID,
		)
		if err != nil {
			return nil, err
		}
		if err := replay.assign(ctx, event, row.TranscriptEntryKey); err != nil {
			return nil, err
		}
	}
	if err := snapshotThrough(math.MaxInt64); err != nil {
		return nil, err
	}
	return restored, nil
}

type compactionProjectionReplay struct {
	identities map[string]transcript.EntryIdentity
	routes     map[string]string
	activeKey  string
}

func (r *compactionProjectionReplay) assign(ctx context.Context, event store.SessionEvent, key string) error {
	prefix, _, ok := strings.Cut(key, ":")
	generation, err := strconv.ParseInt(strings.TrimPrefix(prefix, "g"), 10, 64)
	if !ok || err != nil || generation < 0 {
		return fmt.Errorf("%w: invalid restored entry key %q", transcript.ErrProjectionCorrupt, key)
	}
	state := transcript.ProjectionState{Version: transcript.ProjectionVersion, Generation: generation}
	classifier, err := transcript.NewProjector(state, nil)
	if err != nil {
		return err
	}
	classification, err := classifier.Assign(ctx, event)
	if err != nil {
		return err
	}
	state.ActiveEntryKey = r.activeKey
	resolver := assignedProjectionResolver{
		identities:  r.identities,
		assignedKey: key,
		boundary:    classification.Entry.Kind != transcript.EntryKindAssistant,
	}
	projector, err := transcript.NewProjector(state, resolver)
	if err != nil {
		return err
	}
	assignment, err := projector.Assign(ctx, event)
	if err != nil {
		return err
	}
	if assignment.Entry.Key != key {
		return fmt.Errorf(
			"%w: inconsistent historical assignment %q at sequence %d",
			transcript.ErrProjectionCorrupt,
			key,
			event.Sequence,
		)
	}
	r.identities[key] = assignment.Entry
	for _, completedKey := range assignment.CompletedKeys {
		identity, found := projector.Identity(completedKey)
		if !found {
			return fmt.Errorf("%w: missing completed identity %q", transcript.ErrProjectionCorrupt, completedKey)
		}
		r.identities[completedKey] = identity
	}
	maps.Copy(r.routes, projector.ToolRoutes())
	r.activeKey = projector.State().ActiveEntryKey
	return nil
}

// Persisted ownership fences stale active entries and reused tool IDs without changing canonical completion rules.
var _ transcript.ProjectionResolver = assignedProjectionResolver{}

type assignedProjectionResolver struct {
	identities  map[string]transcript.EntryIdentity
	assignedKey string
	boundary    bool
}

func (r assignedProjectionResolver) EntryIdentity(
	_ context.Context,
	key string,
) (transcript.EntryIdentity, bool, error) {
	if !r.boundary && key != r.assignedKey {
		return transcript.EntryIdentity{}, false, nil
	}
	identity, found := r.identities[key]
	return identity, found, nil
}

func (r assignedProjectionResolver) ToolEntryIdentity(
	_ context.Context,
	_ string,
) (transcript.EntryIdentity, bool, error) {
	identity, found := r.identities[r.assignedKey]
	return identity, found && identity.Kind == transcript.EntryKindAssistant, nil
}

func (r assignedProjectionResolver) LatestAssistantIdentity(
	_ context.Context,
	turnID string,
) (transcript.EntryIdentity, bool, error) {
	identity, found := r.identities[r.assignedKey]
	return identity, found && identity.Kind == transcript.EntryKindAssistant && identity.TurnID == turnID, nil
}
