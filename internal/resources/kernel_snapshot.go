package resources

import (
	"context"

	"fmt"

	"time"

	"github.com/compozy/agh/internal/store"
)

func (k *Kernel) prepareSnapshotApply(
	actor MutationActor,
	snapshot SourceSnapshot,
) (MutationActor, SourceSnapshot, []RawDraft, error) {
	normalizedActor, err := normalizeActor(actor)
	if err != nil {
		return MutationActor{}, SourceSnapshot{}, nil, err
	}
	if normalizedActor.Kind != MutationActorKindExtension {
		return MutationActor{}, SourceSnapshot{}, nil, fmt.Errorf(
			"%w: only extension actors may apply source snapshots",
			ErrPermissionDenied,
		)
	}
	if normalizedActor.SessionNonce == "" {
		return MutationActor{}, SourceSnapshot{}, nil, fmt.Errorf(
			"%w: actor.session_nonce is required",
			ErrValidation,
		)
	}
	if k.extensionReadGrantsEmpty(normalizedActor) {
		return MutationActor{}, SourceSnapshot{}, nil, fmt.Errorf(
			"%w: extension actor requires granted kinds and scopes",
			ErrPermissionDenied,
		)
	}

	normalizedSnapshot, err := normalizeSnapshot(snapshot, k.maxSnapshotRecords)
	if err != nil {
		return MutationActor{}, SourceSnapshot{}, nil, err
	}

	normalizedDrafts := make([]RawDraft, 0, len(snapshot.Records))
	seenKeys := make(map[string]struct{}, len(snapshot.Records))
	totalBytes := 0
	for _, draft := range snapshot.Records {
		normalizedDraft, draftErr := normalizeDraft(draft, k.maxSpecBytes)
		if draftErr != nil {
			return MutationActor{}, SourceSnapshot{}, nil, draftErr
		}
		if normalizedDraft.ExpectedVersion != 0 {
			return MutationActor{}, SourceSnapshot{}, nil, fmt.Errorf(
				"%w: snapshot records must use expected_version 0",
				ErrValidation,
			)
		}
		if accessErr := validateActorWriteAccess(
			normalizedActor,
			normalizedDraft.Kind,
			normalizedDraft.Scope,
		); accessErr != nil {
			return MutationActor{}, SourceSnapshot{}, nil, accessErr
		}

		key := resourceKey(normalizedDraft.Kind, normalizedDraft.ID)
		if _, exists := seenKeys[key]; exists {
			return MutationActor{}, SourceSnapshot{}, nil, fmt.Errorf(
				"%w: duplicate snapshot record %q/%q",
				ErrValidation,
				normalizedDraft.Kind,
				normalizedDraft.ID,
			)
		}
		seenKeys[key] = struct{}{}

		totalBytes += len(normalizedDraft.SpecJSON)
		if totalBytes > k.maxSnapshotBytes {
			return MutationActor{}, SourceSnapshot{}, nil, fmt.Errorf(
				"%w: snapshot payload exceeds %d bytes",
				ErrPayloadTooLarge,
				k.maxSnapshotBytes,
			)
		}
		normalizedDrafts = append(normalizedDrafts, normalizedDraft)
	}

	return normalizedActor, normalizedSnapshot, normalizedDrafts, nil
}

func (k *Kernel) applySnapshotWithExecutor(
	ctx context.Context,
	exec sqlExecutor,
	actor MutationActor,
	snapshot SourceSnapshot,
	drafts []RawDraft,
) error {
	state, err := k.requireActiveSourceState(ctx, exec, actor)
	if err != nil {
		return err
	}

	existingRecords, err := listSourceRecordsWithExecutor(ctx, exec, actor.Source)
	if err != nil {
		return err
	}
	existingByKey := make(map[string]RawRecord, len(existingRecords))
	for _, record := range existingRecords {
		existingByKey[resourceKey(record.Kind, record.ID)] = record
	}

	if err := k.applySnapshotDrafts(ctx, exec, actor, drafts, existingByKey); err != nil {
		return err
	}
	if err := deleteRemovedSourceRecords(ctx, exec, actor.Source, existingRecords, drafts); err != nil {
		return err
	}
	return advanceSourceState(ctx, exec, actor, state, snapshot.SourceVersion, k.now())
}

