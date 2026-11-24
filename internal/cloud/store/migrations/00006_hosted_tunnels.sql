-- +goose Up
ALTER TABLE tunnels ADD COLUMN hosted_origin text;

-- +goose Down
ALTER TABLE tunnels DROP COLUMN hosted_origin;
