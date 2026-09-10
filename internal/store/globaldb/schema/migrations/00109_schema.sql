-- +goose Up
-- create "attention_acknowledgements" table
CREATE TABLE `attention_acknowledgements` (`profile_id` text NOT NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `occurrence_id` text NOT NULL, `acknowledged_at` text NOT NULL, PRIMARY KEY (`profile_id`, `actor_kind`, `actor_id`, `occurrence_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create "attention_snapshots" table
CREATE TABLE `attention_snapshots` (`id` text NULL, `profile_id` text NOT NULL, `actor_kind` text NOT NULL, `actor_id` text NOT NULL, `population` text NOT NULL, `occurrence_ids` text NOT NULL, `expires_at` text NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE NO ACTION, CHECK (json_valid(occurrence_ids)));
-- create index "attention_snapshots_expiry_idx" to table: "attention_snapshots"
CREATE INDEX `attention_snapshots_expiry_idx` ON `attention_snapshots` (`expires_at`);
