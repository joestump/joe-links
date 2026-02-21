// Governing: SPEC-0001 REQ "Server-Side Sessions", ADR-0003
package auth

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/jmoiron/sqlx"
)

const (
	SessionUserIDKey = "user_id"
	SessionRoleKey   = "role"
)

// NewSessionManager creates an SCS session manager backed by the application DB.
// The driver parameter selects the appropriate store: "mysql", "postgres", or
// "sqlite3" (default).
func NewSessionManager(db *sqlx.DB, driver string, lifetime time.Duration) *scs.SessionManager {
	sm := scs.New()
	switch driver {
	case "mysql":
		sm.Store = mysqlstore.New(db.DB)
	case "postgres":
		sm.Store = postgresstore.New(db.DB)
	default: // sqlite3
		sm.Store = sqlite3store.New(db.DB)
	}
	sm.Lifetime = lifetime
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	return sm
}
