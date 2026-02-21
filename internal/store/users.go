// Governing: SPEC-0001 REQ "Local User Records", ADR-0003
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type User struct {
	ID          string    `db:"id"`
	Provider    string    `db:"provider"`
	Subject     string    `db:"subject"`
	Email       string    `db:"email"`
	DisplayName string    `db:"display_name"`
	Role        string    `db:"role"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

type UserStore struct {
	db *sqlx.DB
}

func NewUserStore(db *sqlx.DB) *UserStore {
	return &UserStore{db: db}
}

// Upsert creates or updates a user record on OIDC login.
// adminEmail: if non-empty and matches email on INSERT, role is set to "admin".
//
// TODO: The ON CONFLICT ... DO UPDATE syntax works in SQLite and PostgreSQL but NOT MySQL.
// MySQL needs a separate implementation using INSERT ... ON DUPLICATE KEY UPDATE.
//
// TODO: Placeholder `?` works for SQLite and MySQL but PostgreSQL needs `$1`, `$2`, etc.
// In production, use a DB-agnostic query builder or separate query files per driver.
func (s *UserStore) Upsert(ctx context.Context, provider, subject, email, displayName, adminEmail string) (*User, error) {
	role := "user"
	if adminEmail != "" && email == adminEmail {
		role = "admin"
	}
	id := uuid.New().String()
	now := time.Now().UTC()

	// Try INSERT first; if the (provider, subject) pair already exists, UPDATE instead.
	// The ON CONFLICT ... DO UPDATE syntax preserves the existing role for returning users
	// because we don't include role in the UPDATE clause. For new users, role is set above.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, provider, subject, email, display_name, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (provider, subject) DO UPDATE SET
			email = excluded.email,
			display_name = excluded.display_name,
			updated_at = excluded.updated_at
	`, id, provider, subject, email, displayName, role, now, now)
	if err != nil {
		return nil, err
	}

	var u User
	err = s.db.GetContext(ctx, &u, `SELECT * FROM users WHERE provider = ? AND subject = ?`, provider, subject)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByEmail returns the user matching email, or ErrNotFound.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.db.GetContext(ctx, &u, `SELECT * FROM users WHERE email = ?`, email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.db.GetContext(ctx, &u, `SELECT * FROM users WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
