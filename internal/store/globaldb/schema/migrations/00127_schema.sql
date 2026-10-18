-- +goose Up
-- disable the enforcement of foreign-keys constraints
PRAGMA foreign_keys = off;
-- create "new_extension_profile_markers" table
CREATE TABLE `new_extension_profile_markers` (`extension_name` text NOT NULL, `profile_name` text NOT NULL, `created_profile_id` text NOT NULL, `created_at` text NOT NULL, `created_by_extension` integer NULL, PRIMARY KEY (`extension_name`, `profile_name`), CONSTRAINT `0` FOREIGN KEY (`extension_name`) REFERENCES `extensions` (`name`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (trim(profile_name) <> ''), CHECK (trim(created_profile_id) <> ''), CHECK (created_by_extension IN (0, 1)));
-- copy rows from old table "extension_profile_markers" to new temporary table "new_extension_profile_markers"
INSERT INTO `new_extension_profile_markers` (`extension_name`, `profile_name`, `created_profile_id`, `created_at`) SELECT `extension_name`, `profile_name`, `created_profile_id`, `created_at` FROM `extension_profile_markers`;
-- drop "extension_profile_markers" table after copying rows
DROP TABLE `extension_profile_markers`;
-- rename temporary table "new_extension_profile_markers" to "extension_profile_markers"
ALTER TABLE `new_extension_profile_markers` RENAME TO `extension_profile_markers`;
-- enable back the enforcement of foreign-keys constraints
PRAGMA foreign_keys = on;
