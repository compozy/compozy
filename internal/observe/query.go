package observe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	aghworkspace "github.com/compozy/agh/internal/workspace"
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

	o.mu.RLock()
	memorySource := o.memoryEventSource
	o.mu.RUnlock()
	if memorySource == nil || strings.TrimSpace(query.SessionID) != "" {
		return events, nil
	}

	workspaces, err := o.memoryEventWorkspaces(ctx, query.WorkspaceID)
	if err != nil {
		return nil, err
	}
	memoryQuery, err := memoryEventQueryForWorkspaces(ctx, query, workspaces)
	if err != nil {
		return nil, err
	}
	memoryEvents, err := memorySource.ListMemoryEventSummaries(ctx, workspaces, memoryQuery)
	if err != nil {
		return nil, fmt.Errorf("observe: query memory events: %w", err)
	}

	events = append(filterRegistryMemoryEvents(events), memoryEvents...)
	sortEventSummaries(events)
	return clampEventSummaries(events, query.Limit), nil
}

// QueryTokenStats returns aggregated per-session token usage rows.
func (o *Observer) QueryTokenStats(ctx context.Context, query store.TokenStatsQuery) ([]store.TokenStats, error) {
	return o.registry.ListTokenStats(ctx, query)
}

func (o *Observer) memoryEventWorkspaces(ctx context.Context, workspaceID string) ([]string, error) {
	if o.workspaceResolver == nil {
		return nil, nil
	}
	if trimmedWorkspaceID := strings.TrimSpace(workspaceID); trimmedWorkspaceID != "" {
		resolved, err := o.workspaceResolver.Resolve(ctx, trimmedWorkspaceID)
		if err != nil {
			return nil, fmt.Errorf("observe: resolve memory event workspace %q: %w", trimmedWorkspaceID, err)
		}
		if root := strings.TrimSpace(resolved.RootDir); root != "" {
			return []string{root}, nil
		}
		return nil, nil
	}
	sessions, err := o.registry.ListSessions(ctx, store.SessionListQuery{})
	if err != nil {
		return nil, fmt.Errorf("observe: list sessions for memory event workspaces: %w", err)
	}
	seen := make(map[string]struct{})
	workspaces := make([]string, 0, len(sessions))
	for _, session := range sessions {
		workspaceID := strings.TrimSpace(session.WorkspaceID)
		if workspaceID == "" {
			continue
		}
		if _, exists := seen[workspaceID]; exists {
			continue
		}
		seen[workspaceID] = struct{}{}
		resolved, err := o.workspaceResolver.Resolve(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("observe: resolve memory event workspace %q: %w", workspaceID, err)
		}
		if root := strings.TrimSpace(resolved.RootDir); root != "" {
			workspaces = append(workspaces, root)
		}
	}
	return workspaces, nil
}

func filterRegistryMemoryEvents(events []store.EventSummary) []store.EventSummary {
	filtered := events[:0]
	for _, event := range events {
		if strings.HasPrefix(strings.TrimSpace(event.Type), "memory.") {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func sortEventSummaries(events []store.EventSummary) {
	sort.SliceStable(events, func(i, j int) bool {
		left := events[i]
		right := events[j]
		leftAt := left.Timestamp.UTC()
		rightAt := right.Timestamp.UTC()
		if !leftAt.Equal(rightAt) {
			return leftAt.Before(rightAt)
		}
		if left.Sequence != right.Sequence {
			return left.Sequence < right.Sequence
		}
		return left.ID < right.ID
	})
}

func clampEventSummaries(events []store.EventSummary, limit int) []store.EventSummary {
	if limit <= 0 || len(events) <= limit {
		return events
	}
	return append([]store.EventSummary(nil), events[len(events)-limit:]...)
}

func memoryEventQueryForWorkspaces(
	ctx context.Context,
	query store.EventSummaryQuery,
	workspaces []string,
) (store.EventSummaryQuery, error) {
	if strings.TrimSpace(query.WorkspaceID) == "" || len(workspaces) != 1 {
		return query, nil
	}
	workspaceRoot := strings.TrimSpace(workspaces[0])
	if workspaceRoot == "" {
		return query, nil
	}
	identity, err := aghworkspace.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		return store.EventSummaryQuery{}, fmt.Errorf(
			"observe: resolve memory event workspace identity %q: %w",
			workspaceRoot,
			err,
		)
	}
	query.WorkspaceID = identity.WorkspaceID
	return query, nil
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
