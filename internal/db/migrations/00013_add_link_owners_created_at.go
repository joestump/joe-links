package migrations

// Governing: SPEC-0002 REQ "Multi-Ownership via link_owners", ADR-0005
// Adds the spec-mandated created_at column to link_owners.
//
// This is a Go migration because SQLite does not permit a non-constant default
// (CURRENT_TIMESTAMP) on ALTER TABLE ADD COLUMN, while PostgreSQL and MySQL do.
// On SQLite the column is added nullable and existing rows are backfilled; the
// application always supplies created_at explicitly on insert.

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddLinkOwnersCreatedAt, downAddLinkOwnersCreatedAt)
}

func upAddLinkOwnersCreatedAt(ctx context.Context, tx *sql.Tx) error {
	for _, stmt := range linkOwnersCreatedAtUpStmts() {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func downAddLinkOwnersCreatedAt(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `ALTER TABLE link_owners DROP COLUMN created_at`)
	return err
}

func linkOwnersCreatedAtUpStmts() []string {
	switch dialect {
	case "postgres", "mysql":
		return []string{
			`ALTER TABLE link_owners ADD COLUMN created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`,
		}
	default: // sqlite3 — constant default only on ADD COLUMN, so add nullable then backfill
		return []string{
			`ALTER TABLE link_owners ADD COLUMN created_at TIMESTAMP`,
			`UPDATE link_owners SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL`,
		}
	}
}
