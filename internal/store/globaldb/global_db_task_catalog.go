package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	taskpkg "github.com/compozy/agh/internal/task"
)

var _ taskpkg.CatalogReader = (*TaskRepo)(nil)

const (
	taskMetadataKindLane     = "lane"
	taskMetadataKindOwner    = "owner"
	taskMetadataKindPriority = "priority"
	taskMetadataKindStatus   = "status"
	taskMetadataKindTotal    = "total"
)

// ListTaskCatalog returns one transactionally consistent task catalog page.
func (g *TaskRepo) ListTaskCatalog(
	ctx context.Context,
	query taskpkg.CatalogQuery,
) (page taskpkg.CatalogPage, err error) {
	if err := g.checkReady(ctx, "list task catalog"); err != nil {
		return taskpkg.CatalogPage{}, err
	}
	normalized, err := taskpkg.NormalizeCatalogQuery(query)
	if err != nil {
		return taskpkg.CatalogPage{}, err
	}

	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return taskpkg.CatalogPage{}, fmt.Errorf("store: begin task catalog read: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: rollback task catalog read: %w", rollbackErr))
		}
	}()

	page, err = listTaskCatalogWithExecutor(ctx, tx, normalized)
	if err != nil {
		return taskpkg.CatalogPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return taskpkg.CatalogPage{}, fmt.Errorf("store: commit task catalog read: %w", err)
	}
	committed = true
	return page, nil
}

func listTaskCatalogWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	query taskpkg.CatalogQuery,
) (taskpkg.CatalogPage, error) {
	baseWhere, baseArgs := taskCatalogBaseFilter(query)
	where, filterArgs := taskCatalogFilter(query)
	total, statusFacets, ownerFacets, err := queryTaskCatalogMetadata(
		ctx,
		exec,
		baseWhere,
		where,
		append(append([]any(nil), baseArgs...), filterArgs...),
	)
	if err != nil {
		return taskpkg.CatalogPage{}, err
	}
	tasks, err := queryTaskCatalogPage(ctx, exec, query, baseWhere, baseArgs, where, filterArgs)
	if err != nil {
		return taskpkg.CatalogPage{}, err
	}

	page := taskpkg.CatalogPage{
		Tasks:        tasks,
		StatusFacets: statusFacets,
		OwnerFacets:  ownerFacets,
		Total:        total,
		Limit:        query.Limit,
	}
	if len(page.Tasks) <= query.Limit {
		return page, nil
	}
	page.HasMore = true
	page.Tasks = page.Tasks[:query.Limit]
	page.NextCursor, err = taskpkg.EncodeCatalogCursor(query, &page.Tasks[len(page.Tasks)-1])
	if err != nil {
		return taskpkg.CatalogPage{}, err
	}
	return page, nil
}

func queryTaskCatalogMetadata(
	ctx context.Context,
	exec taskSQLExecutor,
	baseWhere []string,
	where []string,
	args []any,
) (
	total int,
	statusFacets []taskpkg.CatalogStatusFacet,
	ownerFacets []taskpkg.CatalogOwnerFacet,
	err error,
) {
	filtered := taskCatalogStatement("SELECT * FROM catalog", baseWhere, where)
	statement := `WITH filtered_catalog AS (` + filtered + `)
		SELECT 'total', '', '', COUNT(*) FROM filtered_catalog
		UNION ALL
		SELECT 'status', status, '', COUNT(*)
		FROM filtered_catalog GROUP BY status
		UNION ALL
		SELECT 'owner', owner_kind, owner_ref, COUNT(*)
		FROM filtered_catalog
		WHERE owner_kind IS NOT NULL AND owner_ref IS NOT NULL
		GROUP BY owner_kind, owner_ref
		ORDER BY 1 ASC, 2 ASC, 3 ASC`
	// dynamic-sql: catalog filters, sort mode, and cursor boundaries alter the statement structure.
	rows, err := exec.QueryContext(ctx, statement, args...)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("store: query task catalog metadata: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close task catalog metadata: %w", closeErr))
		}
	}()

	for rows.Next() {
		var kind string
		var value string
		var secondaryValue string
		var count int
		if scanErr := rows.Scan(&kind, &value, &secondaryValue, &count); scanErr != nil {
			return 0, nil, nil, fmt.Errorf("store: scan task catalog metadata: %w", scanErr)
		}
		switch kind {
		case taskMetadataKindTotal:
			total = count
		case taskMetadataKindStatus:
			statusFacets = append(statusFacets, taskpkg.CatalogStatusFacet{
				Status: taskpkg.Status(value).Normalize(),
				Count:  count,
			})
		case taskMetadataKindOwner:
			ownerFacets = append(ownerFacets, taskpkg.CatalogOwnerFacet{
				Owner: taskpkg.Ownership{
					Kind: taskpkg.OwnerKind(value).Normalize(),
					Ref:  secondaryValue,
				},
				Count: count,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return 0, nil, nil, fmt.Errorf("store: iterate task catalog metadata: %w", err)
	}
	return total, statusFacets, ownerFacets, nil
}

func queryTaskCatalogPage(
	ctx context.Context,
	exec taskSQLExecutor,
	query taskpkg.CatalogQuery,
	baseWhere []string,
	baseArgs []any,
	where []string,
	filterArgs []any,
) (tasks []taskpkg.Summary, err error) {
	pageWhere, pageFilterArgs, err := taskCatalogPageFilter(
		query,
		append([]string(nil), where...),
		append([]any(nil), filterArgs...),
	)
	if err != nil {
		return nil, err
	}
	statement := taskCatalogStatement("SELECT "+taskCatalogSelectColumns+" FROM catalog", baseWhere, pageWhere) +
		taskCatalogOrderBy(query.Sort) + taskCatalogLimitClause(query.Limit+1)
	pageArgs := append(append([]any(nil), baseArgs...), pageFilterArgs...)
	// dynamic-sql: catalog pagination appends a probe limit to the structurally composed statement.
	rows, err := exec.QueryContext(ctx, statement, pageArgs...)
	if err != nil {
		return nil, fmt.Errorf("store: query task catalog page: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close task catalog page: %w", closeErr))
		}
	}()

	tasks = make([]taskpkg.Summary, 0, query.Limit+1)
	for rows.Next() {
		summary, scanErr := scanTaskCatalogSummary(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tasks = append(tasks, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate task catalog page: %w", err)
	}
	return tasks, nil
}
