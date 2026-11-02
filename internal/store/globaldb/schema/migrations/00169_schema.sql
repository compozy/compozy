-- +goose Up
-- create "agent_instance_seen" table
CREATE TABLE `agent_instance_seen` (`instance_id` text NOT NULL, `operator_id` text NOT NULL, `last_seen_at` text NOT NULL, PRIMARY KEY (`instance_id`, `operator_id`), CONSTRAINT `0` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (length(operator_id) > 0));
