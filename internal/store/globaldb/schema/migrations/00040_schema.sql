-- +goose Up
DELETE FROM resource_records WHERE kind IN ('bundle', 'bundle.activation');
DELETE FROM resource_records WHERE owner_kind = 'bundle.activation';
