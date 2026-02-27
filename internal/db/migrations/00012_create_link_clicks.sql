-- Governing: SPEC-0016 REQ "Click Data Schema", ADR-0016
-- +goose Up
CREATE TABLE IF NOT EXISTS link_clicks (
    id TEXT NOT NULL PRIMARY KEY,
    link_id TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    ip_hash TEXT NOT NULL,
    user_agent TEXT,
    referrer TEXT,
    clicked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_link_clicks_link_id_clicked_at ON link_clicks(link_id, clicked_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_link_clicks_link_id_clicked_at;
DROP TABLE IF EXISTS link_clicks;
