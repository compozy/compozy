-- +goose Up
-- add column "pin_position" to table: "sessions"
ALTER TABLE `sessions` ADD COLUMN `pin_position` integer NULL;
-- keep the existing Pinned order: oldest pin first, as pinned_at ordered it
UPDATE `sessions` SET `pin_position` = (SELECT COUNT(1) FROM `sessions` AS `earlier` WHERE `earlier`.`pinned_at` IS NOT NULL AND (`earlier`.`pinned_at` < `sessions`.`pinned_at` OR (`earlier`.`pinned_at` = `sessions`.`pinned_at` AND `earlier`.`id` < `sessions`.`id`))) WHERE `pinned_at` IS NOT NULL;
