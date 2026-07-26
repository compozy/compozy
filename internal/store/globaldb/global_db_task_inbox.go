package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	taskpkg "github.com/compozy/agh/internal/task"
)

var _ taskpkg.InboxReader = (*TaskRepo)(nil)

type taskInboxMetadata struct {
	total          int
	unreadTotal    int
	archivedTotal  int
	groupByLane    map[taskpkg.InboxLane]taskpkg.InboxLaneGroup
	statusFacets   []taskpkg.InboxStatusFacet
	priorityFacets []taskpkg.InboxPriorityFacet
}

// ListTaskInbox returns one transactionally consistent actor-scoped inbox page.
func (g *TaskRepo) ListTaskInbox(
	ctx context.Context,
	query taskpkg.InboxQuery,
	actor taskpkg.ActorIdentity,
) (page taskpkg.InboxPage, err error) {
	if err := g.checkReady(ctx, "list task inbox"); err != nil {
		return taskpkg.InboxPage{}, err
	}
	normalized, normalizedActor, err := taskpkg.NormalizeInboxQuery(query, actor)
	if err != nil {
		return taskpkg.InboxPage{}, err
	}

	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return taskpkg.InboxPage{}, fmt.Errorf("store: begin task inbox read: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: rollback task inbox read: %w", rollbackErr))
		}
	}()

	page, err = listTaskInboxWithExecutor(ctx, tx, normalized, normalizedActor)
	if err != nil {
		return taskpkg.InboxPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return taskpkg.InboxPage{}, fmt.Errorf("store: commit task inbox read: %w", err)
	}
	committed = true
	return page, nil
}

func listTaskInboxWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	query taskpkg.InboxQuery,
	actor taskpkg.ActorIdentity,
) (taskpkg.InboxPage, error) {
	baseWhere, baseFilterArgs := taskInboxBaseFilter(query)
	where, filterArgs := taskInboxFilter(query)
	actorArgs := taskInboxArgs(actor)
	baseArgs := append(append([]any(nil), baseFilterArgs...), actorArgs...)
	total, unreadTotal, archivedTotal, groups, statusFacets, priorityFacets, err := queryTaskInboxMetadata(
		ctx,
		exec,
		query,
		baseWhere,
		where,
		append(append([]any(nil), baseArgs...), filterArgs...),
	)
	if err != nil {
		return taskpkg.InboxPage{}, err
	}
	items, err := queryTaskInboxPage(ctx, exec, query, actor, baseWhere, where, baseArgs, filterArgs)
	if err != nil {
		return taskpkg.InboxPage{}, err
	}

	page := taskpkg.InboxPage{
		Groups:         groups,
		StatusFacets:   statusFacets,
		PriorityFacets: priorityFacets,
		Total:          total,
		UnreadTotal:    unreadTotal,
		ArchivedTotal:  archivedTotal,
		Limit:          query.Limit,
	}
	if len(items) > query.Limit {
		page.HasMore = true
		items = items[:query.Limit]
		page.NextCursor, err = taskpkg.EncodeInboxCursor(query, actor, items[len(items)-1])
		if err != nil {
			return taskpkg.InboxPage{}, err
		}
	}
	itemsByLane := make(map[taskpkg.InboxLane][]taskpkg.InboxItem, len(page.Groups))
	for _, item := range items {
		itemsByLane[item.Lane] = append(itemsByLane[item.Lane], item)
	}
	for index := range page.Groups {
		page.Groups[index].Items = itemsByLane[page.Groups[index].Lane]
	}
	return page, nil
}

