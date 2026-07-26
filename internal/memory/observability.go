package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
)

func filterCatalogFiltersByWorkspaceID(filters []catalogFilter, workspaceID string) []catalogFilter {
	trimmed := strings.TrimSpace(workspaceID)
	filtered := make([]catalogFilter, 0, len(filters))
	for _, filter := range filters {
		if strings.TrimSpace(filter.workspaceID) == trimmed {
			filtered = append(filtered, filter)
		}
	}
	return filtered
}

func (c *catalog) listEventSummaries(
	ctx context.Context,
	sourceID string,
	filters []catalogFilter,
	query storepkg.EventSummaryQuery,
) ([]storepkg.EventSummary, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}

	sqlQuery, args := memoryEventSummarySQL(query, filters)
	rows, err := db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("memory: query memory events: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	summaries := make([]storepkg.EventSummary, 0)
	for rows.Next() {
		summary, scanErr := scanMemoryEventSummary(rows, sourceID)
		if scanErr != nil {
			return nil, scanErr
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate memory events: %w", err)
	}
	return summaries, nil
}

func memoryEventSummarySQL(query storepkg.EventSummaryQuery, filters []catalogFilter) (string, []any) {
	base := `SELECT id, op, COALESCE(session_id, '') AS session_id,
		COALESCE(workspace_id, '') AS workspace_id, COALESCE(agent_name, '') AS agent_name,
		COALESCE(actor_kind, '') AS actor_kind, '' AS actor_id, metadata, ts_ms
		FROM memory_events`
	since := int64(0)
	if !query.Since.IsZero() {
		since = timeToUnixMillis(query.Since.UTC())
	}
	where, args := storepkg.BuildClauses(
		storepkg.StringClause("workspace_id", query.WorkspaceID),
		storepkg.StringClause("session_id", query.SessionID),
		storepkg.StringClause("agent_name", query.AgentName),
		storepkg.StringClause("op", query.Type),
		storepkg.Int64Clause("ts_ms", ">=", since),
	)
	filterWhere, filterArgs := memoryEventSummaryVisibilityClause(filters)
	if filterWhere != "" {
		where = append(where, filterWhere)
		args = append(args, filterArgs...)
	}
	base = storepkg.AppendWhere(base, where)
	if query.Limit <= 0 {
		return base + ` ORDER BY ts_ms ASC, id ASC`, args
	}
	args = append(args, query.Limit)
	return `SELECT id, op, session_id, workspace_id, agent_name, actor_kind, actor_id, metadata, ts_ms
		FROM (` + base + ` ORDER BY ts_ms DESC, id DESC LIMIT ?)
		ORDER BY ts_ms ASC, id ASC`, args
}

func memoryEventSummaryVisibilityClause(filters []catalogFilter) (string, []any) {
	if len(filters) == 0 {
		return "", nil
	}

	clauses := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	hasWorkspaceFilter := false
	for _, filter := range filters {
		if filter.scope.Normalize() == memcontract.ScopeWorkspace {
			hasWorkspaceFilter = true
			break
		}
	}
	for _, filter := range filters {
		switch filter.scope.Normalize() {
		case memcontract.ScopeGlobal:
			if hasWorkspaceFilter {
				clauses = append(
					clauses,
					"((COALESCE(scope, '') = '' AND COALESCE(workspace_id, '') = '') OR scope = 'global')",
				)
				continue
			}
			clauses = append(clauses, "(COALESCE(scope, '') = '' OR scope = 'global')")
		case memcontract.ScopeWorkspace:
			workspaceID := strings.TrimSpace(filter.workspaceID)
			if workspaceID == "" {
				continue
			}
			clauses = append(clauses, "((scope = 'workspace' OR COALESCE(scope, '') = '') AND workspace_id = ?)")
			args = append(args, workspaceID)
		case memcontract.ScopeAgent:
			workspaceID := strings.TrimSpace(filter.workspaceID)
			if workspaceID == "" {
				continue
			}
			clauses = append(clauses, "(scope = 'agent' AND workspace_id = ?)")
			args = append(args, workspaceID)
		}
	}
	if len(clauses) == 0 {
		return "1 = 0", nil
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func scanMemoryEventSummary(scanner memoryEventSummaryScanner, sourceID string) (storepkg.EventSummary, error) {
	var (
		rowID       int64
		op          string
		sessionID   string
		workspaceID string
		agentName   string
		actorKind   string
		actorID     string
		rawMeta     string
		tsMillis    int64
	)
	if err := scanner.Scan(
		&rowID,
		&op,
		&sessionID,
		&workspaceID,
		&agentName,
		&actorKind,
		&actorID,
		&rawMeta,
		&tsMillis,
	); err != nil {
		return storepkg.EventSummary{}, fmt.Errorf("memory: scan memory event summary: %w", err)
	}
	metadata, err := parseMemoryEventMetadata(rawMeta)
	if err != nil {
		return storepkg.EventSummary{}, err
	}
	id := fmt.Sprintf("memevt-%s-%020d", sanitizeEventSourceID(sourceID), rowID)
	return storepkg.EventSummary{
		ID:          id,
		Type:        strings.TrimSpace(op),
		SessionID:   strings.TrimSpace(sessionID),
		WorkspaceID: strings.TrimSpace(workspaceID),
		AgentName:   strings.TrimSpace(agentName),
		EventCorrelation: storepkg.EventCorrelation{
			ActorKind: strings.TrimSpace(actorKind),
			ActorID:   strings.TrimSpace(actorID),
		},
		Summary:   strings.TrimSpace(metadata[memoryEventMetadataSummaryKey]),
		Timestamp: timeFromUnixMillis(tsMillis),
	}, nil
}

type memoryEventSummaryScanner interface {
	Scan(dest ...any) error
}

func parseMemoryEventMetadata(raw string) (map[string]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]string{}, nil
	}
	values := make(map[string]string)
	if err := json.Unmarshal([]byte(trimmed), &values); err == nil {
		return values, nil
	}

	var generic map[string]any
	if err := json.Unmarshal([]byte(trimmed), &generic); err != nil {
		return nil, fmt.Errorf("memory: decode memory event metadata: %w", err)
	}
	for key, value := range generic {
		values[key] = fmt.Sprint(value)
	}
	return values, nil
}

func sanitizeEventSourceID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-")
	return replacer.Replace(trimmed)
}

