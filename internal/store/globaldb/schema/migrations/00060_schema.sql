-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_loop_run_events" table
CREATE TABLE `new_loop_run_events` (`watch_seq` integer NOT NULL PRIMARY KEY AUTOINCREMENT, `id` text NOT NULL, `loop_run_id` text NOT NULL, `workspace_id` text NOT NULL, `seq` integer NOT NULL, `kind` text NOT NULL, `payload_json` text NOT NULL, `at` timestamp NOT NULL, `delivery_key` text NULL);
-- copy rows from old table "loop_run_events" to new temporary table "new_loop_run_events"
INSERT INTO `new_loop_run_events` (`watch_seq`, `id`, `loop_run_id`, `workspace_id`, `seq`, `kind`, `payload_json`, `at`, `delivery_key`) SELECT `rowid`, `id`, `loop_run_id`, `workspace_id`, `seq`, `kind`, `payload_json`, `at`, `delivery_key` FROM `loop_run_events` ORDER BY `rowid`;
-- drop "loop_run_events" table after copying rows
DROP TABLE `loop_run_events`;
-- rename temporary table "new_loop_run_events" to "loop_run_events"
ALTER TABLE `new_loop_run_events` RENAME TO `loop_run_events`;
-- create index "loop_run_events_id" to table: "loop_run_events"
CREATE UNIQUE INDEX `loop_run_events_id` ON `loop_run_events` (`id`);
-- create index "idx_loop_run_events_run_seq" to table: "loop_run_events"
CREATE INDEX `idx_loop_run_events_run_seq` ON `loop_run_events` (`loop_run_id`, `seq`);
-- create index "idx_loop_run_events_watch_stream" to table: "loop_run_events"
CREATE INDEX `idx_loop_run_events_watch_stream` ON `loop_run_events` (`workspace_id`, `watch_seq`);
-- create index "uq_loop_run_events_delivery" to table: "loop_run_events"
CREATE UNIQUE INDEX `uq_loop_run_events_delivery` ON `loop_run_events` (`loop_run_id`, `delivery_key`) WHERE delivery_key IS NOT NULL;
-- Hard-cut parked loop-event cursors to the new durable namespace and arm
-- them at the migration boundary. Other stream cursors and subscriptions stay intact.
UPDATE loop_generation_outputs AS lgo
SET output_ref = json_set(lgo.output_ref, '$.cursor_version', 1)
WHERE json_valid(lgo.output_ref)
  AND json_extract(lgo.output_ref, '$.kind') = 'watch_events_pending';
UPDATE loop_generation_outputs AS lgo
SET output_ref = json_set(
	lgo.output_ref,
	'$.cursors.loop_run_events',
	COALESCE((
		SELECT MAX(lre.watch_seq)
		FROM loop_run_events lre
		JOIN loop_runs lr
		  ON lr.id = lre.loop_run_id
		 AND lr.workspace_id = lre.workspace_id
		WHERE lr.workspace_id = (
			SELECT parked.workspace_id
			FROM loop_runs parked
			WHERE parked.id = lgo.loop_run_id
		)
	), 0)
)
WHERE json_valid(lgo.output_ref)
  AND json_extract(lgo.output_ref, '$.kind') = 'watch_events_pending'
  AND json_type(lgo.output_ref, '$.cursors.loop_run_events') IS NOT NULL;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
