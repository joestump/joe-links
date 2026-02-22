package migrations

// Governing: SPEC-0012 REQ "Database Migration for display_name_slug"
// This Go migration replaces the SQL version because the slug backfill uses
// database-specific functions:
//   - TRIM(col, chars)  is SQLite-specific
//   - BTRIM(col, chars) is PostgreSQL
//   - TRIM(BOTH chars FROM col) is MySQL / standard SQL
//
// Additionally, the UPDATE … FROM subquery and || concatenation operator are
// not supported in MySQL without sql_mode adjustments.

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddDisplayNameSlug, downAddDisplayNameSlug)
}

func upAddDisplayNameSlug(ctx context.Context, tx *sql.Tx) error {
	stmts := displayNameSlugUpStmts()
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func downAddDisplayNameSlug(ctx context.Context, tx *sql.Tx) error {
	stmts := displayNameSlugDownStmts()
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func displayNameSlugUpStmts() []string {
	switch dialect {
	case "postgres":
		return []string{
			`ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name_slug TEXT NOT NULL DEFAULT ''`,

			// Derive slug from display_name: lowercase, replace spaces with hyphens, strip
			// common punctuation.
			`UPDATE users SET display_name_slug = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
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
WHERE display_name_slug = ''`,

			// Strip leading/trailing hyphens — PostgreSQL uses BTRIM.
			`UPDATE users SET display_name_slug = BTRIM(display_name_slug, '-')
WHERE display_name_slug LIKE '-%' OR display_name_slug LIKE '%-'`,

			// Handle empty slugs (display_name was all special chars).
			`UPDATE users SET display_name_slug = 'user-' || SUBSTR(id, 1, 8)
WHERE display_name_slug = ''`,

			// Deduplicate: for users sharing a derived slug, append -2, -3, etc.
			// The first user in each group keeps the bare slug; subsequent users get a suffix.
			`UPDATE users SET display_name_slug = display_name_slug || '-' || CAST(dup_rank AS TEXT)
FROM (
    SELECT id AS dup_id, ROW_NUMBER() OVER (
        PARTITION BY display_name_slug ORDER BY created_at, id
    ) AS dup_rank
    FROM users
) ranked
WHERE users.id = ranked.dup_id AND ranked.dup_rank > 1`,

			`CREATE UNIQUE INDEX idx_users_display_name_slug ON users (display_name_slug)`,
		}

	case "mysql":
		return []string{
			`ALTER TABLE users ADD COLUMN display_name_slug VARCHAR(255) NOT NULL DEFAULT ''`,

			`UPDATE users SET display_name_slug = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
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
        '\\', ''),
        '--', '-')
WHERE display_name_slug = ''`,

			// MySQL: TRIM(BOTH chars FROM col).
			`UPDATE users SET display_name_slug = TRIM(BOTH '-' FROM display_name_slug)
WHERE display_name_slug LIKE '-%' OR display_name_slug LIKE '%-'`,

			// MySQL: use CONCAT instead of ||.
			`UPDATE users SET display_name_slug = CONCAT('user-', SUBSTR(id, 1, 8))
WHERE display_name_slug = ''`,

			// MySQL UPDATE … JOIN syntax for the deduplication window.
			`UPDATE users u
JOIN (
    SELECT id AS dup_id, ROW_NUMBER() OVER (
        PARTITION BY display_name_slug ORDER BY created_at, id
    ) AS dup_rank
    FROM users
) ranked ON u.id = ranked.dup_id AND ranked.dup_rank > 1
SET u.display_name_slug = CONCAT(u.display_name_slug, '-', CAST(ranked.dup_rank AS CHAR))`,

			`CREATE UNIQUE INDEX idx_users_display_name_slug ON users (display_name_slug)`,
		}

	default: // sqlite3
		return []string{
			`ALTER TABLE users ADD COLUMN display_name_slug TEXT NOT NULL DEFAULT ''`,

			`UPDATE users SET display_name_slug = REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
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
WHERE display_name_slug = ''`,

			// SQLite 3.39+: TRIM(col, chars) strips specified chars.
			`UPDATE users SET display_name_slug = TRIM(display_name_slug, '-')
WHERE display_name_slug LIKE '-%' OR display_name_slug LIKE '%-'`,

			`UPDATE users SET display_name_slug = 'user-' || SUBSTR(id, 1, 8)
WHERE display_name_slug = ''`,

			`UPDATE users SET display_name_slug = display_name_slug || '-' || CAST(dup_rank AS TEXT)
FROM (
    SELECT id AS dup_id, ROW_NUMBER() OVER (
        PARTITION BY display_name_slug ORDER BY created_at, id
    ) AS dup_rank
    FROM users
) ranked
WHERE users.id = ranked.dup_id AND ranked.dup_rank > 1`,

			`CREATE UNIQUE INDEX idx_users_display_name_slug ON users (display_name_slug)`,
		}
	}
}

func displayNameSlugDownStmts() []string {
	switch dialect {
	case "mysql":
		return []string{
			`ALTER TABLE users DROP INDEX idx_users_display_name_slug`,
			`ALTER TABLE users DROP COLUMN display_name_slug`,
		}
	default: // sqlite3, postgres
		return []string{
			`DROP INDEX IF EXISTS idx_users_display_name_slug`,
			`ALTER TABLE users DROP COLUMN display_name_slug`,
		}
	}
}