func sortEventSummaries(summaries []storepkg.EventSummary) {
	sort.SliceStable(summaries, func(i, j int) bool {
		left := summaries[i]
		right := summaries[j]
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

func clampEventSummaries(summaries []storepkg.EventSummary, limit int) []storepkg.EventSummary {
	if limit <= 0 || len(summaries) <= limit {
		return summaries
	}
	return append([]storepkg.EventSummary(nil), summaries[len(summaries)-limit:]...)
}

func catalogHealthKey(source observabilitySource, entry catalogDocument) string {
	return source.id + "::" + strings.TrimSpace(entry.ID)
}

func actualCatalogKey(source observabilitySource, id string) string {
	return source.id + "::" + strings.TrimSpace(id)
}

type healthAccumulator struct {
	entriesByID     map[string]catalogDocument
	actualByID      map[string]struct{}
	lastReindex     *time.Time
	lastOperationAt *time.Time
	operationCount  int
}

func newHealthAccumulator() *healthAccumulator {
	return &healthAccumulator{
		entriesByID: make(map[string]catalogDocument),
		actualByID:  make(map[string]struct{}),
	}
}

func (a *healthAccumulator) addSource(ctx context.Context, source observabilitySource) error {
	if err := ensureObservabilitySourceReady(ctx, source); err != nil {
		return err
	}
	if err := a.addCatalogEntries(ctx, source); err != nil {
		return err
	}
	if err := a.addActualEntries(source); err != nil {
		return err
	}
	if err := a.addReindexTimestamp(ctx, source); err != nil {
		return err
	}
	return a.addOperationStats(ctx, source)
}

func ensureObservabilitySourceReady(ctx context.Context, source observabilitySource) error {
	for _, filter := range source.filters {
		if err := source.store.ensureCatalogFilterReady(ctx, filter); err != nil {
			return err
		}
	}
	return nil
}

func (a *healthAccumulator) addCatalogEntries(ctx context.Context, source observabilitySource) error {
	entries, err := source.catalog.listEntries(ctx, source.filters)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		a.entriesByID[catalogHealthKey(source, entry)] = entry
	}
	return nil
}

func (a *healthAccumulator) addActualEntries(source observabilitySource) error {
	actual, err := source.store.collectActualCatalogIDs(source.filters)
	if err != nil {
		return err
	}
	for id := range actual {
		a.actualByID[actualCatalogKey(source, id)] = struct{}{}
	}
	return nil
}

func (a *healthAccumulator) addReindexTimestamp(ctx context.Context, source observabilitySource) error {
	reindexedAt, err := source.catalog.lastReindex(ctx)
	if err != nil {
		return err
	}
	if a.lastReindex == nil || reindexedAt != nil && reindexedAt.After(*a.lastReindex) {
		a.lastReindex = reindexedAt
	}
	return nil
}

func (a *healthAccumulator) addOperationStats(ctx context.Context, source observabilitySource) error {
	count, operatedAt, err := source.catalog.operationStats(ctx, source.filters)
	if err != nil {
		return err
	}
	a.operationCount += count
	if a.lastOperationAt == nil || operatedAt != nil && operatedAt.After(*a.lastOperationAt) {
		a.lastOperationAt = operatedAt
	}
	return nil
}

func (a *healthAccumulator) stats() memcontract.HealthStats {
	orphaned := 0
	for key := range a.entriesByID {
		if _, exists := a.actualByID[key]; !exists {
			orphaned++
		}
	}
	return memcontract.HealthStats{
		IndexedFiles:    len(a.entriesByID),
		OrphanedFiles:   orphaned,
		LastReindex:     a.lastReindex,
		OperationCount:  a.operationCount,
		LastOperationAt: a.lastOperationAt,
	}
}
