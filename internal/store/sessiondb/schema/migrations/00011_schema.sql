-- +goose Up
-- add column "reported_cost_amount" to table: "token_usage"
ALTER TABLE `token_usage` ADD COLUMN `reported_cost_amount` real NULL;
UPDATE token_usage SET reported_cost_amount = cost_amount;
CREATE TEMP TABLE usage_cost_baselines AS
WITH reports AS (
    SELECT turn_id, sequence, json_extract(content, '$.usage.CostAmount') AS cost_amount,
           trim(json_extract(content, '$.usage.CostCurrency')) AS cost_currency
    FROM events
    WHERE type IN ('usage', 'done') AND json_valid(content)
      AND json_type(content, '$.usage.CostAmount') IN ('real', 'integer')
      AND json_extract(content, '$.usage.CostAmount') >= 0
      AND json_extract(content, '$.usage.CostAmount') <= 1.7976931348623157e308
      AND length(trim(json_extract(content, '$.usage.CostCurrency'))) > 0
), ranked AS (
    SELECT *, MIN(sequence) OVER (PARTITION BY turn_id, cost_currency) AS first_sequence,
           ROW_NUMBER() OVER (PARTITION BY turn_id, cost_currency ORDER BY sequence DESC) AS latest
    FROM reports
), ordered AS (
    SELECT turn_id, cost_currency, cost_amount,
           LAG(cost_amount, 1, 0) OVER (PARTITION BY cost_currency ORDER BY first_sequence) AS previous
    FROM ranked WHERE latest = 1
)
SELECT *, CASE WHEN cost_amount < previous THEN cost_amount ELSE cost_amount - previous END AS turn_cost
FROM ordered;
CREATE UNIQUE INDEX usage_cost_baselines_turn_currency ON usage_cost_baselines(turn_id, cost_currency);
UPDATE token_usage SET cost_amount = (
    SELECT turn_cost FROM usage_cost_baselines WHERE usage_cost_baselines.turn_id = token_usage.turn_id
      AND usage_cost_baselines.cost_currency = trim(token_usage.cost_currency)
), reported_cost_amount = COALESCE((
    SELECT cost_amount FROM usage_cost_baselines WHERE usage_cost_baselines.turn_id = token_usage.turn_id
      AND usage_cost_baselines.cost_currency = trim(token_usage.cost_currency)
), reported_cost_amount) WHERE reported_cost_amount IS NOT NULL;
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
