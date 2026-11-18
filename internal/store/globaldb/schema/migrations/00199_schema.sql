-- +goose Up
-- create "usage_cost_repair_state" table
CREATE TABLE `usage_cost_repair_state` (`singleton` integer NULL, `pending` integer NOT NULL, PRIMARY KEY (`singleton`), CHECK (singleton = 1), CHECK (pending IN (0, 1)));
INSERT INTO usage_cost_repair_state(singleton, pending)
SELECT 1, 1 WHERE EXISTS(SELECT 1 FROM token_stats WHERE cost_source = 'agent_reported')
    OR EXISTS(SELECT 1 FROM token_usage_daily WHERE cost_source = 'agent_reported');
-- The migration owner captures the agent-reported targets before invalidating and
-- repairing them in one transaction. Other cost provenance remains untouched.
