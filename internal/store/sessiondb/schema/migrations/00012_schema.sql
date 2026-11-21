-- +goose Up
-- add column "part_count" to table: "transcript_entries"
ALTER TABLE `transcript_entries` ADD COLUMN `part_count` integer NOT NULL DEFAULT 0;
-- create "transcript_entry_parts" table
CREATE TABLE `transcript_entry_parts` (`entry_key` text NOT NULL, `part_index` integer NOT NULL, `field` integer NOT NULL, `sequence` integer NOT NULL, `content` text NOT NULL, PRIMARY KEY (`entry_key`, `part_index`, `field`, `sequence`), CONSTRAINT `0` FOREIGN KEY (`entry_key`) REFERENCES `transcript_entries` (`entry_key`) ON UPDATE NO ACTION ON DELETE CASCADE) WITHOUT ROWID;
