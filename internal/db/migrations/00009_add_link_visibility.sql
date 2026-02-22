-- Governing: SPEC-0010 REQ "Visibility Column on Links Table"
-- +goose Up
ALTER TABLE links ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public';

-- +goose Down
ALTER TABLE links DROP COLUMN visibility;
