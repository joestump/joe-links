-- Governing: SPEC-0002 REQ "Links Table"
-- +goose Up
CREATE TABLE IF NOT EXISTS links (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL,
    url TEXT NOT NULL,
    title TEXT,
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_links_slug ON links(slug);

-- +goose Down
DROP TABLE IF EXISTS links;
