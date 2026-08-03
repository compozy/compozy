package daemon

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/resources"
)

type managedResourceValue[T any] struct {
	id      string
	scope   resources.ResourceScope
	owner   *resources.ResourceOwner
	spec    T
	encoded []byte
}

func validateAndEncodeResource[T any](
	ctx context.Context,
	codec resources.KindCodec[T],
	scope resources.ResourceScope,
	spec T,
) (T, []byte, error) {
	var zero T
	encoded, err := codec.Encode(spec)
	if err != nil {
		return zero, nil, err
	}
	validated, err := codec.DecodeAndValidate(ctx, scope.Normalize(), encoded)
	if err != nil {
		return zero, nil, err
	}
	canonical, err := codec.Encode(validated)
	if err != nil {
		return zero, nil, err
	}
	return validated, canonical, nil
}

func syncManagedResources[T any](
	ctx context.Context,
	actor resources.MutationActor,
	store resources.Store[T],
	codec resources.KindCodec[T],
	desired map[string]managedResourceValue[T],
	label string,
) (bool, error) {
	source := actor.Source
	current, err := store.List(ctx, actor, resources.ResourceFilter{Source: &source})
	if err != nil {
		return false, fmt.Errorf("daemon: list managed %s resources: %w", label, err)
	}
	currentByID := make(map[string]resources.Record[T], len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}

	desiredIDs := make([]string, 0, len(desired))
	for id := range desired {
		desiredIDs = append(desiredIDs, id)
	}
	slices.Sort(desiredIDs)

	changed := false
	for _, id := range desiredIDs {
		value := desired[id]
		existing, ok := currentByID[id]
		if ok {
			existingEncoded, encodeErr := codec.Encode(existing.Spec)
			if encodeErr != nil {
				return false, fmt.Errorf("daemon: encode managed %s %q: %w", label, id, encodeErr)
			}
			if existing.Scope == value.scope && existing.Owner.Normalize() == managedDraftOwner(actor, value.owner) &&
				existing.Source.Normalize() == actor.Source.Normalize() && bytes.Equal(existingEncoded, value.encoded) {
				delete(currentByID, id)
				continue
			}
		}
		expectedVersion := int64(0)
		if ok {
			expectedVersion = existing.Version
		}
		if _, err := store.Put(ctx, actor, resources.Draft[T]{
			ID: value.id, Scope: value.scope, Owner: value.owner, ExpectedVersion: expectedVersion, Spec: value.spec,
		}); err != nil {
			return false, fmt.Errorf("daemon: sync %s %q: %w", label, id, err)
		}
		changed = true
		delete(currentByID, id)
	}

	staleIDs := make([]string, 0, len(currentByID))
	for id := range currentByID {
		staleIDs = append(staleIDs, id)
	}
	slices.Sort(staleIDs)
	for _, id := range staleIDs {
		stale := currentByID[id]
		if err := store.Delete(ctx, actor, stale.ID, stale.Version); err != nil {
			return false, fmt.Errorf("daemon: delete stale %s %q: %w", label, stale.ID, err)
		}
		changed = true
	}
	return changed, nil
}
