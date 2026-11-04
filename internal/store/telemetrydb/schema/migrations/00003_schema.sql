-- +goose Up
-- create "telemetry_identity" table
CREATE TABLE `telemetry_identity` (`singleton` integer NULL, `identity` text NOT NULL, PRIMARY KEY (`singleton`), CHECK (singleton = 1), CHECK (length(identity) > 0));
