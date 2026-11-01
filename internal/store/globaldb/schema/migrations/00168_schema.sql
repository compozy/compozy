-- +goose Up
-- create "agent_instance_rotation_wait" table
CREATE TABLE `agent_instance_rotation_wait` (`instance_id` text NULL, `main_session_id` text NOT NULL, `started_at` text NOT NULL, PRIMARY KEY (`instance_id`), CONSTRAINT `0` FOREIGN KEY (`main_session_id`) REFERENCES `sessions` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT `1` FOREIGN KEY (`instance_id`) REFERENCES `agent_instances` (`id`) ON UPDATE NO ACTION ON DELETE CASCADE);
