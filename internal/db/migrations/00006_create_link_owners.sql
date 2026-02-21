-- Governing: SPEC-0002 REQ "Link Owners Join Table", ADR-0005
-- +goose Up
CREATE TABLE IF NOT EXISTS link_owners (
    link_id TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_primary INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (link_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS link_owners;
