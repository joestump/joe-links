-- Governing: SPEC-0002 REQ "Tags Table"
-- +goose Up
CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_slug ON tags(slug);

-- +goose Down
DROP TABLE IF EXISTS tags;
