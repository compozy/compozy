-- +goose Up
-- create "usage_cost_repair_state" table
CREATE TABLE `usage_cost_repair_state` (`singleton` integer NULL, `pending` integer NOT NULL, PRIMARY KEY (`singleton`), CHECK (singleton = 1), CHECK (pending IN (0, 1)));
INSERT INTO usage_cost_repair_state(singleton, pending)
SELECT 1, 1 WHERE EXISTS(SELECT 1 FROM token_stats) OR EXISTS(SELECT 1 FROM token_usage_daily);
-- Never expose an inflated aggregate while the resumable ledger repair is pending.
UPDATE token_stats SET total_cost = NULL, cost_currency = NULL, cost_status = 'unknown', cost_source = 'none';
UPDATE token_usage_daily SET total_cost = NULL, cost_currency = NULL, cost_status = 'unknown', cost_source = 'none';
