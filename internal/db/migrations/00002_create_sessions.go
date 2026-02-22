package migrations

// Governing: SPEC-0001 REQ "Server-Side Sessions", ADR-0003
// This Go migration replaces the SQL version because the sessions table schema
// differs by database driver (BLOB/REAL for SQLite, BYTEA/TIMESTAMPTZ for
// PostgreSQL, BLOB/TIMESTAMP(6) for MySQL), matching what each scs store adapter
// expects.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateSessions, downCreateSessions)
}

func upCreateSessions(ctx context.Context, tx *sql.Tx) error {
	var ddl string
	switch dialect {
	case "postgres":
		ddl = `CREATE TABLE IF NOT EXISTS sessions (
    token  TEXT PRIMARY KEY,
    data   BYTEA NOT NULL,
    expiry TIMESTAMPTZ NOT NULL
)`
	case "mysql":
		ddl = `CREATE TABLE IF NOT EXISTS sessions (
    token  VARCHAR(43) PRIMARY KEY,
    data   BLOB NOT NULL,
    expiry TIMESTAMP(6) NOT NULL
)`
	default: // sqlite3
		ddl = `CREATE TABLE IF NOT EXISTS sessions (
    token  TEXT PRIMARY KEY,
    data   BLOB NOT NULL,
    expiry REAL NOT NULL
)`
	}
	if _, err := tx.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create sessions table: %w", err)
	}
	_, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions (expiry)`)
	return err
}

func downCreateSessions(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS sessions`)
	return err
}
