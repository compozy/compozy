package sessiondb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

const usageCostMigrationVersion = 11

// usageCostMigrationReconciliation completes migration 00011 on a session
// database that already carries token_usage.reported_cost_amount without
// version 11 recorded. The immutable migration starts with an unguarded ADD
// COLUMN and would fail there, so the same transformation runs here in an
// idempotent form: only usage rows without a retained report and only events
// without ReportedCostAmount change, and turns whose canonical events are
// already normalized keep their stored cost. Only the column definition that
// migration publishes is recognized; any other column of that name falls
// through to the migration, which fails on it explicitly.
func usageCostMigrationReconciliation() *store.MigrationReconciliation {
	return &store.MigrationReconciliation{
		Name:     "usage_cost_reported_amount",
		Version:  usageCostMigrationVersion,
		Detect:   detectUsageCostColumn,
		Complete: completeUsageCostMigration,
	}
}

func detectUsageCostColumn(ctx context.Context, db store.MigrationQuerier) (bool, error) {
	var count int
	// dynamic-sql: pragma_table_info inspects physical schema outside sqlc's query model.
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM pragma_table_info('token_usage') WHERE name = 'reported_cost_amount'
AND lower(type) = 'real' AND "notnull" = 0 AND dflt_value IS NULL AND pk = 0`,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("store: inspect token_usage reported cost column: %w", err)
	}
	return count > 0, nil
}

// usageCostReconciliationStatements mirrors 00011_schema.sql after its ADD
// COLUMN. A report is the retained ReportedCostAmount when present, otherwise
// the legacy cumulative CostAmount, which is the baseline the runtime reads.
// A usage row without a report takes it from its canonical turn history first:
// a normalized turn's cost_amount is already the per-turn cost, not the report.
var usageCostReconciliationStatements = []string{
	`DROP TABLE IF EXISTS temp.usage_cost_baselines`,
	`DROP TABLE IF EXISTS temp.usage_cost_normalized_turns`,
	`CREATE TEMP TABLE usage_cost_normalized_turns AS
SELECT DISTINCT turn_id FROM events
WHERE json_valid(content) AND json_type(content, '$.usage.ReportedCostAmount') IN ('real', 'integer')`,
	`CREATE INDEX usage_cost_normalized_turns_turn ON usage_cost_normalized_turns(turn_id)`,
	`UPDATE token_usage SET reported_cost_amount = COALESCE((
    SELECT json_extract(content, '$.usage.ReportedCostAmount') FROM events
    WHERE events.turn_id = token_usage.turn_id AND type IN ('usage', 'done') AND json_valid(content)
      AND json_type(content, '$.usage.ReportedCostAmount') IN ('real', 'integer')
      AND trim(json_extract(content, '$.usage.CostCurrency')) = trim(token_usage.cost_currency)
    ORDER BY sequence DESC LIMIT 1
), cost_amount)
WHERE reported_cost_amount IS NULL AND cost_amount IS NOT NULL`,
	`CREATE TEMP TABLE usage_cost_baselines AS
WITH reports AS (
    SELECT turn_id, sequence,
           CASE WHEN json_type(content, '$.usage.ReportedCostAmount') IN ('real', 'integer')
                THEN json_extract(content, '$.usage.ReportedCostAmount')
                ELSE json_extract(content, '$.usage.CostAmount') END AS cost_amount,
           trim(json_extract(content, '$.usage.CostCurrency')) AS cost_currency
    FROM events
    WHERE type IN ('usage', 'done') AND json_valid(content)
      AND (json_type(content, '$.usage.ReportedCostAmount') IN ('real', 'integer')
           OR json_type(content, '$.usage.CostAmount') IN ('real', 'integer'))
      AND length(trim(json_extract(content, '$.usage.CostCurrency'))) > 0
), valid AS (
    SELECT * FROM reports WHERE cost_amount >= 0 AND cost_amount <= 1.7976931348623157e308
), ranked AS (
    SELECT *, MIN(sequence) OVER (PARTITION BY turn_id, cost_currency) AS first_sequence,
           ROW_NUMBER() OVER (PARTITION BY turn_id, cost_currency ORDER BY sequence DESC) AS latest
    FROM valid
), ordered AS (
    SELECT turn_id, cost_currency, cost_amount,
           LAG(cost_amount, 1, 0) OVER (PARTITION BY cost_currency ORDER BY first_sequence) AS previous
    FROM ranked WHERE latest = 1
)
SELECT *, CASE WHEN cost_amount < previous THEN 0 ELSE cost_amount - previous END AS turn_cost
FROM ordered`,
	`CREATE UNIQUE INDEX usage_cost_baselines_turn_currency
ON usage_cost_baselines(turn_id, cost_currency)`,
	`UPDATE token_usage SET cost_amount = (
    SELECT turn_cost FROM usage_cost_baselines WHERE usage_cost_baselines.turn_id = token_usage.turn_id
      AND usage_cost_baselines.cost_currency = trim(token_usage.cost_currency)
), reported_cost_amount = COALESCE((
    SELECT cost_amount FROM usage_cost_baselines WHERE usage_cost_baselines.turn_id = token_usage.turn_id
      AND usage_cost_baselines.cost_currency = trim(token_usage.cost_currency)
), reported_cost_amount)
WHERE reported_cost_amount IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM usage_cost_normalized_turns WHERE usage_cost_normalized_turns.turn_id = token_usage.turn_id
)`,
	`UPDATE events SET content = json_set(content,
    '$.usage.ReportedCostAmount', json_extract(content, '$.usage.CostAmount'),
    '$.usage.CostAmount', (
        SELECT CASE WHEN json_extract(events.content, '$.usage.CostAmount') < previous
                    THEN 0
                    ELSE json_extract(events.content, '$.usage.CostAmount') - previous END
        FROM usage_cost_baselines WHERE usage_cost_baselines.turn_id = events.turn_id
          AND trim(cost_currency) = trim(json_extract(events.content, '$.usage.CostCurrency'))
    )
) WHERE json_valid(content) AND json_type(content, '$.usage.CostAmount') IN ('real', 'integer')
  AND json_type(content, '$.usage.ReportedCostAmount') IS NULL`,
	`DROP TABLE usage_cost_baselines`,
	`DROP TABLE usage_cost_normalized_turns`,
	`UPDATE conversation_rewind_state SET baseline_stale = 1
WHERE EXISTS (SELECT 1 FROM events WHERE sequence <= covered_through_sequence
  AND json_valid(content) AND json_type(content, '$.usage.ReportedCostAmount') IN ('real', 'integer'))`,
}

func completeUsageCostMigration(ctx context.Context, tx *store.WriteTx) error {
	for index, statement := range usageCostReconciliationStatements {
		// dynamic-sql: temp tables and JSON rewrites mirror migration 00011 and cannot be expressed in sqlc.
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("store: complete session usage cost migration statement %d: %w", index+1, err)
		}
	}
	return nil
}
