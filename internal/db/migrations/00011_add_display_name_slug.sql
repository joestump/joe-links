-- Governing: SPEC-0012 REQ "Database Migration for display_name_slug"
-- +goose Up
ALTER TABLE users ADD COLUMN display_name_slug TEXT NOT NULL DEFAULT '';

-- Populate display_name_slug for existing users.
-- Derive slug: lowercase, replace spaces with hyphens, strip non-alphanumeric/hyphen chars.
-- SQLite REPLACE + LOWER handles the basic derivation; Go code handles edge cases going forward.
UPDATE users SET display_name_slug = REPLACE(LOWER(TRIM(display_name)), ' ', '-')
WHERE display_name_slug = '';

CREATE UNIQUE INDEX idx_users_display_name_slug ON users(display_name_slug);

-- +goose Down
DROP INDEX IF EXISTS idx_users_display_name_slug;
ALTER TABLE users DROP COLUMN display_name_slug;
