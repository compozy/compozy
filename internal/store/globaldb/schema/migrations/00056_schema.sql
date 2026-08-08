-- +goose Up
-- create "gateway_device_sessions" table
CREATE TABLE `gateway_device_sessions` (`id` text NULL, `name` text NOT NULL, `token_hash` text NOT NULL, `actor_kind` text NOT NULL, `pairing_origin` text NOT NULL, `revoke_epoch` integer NOT NULL DEFAULT 0, `created_at` text NOT NULL, `last_seen_at` text NULL, `revoked_at` text NULL, PRIMARY KEY (`id`), CHECK (length(trim(name)) > 0), CHECK (length(token_hash) = 64), CHECK (actor_kind IN ('operator_device', 'cli_profile')), CHECK (length(trim(pairing_origin)) > 0), CHECK (revoke_epoch >= 0));
-- create index "gateway_device_sessions_token_hash" to table: "gateway_device_sessions"
CREATE UNIQUE INDEX `gateway_device_sessions_token_hash` ON `gateway_device_sessions` (`token_hash`);
-- create "gateway_providers" table
CREATE TABLE `gateway_providers` (`name` text NULL, `install_source` text NOT NULL, `digest_confirmed` text NULL, `confirmed_at` text NULL, PRIMARY KEY (`name`), CHECK (length(trim(name)) > 0), CHECK (length(trim(install_source)) > 0));
-- create "gateway_provider_activations" table
CREATE TABLE `gateway_provider_activations` (`provider_name` text NOT NULL, `tier` text NOT NULL, `desired_state` text NOT NULL, `observed_state` text NOT NULL, `generation` integer NOT NULL DEFAULT 0, `last_health_at` text NULL, `last_error` text NULL, PRIMARY KEY (`provider_name`, `tier`), CONSTRAINT `0` FOREIGN KEY (`provider_name`) REFERENCES `gateway_providers` (`name`) ON UPDATE NO ACTION ON DELETE CASCADE, CHECK (tier IN ('private', 'public')), CHECK (desired_state IN ('enabled', 'disabled')), CHECK (observed_state IN ('down', 'establishing', 'up', 'degraded')), CHECK (generation >= 0));
-- create index "gateway_active_provider_per_tier" to table: "gateway_provider_activations"
CREATE UNIQUE INDEX `gateway_active_provider_per_tier` ON `gateway_provider_activations` (`tier`) WHERE desired_state = 'enabled';
-- create "gateway_surface_exposure" table
CREATE TABLE `gateway_surface_exposure` (`surface` text NOT NULL, `tier` text NOT NULL, `desired_state` text NOT NULL, `observed_state` text NOT NULL, `generation` integer NOT NULL DEFAULT 0, `consented_at` text NULL, PRIMARY KEY (`surface`, `tier`), CHECK (surface IN ('operator_ui', 'webhook_ingress')), CHECK (tier IN ('private', 'public')), CHECK (desired_state IN ('enabled', 'disabled')), CHECK (observed_state IN ('off', 'on')), CHECK (generation >= 0), CHECK (surface <> 'operator_ui' OR tier <> 'public' OR desired_state = 'disabled' OR consented_at IS NOT NULL));
