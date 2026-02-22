// Package migrations contains dialect-aware Go database migrations that cannot
// be expressed as a single cross-database SQL statement.
package migrations

// dialect is set by the parent db package before migrations are applied.
var dialect string

// SetDialect configures the SQL dialect for Go migrations.
// Must be called before goose.Up. Valid values: "sqlite3", "postgres", "mysql".
func SetDialect(d string) {
	dialect = d
}
