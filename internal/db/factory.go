// Governing: SPEC-0001 REQ "Pluggable Database Backend", ADR-0002
package db

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// New opens a database connection for the given driver and DSN.
// Supported drivers: sqlite3, mysql, postgres.
func New(driver, dsn string) (*sqlx.DB, error) {
	switch driver {
	case "sqlite3":
		// modernc/sqlite uses "sqlite" as the driver name (CGO-free)
		db, err := sqlx.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("open sqlite: %w", err)
		}
		// SQLite WAL mode for better concurrency
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return nil, fmt.Errorf("enable WAL: %w", err)
		}
		return db, nil
	case "mysql":
		db, err := sqlx.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("open mysql: %w", err)
		}
		return db, nil
	case "postgres":
		db, err := sqlx.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		return db, nil
	default:
		return nil, fmt.Errorf("unsupported DB driver %q: must be sqlite3, mysql, or postgres", driver)
	}
}
