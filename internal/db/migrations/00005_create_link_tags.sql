-- Governing: SPEC-0002 REQ "Link Tags Join Table"
-- +goose Up
CREATE TABLE IF NOT EXISTS link_tags (
    link_id TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (link_id, tag_id)
);

-- +goose Down
DROP TABLE IF EXISTS link_tags;
