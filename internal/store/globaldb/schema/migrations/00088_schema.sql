-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- add column "write_target" to table: "config_apply_records"
ALTER TABLE `config_apply_records` ADD COLUMN `write_target` text NOT NULL DEFAULT '';
-- add column "write_path" to table: "config_apply_records"
ALTER TABLE `config_apply_records` ADD COLUMN `write_path` text NOT NULL DEFAULT '';
-- create "new_extension_profile_enablement" table
CREATE TABLE `new_extension_profile_enablement` (`extension_name` text NOT NULL, `profile_id` text NOT NULL, `enabled` integer NOT NULL, PRIMARY KEY (`extension_name`, `profile_id`), CONSTRAINT `0` FOREIGN KEY (`profile_id`) REFERENCES `profiles` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (enabled IN (0, 1)));
-- copy rows from old table "extension_profile_enablement" to new temporary table "new_extension_profile_enablement"
INSERT INTO `new_extension_profile_enablement` (`extension_name`, `profile_id`, `enabled`) SELECT `extension_name`, `profile_id`, `enabled` FROM `extension_profile_enablement`;
-- drop trigger "extension_dev_links_profile_enablement_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `extension_dev_links_profile_enablement_delete`;
-- drop trigger "extension_dev_links_profile_enablement_insert" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `extension_dev_links_profile_enablement_insert`;
-- drop trigger "extensions_profile_enablement_delete" before rebuilding a referenced table
DROP TRIGGER IF EXISTS `extensions_profile_enablement_delete`;
-- drop "extension_profile_enablement" table after copying rows
DROP TABLE `extension_profile_enablement`;
-- rename temporary table "new_extension_profile_enablement" to "extension_profile_enablement"
ALTER TABLE `new_extension_profile_enablement` RENAME TO `extension_profile_enablement`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
-- recreate trigger "extension_dev_links_profile_enablement_delete" after rebuilding table "extension_dev_links"
-- +goose StatementBegin
CREATE TRIGGER extension_dev_links_profile_enablement_delete
AFTER DELETE ON extension_dev_links
WHEN NOT EXISTS (
		SELECT 1 FROM extensions WHERE name = OLD.extension_name
	)
	AND NOT EXISTS (
		SELECT 1 FROM extension_dev_links WHERE extension_name = OLD.extension_name
	)
BEGIN
	DELETE FROM extension_profile_enablement WHERE extension_name = OLD.extension_name;
END;
-- +goose StatementEnd
-- recreate trigger "extension_dev_links_profile_enablement_insert" after rebuilding table "extension_profile_enablement"
-- +goose StatementBegin
CREATE TRIGGER extension_dev_links_profile_enablement_insert
BEFORE INSERT ON extension_profile_enablement
WHEN NOT EXISTS (
		SELECT 1 FROM extensions WHERE name = NEW.extension_name
	)
	AND NOT EXISTS (
		SELECT 1 FROM extension_dev_links WHERE extension_name = NEW.extension_name
	)
BEGIN
	SELECT RAISE(ABORT, 'extension_not_found');
END;
-- +goose StatementEnd
-- recreate trigger "extensions_profile_enablement_delete" after rebuilding table "extensions"
-- +goose StatementBegin
CREATE TRIGGER extensions_profile_enablement_delete
AFTER DELETE ON extensions
WHEN NOT EXISTS (
	SELECT 1 FROM extension_dev_links WHERE extension_name = OLD.name
)
BEGIN
	DELETE FROM extension_profile_enablement WHERE extension_name = OLD.name;
END;
-- +goose StatementEnd

