-- +goose Up
-- add column "reported_cost_amount" to table: "token_usage"
ALTER TABLE `token_usage` ADD COLUMN `reported_cost_amount` real NULL;
UPDATE token_usage SET reported_cost_amount = cost_amount;
CREATE TEMP TABLE usage_cost_baselines AS
WITH ordered AS (
    SELECT turn_id, cost_currency, cost_amount,
           LAG(cost_amount, 1, 0) OVER (
               PARTITION BY trim(cost_currency) ORDER BY
               COALESCE((SELECT MIN(sequence) FROM events WHERE events.turn_id = token_usage.turn_id), 9223372036854775807),
               timestamp, turn_id
           ) AS previous
    FROM token_usage
    WHERE cost_amount >= 0 AND cost_amount <= 1.7976931348623157e308
      AND length(trim(cost_currency)) > 0
)
SELECT *, CASE WHEN cost_amount < previous THEN cost_amount ELSE cost_amount - previous END AS turn_cost
FROM ordered;
UPDATE token_usage SET cost_amount = (
    SELECT turn_cost FROM usage_cost_baselines WHERE usage_cost_baselines.turn_id = token_usage.turn_id
) WHERE reported_cost_amount IS NOT NULL;
-- Canonical payloads retain their original monetary evidence. All other fields,
-- event IDs, ordering and archive flags remain unchanged.
UPDATE events SET content = json_set(content,
    '$.usage.ReportedCostAmount', json_extract(content, '$.usage.CostAmount'),
    '$.usage.CostAmount', (
        SELECT CASE WHEN json_extract(events.content, '$.usage.CostAmount') < previous
                    THEN json_extract(events.content, '$.usage.CostAmount')
                    ELSE json_extract(events.content, '$.usage.CostAmount') - previous END
        FROM usage_cost_baselines WHERE usage_cost_baselines.turn_id = events.turn_id
          AND trim(cost_currency) = trim(json_extract(events.content, '$.usage.CostCurrency'))
    )
) WHERE json_valid(content) AND json_type(content, '$.usage.CostAmount') IN ('real', 'integer');
DROP TABLE usage_cost_baselines;

-- Rewind replay is a derived snapshot; refresh it through its existing owner.
UPDATE conversation_rewind_state SET baseline_stale = 1
WHERE EXISTS (SELECT 1 FROM events WHERE sequence <= covered_through_sequence
  AND json_valid(content) AND json_type(content, '$.usage.ReportedCostAmount') IN ('real', 'integer'));