func (k *Kernel) requireActiveSourceState(
	ctx context.Context,
	exec sqlExecutor,
	actor MutationActor,
) (sourceState, error) {
	state, found, err := lookupSourceState(ctx, exec, actor.Source)
	if err != nil {
		return sourceState{}, err
	}
	if !found || state.SessionNonce != actor.SessionNonce {
		return sourceState{}, fmt.Errorf(
			"%w: source %q/%q nonce %q",
			ErrSessionNotActive,
			actor.Source.Kind,
			actor.Source.ID,
			actor.SessionNonce,
		)
	}
	return state, nil
}

func (k *Kernel) applySnapshotDrafts(
	ctx context.Context,
	exec sqlExecutor,
	actor MutationActor,
	drafts []RawDraft,
	existingByKey map[string]RawRecord,
) error {
	for _, draft := range drafts {
		if err := k.applySnapshotDraft(ctx, exec, actor, draft, existingByKey); err != nil {
			return err
		}
	}
	return nil
}

func (k *Kernel) applySnapshotDraft(
	ctx context.Context,
	exec sqlExecutor,
	actor MutationActor,
	draft RawDraft,
	existingByKey map[string]RawRecord,
) error {
	key := resourceKey(draft.Kind, draft.ID)
	existing, found := existingByKey[key]
	if !found {
		record, lookupFound, err := lookupRecordWithExecutor(ctx, exec, draft.Kind, draft.ID)
		if err != nil {
			return err
		}
		if lookupFound {
			return fmt.Errorf(
				"%w: snapshot cannot overwrite source %q/%q",
				ErrConflict,
				record.Source.Kind,
				record.Source.ID,
			)
		}
		_, err = k.insertRawRecord(ctx, exec, actor, draft)
		return err
	}

	nextRecord := RawRecord{
		Kind:      draft.Kind,
		ID:        draft.ID,
		Version:   existing.Version + 1,
		Scope:     draft.Scope,
		Owner:     ownerFromActor(actor),
		Source:    actor.Source,
		SpecJSON:  append([]byte(nil), draft.SpecJSON...),
		CreatedAt: existing.CreatedAt,
		UpdatedAt: k.now(),
	}
	if recordsEqual(existing, nextRecord) {
		return nil
	}
	return updateRecord(ctx, exec, nextRecord, existing.Version)
}

func deleteRemovedSourceRecords(
	ctx context.Context,
	exec sqlExecutor,
	source ResourceSource,
	existingRecords []RawRecord,
	drafts []RawDraft,
) error {
	desiredKeys := make(map[string]struct{}, len(drafts))
	for _, draft := range drafts {
		desiredKeys[resourceKey(draft.Kind, draft.ID)] = struct{}{}
	}
	for _, record := range existingRecords {
		if _, keep := desiredKeys[resourceKey(record.Kind, record.ID)]; keep {
			continue
		}
		if _, err := exec.ExecContext(
			ctx,
			deleteStaleSourceRecordQuery,
			record.Kind,
			record.ID,
			source.Kind,
			source.ID,
		); err != nil {
			return fmt.Errorf(
				"resources: delete stale source record %q/%q for %q/%q: %w",
				record.Kind,
				record.ID,
				source.Kind,
				source.ID,
				err,
			)
		}
	}
	return nil
}

func advanceSourceState(
	ctx context.Context,
	exec sqlExecutor,
	actor MutationActor,
	state sourceState,
	sourceVersion int64,
	now time.Time,
) error {
	if sourceVersion <= state.LastSnapshotVersion {
		return fmt.Errorf(
			"%w: expected > %d, got %d",
			ErrStaleSourceVersion,
			state.LastSnapshotVersion,
			sourceVersion,
		)
	}

	result, err := exec.ExecContext(
		ctx,
		updateSourceStateQuery,
		sourceVersion,
		store.FormatTimestamp(now),
		actor.Source.Kind,
		actor.Source.ID,
		actor.SessionNonce,
	)
	if err != nil {
		return fmt.Errorf(
			"resources: update source state %q/%q: %w",
			actor.Source.Kind,
			actor.Source.ID,
			err,
		)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"resources: rows affected for source state %q/%q: %w",
			actor.Source.Kind,
			actor.Source.ID,
			err,
		)
	}
	if rowsAffected == 0 {
		return fmt.Errorf(
			"%w: source %q/%q nonce %q",
			ErrSessionNotActive,
			actor.Source.Kind,
			actor.Source.ID,
			actor.SessionNonce,
		)
	}
	return nil
}
