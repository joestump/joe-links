-- Governing: SPEC-0012 REQ "Database Migration for display_name_slug"
-- +goose Up
ALTER TABLE users ADD COLUMN display_name_slug TEXT NOT NULL DEFAULT '';

-- Populate display_name_slug for existing users.
-- Derive slug: lowercase, replace spaces with hyphens, strip common non-alphanumeric chars.
-- This handles the most common special characters; the Go DeriveDisplayNameSlug function
-- handles all edge cases for future logins via Upsert.
UPDATE users SET display_name_slug = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
    REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
        REPLACE(LOWER(TRIM(display_name)),
        ' ', '-'),
        '''', ''),
        '"', ''),
        '.', ''),
        ',', ''),
        '!', ''),
        '?', ''),
        '(', ''),
        ')', ''),
        '@', ''),
        '#', ''),
        '&', ''),
        '+', ''),
        '/', ''),
        '\', ''),
        '--', '-')
WHERE display_name_slug = '';

-- Strip leading/trailing hyphens (TRIM with characters is SQLite 3.39+).
UPDATE users SET display_name_slug = TRIM(display_name_slug, '-')
WHERE display_name_slug LIKE '-%' OR display_name_slug LIKE '%-';

-- Handle empty slugs (e.g. display_name was all special chars).
UPDATE users SET display_name_slug = 'user-' || SUBSTR(id, 1, 8)
WHERE display_name_slug = '';

-- Handle duplicate slugs: for users that share a derived slug, append -2, -3, etc.
-- Uses a window function (ROW_NUMBER) to assign unique suffixes per duplicate group.
-- Only the first user in each group keeps the bare slug; subsequent users get a suffix.
UPDATE users SET display_name_slug = display_name_slug || '-' || CAST(dup_rank AS TEXT)
FROM (
    SELECT id AS dup_id, ROW_NUMBER() OVER (
        PARTITION BY display_name_slug ORDER BY created_at, id
    ) AS dup_rank
    FROM users
) ranked
WHERE users.id = ranked.dup_id AND ranked.dup_rank > 1;

CREATE UNIQUE INDEX idx_users_display_name_slug ON users(display_name_slug);

-- +goose Down
DROP INDEX IF EXISTS idx_users_display_name_slug;
ALTER TABLE users DROP COLUMN display_name_slug;
