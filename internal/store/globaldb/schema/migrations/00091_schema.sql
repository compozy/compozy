-- +goose Up
-- drop index "idx_skill_exposures_skill_name" from table: "skill_exposures"
DROP INDEX `idx_skill_exposures_skill_name`;