func queryTaskInboxMetadata(
	ctx context.Context,
	exec taskSQLExecutor,
	query taskpkg.InboxQuery,
	baseWhere []string,
	where []string,
	args []any,
) (
	total int,
	unreadTotal int,
	archivedTotal int,
	groups []taskpkg.InboxLaneGroup,
	statusFacets []taskpkg.InboxStatusFacet,
	priorityFacets []taskpkg.InboxPriorityFacet,
	err error,
) {
	filtered := taskInboxStatement(
		"SELECT lane, status, priority, is_unread FROM inbox",
		baseWhere,
		where,
	)
	statement := `WITH filtered_inbox AS (` + filtered + `)
		SELECT 'total', '', COUNT(*), COALESCE(SUM(is_unread), 0),
			COALESCE(SUM(CASE WHEN lane = 'archived' THEN 1 ELSE 0 END), 0) FROM filtered_inbox
		UNION ALL
		SELECT 'lane', lane, COUNT(*), COALESCE(SUM(is_unread), 0), 0 FROM filtered_inbox GROUP BY lane
		UNION ALL
		SELECT 'status', status, COUNT(*), 0, 0 FROM filtered_inbox GROUP BY status
		UNION ALL
		SELECT 'priority', priority, COUNT(*), 0, 0 FROM filtered_inbox GROUP BY priority
		ORDER BY 1 ASC, 2 ASC`
	// dynamic-sql: inbox ownership, attention, filter, and pagination clauses alter statement structure.
	rows, err := exec.QueryContext(ctx, statement, args...)
	if err != nil {
		return 0, 0, 0, nil, nil, nil, fmt.Errorf("store: query task inbox metadata: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close task inbox facets: %w", closeErr))
		}
	}()

	metadata, err := scanTaskInboxMetadata(rows)
	if err != nil {
		return 0, 0, 0, nil, nil, nil, err
	}
	groups = make([]taskpkg.InboxLaneGroup, 0, len(taskpkg.InboxLanes(query.Lane)))
	for _, lane := range taskpkg.InboxLanes(query.Lane) {
		group := metadata.groupByLane[lane]
		group.Lane = lane
		groups = append(groups, group)
	}
	return metadata.total,
		metadata.unreadTotal,
		metadata.archivedTotal,
		groups,
		metadata.statusFacets,
		metadata.priorityFacets,
		nil
}

func scanTaskInboxMetadata(rows *sql.Rows) (taskInboxMetadata, error) {
	metadata := taskInboxMetadata{
		groupByLane: make(map[taskpkg.InboxLane]taskpkg.InboxLaneGroup),
	}
	for rows.Next() {
		var kind string
		var value string
		var count int
		var unreadCount int
		var archivedCount int
		if err := rows.Scan(&kind, &value, &count, &unreadCount, &archivedCount); err != nil {
			return taskInboxMetadata{}, fmt.Errorf("store: scan task inbox metadata: %w", err)
		}
		metadata.record(kind, value, count, unreadCount, archivedCount)
	}
	if err := rows.Err(); err != nil {
		return taskInboxMetadata{}, fmt.Errorf("store: iterate task inbox metadata: %w", err)
	}
	return metadata, nil
}

func (m *taskInboxMetadata) record(kind string, value string, count int, unreadCount int, archivedCount int) {
	switch kind {
	case taskMetadataKindTotal:
		m.total = count
		m.unreadTotal = unreadCount
		m.archivedTotal = archivedCount
	case taskMetadataKindLane:
		lane := taskpkg.InboxLane(value).Normalize()
		m.groupByLane[lane] = taskpkg.InboxLaneGroup{Lane: lane, Count: count, UnreadCount: unreadCount}
	case taskMetadataKindStatus:
		m.statusFacets = append(m.statusFacets, taskpkg.InboxStatusFacet{
			Status: taskpkg.Status(value).Normalize(),
			Count:  count,
		})
	case taskMetadataKindPriority:
		m.priorityFacets = append(m.priorityFacets, taskpkg.InboxPriorityFacet{
			Priority: taskpkg.Priority(value).Normalize(),
			Count:    count,
		})
	}
}

func queryTaskInboxPage(
	ctx context.Context,
	exec taskSQLExecutor,
	query taskpkg.InboxQuery,
	actor taskpkg.ActorIdentity,
	baseWhere []string,
	where []string,
	baseArgs []any,
	filterArgs []any,
) (items []taskpkg.InboxItem, err error) {
	pageWhere, pageFilterArgs, err := taskInboxPageFilter(
		query,
		actor,
		append([]string(nil), where...),
		append([]any(nil), filterArgs...),
	)
	if err != nil {
		return nil, err
	}
	statement := taskInboxStatement("SELECT "+taskInboxSelectColumns+" FROM inbox", baseWhere, pageWhere) +
		" ORDER BY is_unread DESC, last_activity_at DESC, priority_rank DESC, id ASC" +
		taskCatalogLimitClause(query.Limit+1)
	args := append(append([]any(nil), baseArgs...), pageFilterArgs...)
	// dynamic-sql: inbox aggregate filters share the structurally composed predicate set.
	rows, err := exec.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query task inbox page: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close task inbox page: %w", closeErr))
		}
	}()

	items = make([]taskpkg.InboxItem, 0, query.Limit+1)
	for rows.Next() {
		item, scanErr := scanTaskInboxItem(rows, actor)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate task inbox page: %w", err)
	}
	return items, nil
}
