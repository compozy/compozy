-- +goose Up
-- Marketplace manifests are a greenfield v2 hard cut. Cached v1 payloads are
-- untrusted because their shape is no longer accepted by the catalog reader.
DELETE FROM marketplace_catalog_entries;
DELETE FROM marketplace_catalog_state;
