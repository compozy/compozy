package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ConditionalUpsert performs an upsert with optional If-Match handling.
// When ifMatch is non-empty, it calls PutIfMatch and returns the new ETag
// without attempting a Get. When ifMatch is empty, it performs a Get to
// determine whether the resource will be created, then calls Put.
// It returns the new ETag, whether the resource was created, and any error.
func ConditionalUpsert(
	ctx context.Context,
	store ResourceStore,
	key ResourceKey,
	value any,
	ifMatch string,
) (ETag, bool, error) {
	trimmed := strings.TrimSpace(ifMatch)
	if trimmed != "" {
		etag, err := store.PutIfMatch(ctx, key, value, ETag(trimmed))
		if err != nil {
			return "", false, fmt.Errorf("conditional put failed: %w", err)
		}
		return etag, false, nil
	}
	_, _, err := store.Get(ctx, key)
	created := errors.Is(err, ErrNotFound)
	if err != nil && !created {
		return "", false, fmt.Errorf("inspect resource: %w", err)
	}
	etag, putErr := store.Put(ctx, key, value)
	if putErr != nil {
		return "", false, fmt.Errorf("put resource: %w", putErr)
	}
	return etag, created, nil
}
