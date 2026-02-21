-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS links (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    url         TEXT NOT NULL,
    owner_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS links_slug_idx ON links(slug);
CREATE INDEX IF NOT EXISTS links_owner_idx ON links(owner_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS links;
-- +goose StatementEnd
