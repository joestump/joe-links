// Governing: SPEC-0001 REQ "Database Schema Migrations", ADR-0002
package db

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

//go:embed migrations
var Migrations embed.FS

// Migrate runs all pending goose migrations from the embedded migration files.
// It must be called before the HTTP server starts accepting requests.
func Migrate(db *sqlx.DB, driver string) error {
	gooseDriver, err := gooseDialect(driver)
	if err != nil {
		return err
	}

	if err := goose.SetDialect(gooseDriver); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	sub, err := fs.Sub(Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("sub migrations fs: %w", err)
	}

	goose.SetBaseFS(sub)
	if err := goose.Up(db.DB, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	goose.SetBaseFS(nil)

	return nil
}

func gooseDialect(driver string) (string, error) {
	switch driver {
	case "sqlite3":
		return "sqlite3", nil
	case "mysql":
		return "mysql", nil
	case "postgres":
		return "postgres", nil
	default:
		return "", fmt.Errorf("unknown driver for goose dialect: %q", driver)
	}
}
