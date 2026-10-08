package observe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
)

// QueryEvents returns cross-session event summaries ordered for CLI/API consumption.
func (o *Observer) QueryEvents(ctx context.Context, query store.EventSummaryQuery) ([]store.EventSummary, error) {
	if ctx == nil {
		return nil, errors.New("observe: query events context is required")
	}
	events, err := o.registry.ListEventSummaries(ctx, query)
	if err != nil {
		return nil, err
	}

	return events, nil
}

// QueryTokenStats returns aggregated per-session token usage rows.
func (o *Observer) QueryTokenStats(ctx context.Context, query store.TokenStatsQuery) ([]store.TokenStats, error) {
	return o.registry.ListTokenStats(ctx, query)
}

// QueryPermissionLog returns permission audit rows.
func (o *Observer) QueryPermissionLog(
	ctx context.Context,
	query store.PermissionLogQuery,
) ([]store.PermissionLogEntry, error) {
	return o.registry.ListPermissionLog(ctx, query)
}

// QueryHookCatalog returns the resolved hook catalog for the supplied filter.
func (o *Observer) QueryHookCatalog(
	ctx context.Context,
	filter hookspkg.CatalogFilter,
) ([]hookspkg.CatalogEntry, error) {
	if ctx == nil {
		return nil, errors.New("observe: hook catalog context is required")
	}

	o.mu.RLock()
	source := o.hookCatalogSource
	o.mu.RUnlock()
	if source == nil {
		return nil, nil
	}

	return source.Catalog(filter)
}

// QueryHookRuns returns persisted per-session hook execution records.
func (o *Observer) QueryHookRuns(
	ctx context.Context,
	query store.HookRunQuery,
) (records []hookspkg.HookRunRecord, err error) {
	if ctx == nil {
		return nil, errors.New("observe: hook runs context is required")
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if event := strings.TrimSpace(query.Event); event != "" {
		if err := hookspkg.HookEvent(event).Validate(); err != nil {
			return nil, err
		}
	}

	storeHandle, cleanup, err := o.openHookRunStore(ctx, query.SessionID)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	default:
		return nil, err
	}
	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil && err == nil {
			err = cleanupErr
		}
	}()

	records, err = storeHandle.QueryHookRuns(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("observe: query hook runs for %q: %w", strings.TrimSpace(query.SessionID), err)
	}
	return records, nil
}

// QueryHookEvents returns the supported hook taxonomy metadata.
func (o *Observer) QueryHookEvents(_ context.Context, filter hookspkg.EventFilter) ([]hookspkg.EventDescriptor, error) {
	return hookspkg.FilterEventDescriptors(filter), nil
}

// WriteHookRecord persists one hook execution record when the session database already exists.
func (o *Observer) WriteHookRecord(ctx context.Context, sessionID string, record hookspkg.HookRunRecord) (err error) {
	if ctx == nil {
		return errors.New("observe: write hook record context is required")
	}

	storeHandle, cleanup, err := o.openHookRunStore(ctx, sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil && err == nil {
			err = cleanupErr
		}
	}()

	if err := storeHandle.RecordHookRun(ctx, record); err != nil {
		return fmt.Errorf("observe: write hook record for %q: %w", strings.TrimSpace(sessionID), err)
	}
	return nil
}
