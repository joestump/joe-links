-- +goose Up
CREATE TABLE IF NOT EXISTS keywords (
    id          TEXT PRIMARY KEY,
    keyword     TEXT NOT NULL UNIQUE,
    url_template TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS keywords;
