-- Governing: SPEC-0010 REQ "Link Shares Table"
-- +goose Up
CREATE TABLE IF NOT EXISTS link_shares (
    link_id    TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shared_by  TEXT NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (link_id, user_id)
);

CREATE INDEX idx_link_shares_link_user ON link_shares(link_id, user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_link_shares_link_user;
DROP TABLE IF EXISTS link_shares;
