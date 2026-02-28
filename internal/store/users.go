// Governing: SPEC-0001 REQ "Local User Records", ADR-0003
package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type User struct {
	ID              string    `db:"id"`
	Provider        string    `db:"provider"`
	Subject         string    `db:"subject"`
	Email           string    `db:"email"`
	DisplayName     string    `db:"display_name"`
	DisplayNameSlug string    `db:"display_name_slug"`
	Role            string    `db:"role"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

var (
	reWhitespace      = regexp.MustCompile(`\s+`)
	reNonSlugChar     = regexp.MustCompile(`[^a-z0-9-]`)
	reConsecutiveHyph = regexp.MustCompile(`-{2,}`)
)

// DeriveDisplayNameSlug converts a display name into a URL-safe slug.
// Governing: SPEC-0012 REQ "Display Name Slug Derivation and Lookup"
func DeriveDisplayNameSlug(displayName string) string {
	s := strings.ToLower(displayName)
	s = reWhitespace.ReplaceAllString(s, "-")
	s = reNonSlugChar.ReplaceAllString(s, "")
	s = reConsecutiveHyph.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

type UserStore struct {
	db *sqlx.DB
}

func NewUserStore(db *sqlx.DB) *UserStore {
	return &UserStore{db: db}
}

// q rebinds ? placeholders to the driver's native format ($1,$2,... for PostgreSQL).
func (s *UserStore) q(query string) string { return s.db.Rebind(query) }

// GetByDisplayNameSlug returns the user matching the given display_name_slug, or ErrNotFound.
// Governing: SPEC-0012 REQ "Display Name Slug Derivation and Lookup"
func (s *UserStore) GetByDisplayNameSlug(ctx context.Context, slug string) (*User, error) {
	var u User
	err := s.db.GetContext(ctx, &u, s.q(`SELECT * FROM users WHERE display_name_slug = ?`), slug)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// resolveUniqueSlug derives a slug from displayName and appends a numeric suffix if needed.
// Governing: SPEC-0012 REQ "Display Name Slug Derivation and Lookup"
func (s *UserStore) resolveUniqueSlug(ctx context.Context, displayName, excludeUserID string) (string, error) {
	base := DeriveDisplayNameSlug(displayName)
	if base == "" {
		base = "user"
	}
	candidate := base
	suffix := 2
	for {
		var count int
		err := s.db.GetContext(ctx, &count,
			s.q(`SELECT COUNT(*) FROM users WHERE display_name_slug = ? AND id != ?`), candidate, excludeUserID)
		if err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, suffix)
		suffix++
	}
}

// Upsert creates or updates a user record on OIDC login.
// role is applied on INSERT (new user). For existing users the role column is
// intentionally not updated here — callers promote via UpdateRole after Upsert
// so that manual role changes made through the admin UI are preserved across logins.
// Governing: SPEC-0012 REQ "Display Name Slug Derivation and Lookup", ADR-0002
func (s *UserStore) Upsert(ctx context.Context, provider, subject, email, displayName, role string) (*User, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	// Look up existing user to get their ID for slug uniqueness check.
	var existingID string
	var existing User
	err := s.db.GetContext(ctx, &existing, s.q(`SELECT * FROM users WHERE provider = ? AND subject = ?`), provider, subject)
	switch {
	case err == nil:
		existingID = existing.ID
	case err == sql.ErrNoRows:
		// New user — existingID stays empty.
	default:
		return nil, fmt.Errorf("lookup existing user: %w", err)
	}

	// Derive a unique display_name_slug for this user.
	slug, err := s.resolveUniqueSlug(ctx, displayName, existingID)
	if err != nil {
		return nil, err
	}

	// MySQL does not support ON CONFLICT ... DO UPDATE; use explicit INSERT/UPDATE instead.
	// SQLite and PostgreSQL both support the UPSERT syntax.
	// Governing: ADR-0002 (pluggable database drivers)
	if s.db.DriverName() == "mysql" {
		if existingID != "" {
			_, err = s.db.ExecContext(ctx, s.q(`
				UPDATE users SET email = ?, display_name = ?, display_name_slug = ?, role = ?, updated_at = ?
				WHERE provider = ? AND subject = ?
			`), email, displayName, slug, role, now, provider, subject)
		} else {
			_, err = s.db.ExecContext(ctx, s.q(`
				INSERT INTO users (id, provider, subject, email, display_name, display_name_slug, role, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			`), id, provider, subject, email, displayName, slug, role, now, now)
		}
	} else {
		// SQLite and PostgreSQL: atomic upsert.
		// Role is included in the UPDATE so admin assignment via email/group is enforced on every login.
		_, err = s.db.ExecContext(ctx, s.q(`
			INSERT INTO users (id, provider, subject, email, display_name, display_name_slug, role, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (provider, subject) DO UPDATE SET
				email = excluded.email,
				display_name = excluded.display_name,
				display_name_slug = excluded.display_name_slug,
				role = excluded.role,
				updated_at = excluded.updated_at
		`), id, provider, subject, email, displayName, slug, role, now, now)
	}
	if err != nil {
		return nil, err
	}

	var u User
	err = s.db.GetContext(ctx, &u, s.q(`SELECT * FROM users WHERE provider = ? AND subject = ?`), provider, subject)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByEmail returns the user matching email, or ErrNotFound.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.db.GetContext(ctx, &u, s.q(`SELECT * FROM users WHERE email = ?`), email)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.db.GetContext(ctx, &u, s.q(`SELECT * FROM users WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListAll returns all users ordered by display name.
// Governing: SPEC-0004 REQ "Admin Dashboard"
func (s *UserStore) ListAll(ctx context.Context) ([]*User, error) {
	var users []*User
	err := s.db.SelectContext(ctx, &users, `SELECT * FROM users ORDER BY display_name ASC`)
	if err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateRole sets the role for the given user and returns the updated record.
// Governing: SPEC-0004 REQ "Admin Dashboard" — inline role toggle
func (s *UserStore) UpdateRole(ctx context.Context, id, role string) (*User, error) {
	_, err := s.db.ExecContext(ctx, s.q(`UPDATE users SET role = ?, updated_at = ? WHERE id = ?`),
		role, time.Now().UTC(), id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// CountPrimaryLinks returns the number of links where userID is the primary owner.
// Governing: SPEC-0011 REQ "Admin User Deletion with Link Handling", ADR-0005
func (s *UserStore) CountPrimaryLinks(ctx context.Context, userID string) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		s.q(`SELECT COUNT(*) FROM link_owners WHERE user_id = ? AND is_primary = 1`), userID)
	return count, err
}

// DeleteUserWithLinks deletes a user and handles their links according to linkAction.
// linkAction "reassign": transfers primary ownership to adminID, removes co-ownership rows.
// linkAction "delete": deletes links where user is sole primary owner, removes co-ownership rows.
// The user record deletion cascades to api_tokens, sessions, and link_owners via FK constraints.
// Governing: SPEC-0011 REQ "Admin User Deletion with Link Handling", REQ "Admin User Deletion Endpoint", ADR-0005
func (s *UserStore) DeleteUserWithLinks(ctx context.Context, targetID, adminID, linkAction string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	switch linkAction {
	case "reassign":
		// Transfer primary ownership to admin
		_, err = tx.ExecContext(ctx,
			tx.Rebind(`UPDATE link_owners SET user_id = ? WHERE user_id = ? AND is_primary = 1`),
			adminID, targetID)
		if err != nil {
			return err
		}
	case "delete":
		// Delete links where target is sole primary owner
		_, err = tx.ExecContext(ctx, tx.Rebind(`
			DELETE FROM links WHERE id IN (
				SELECT link_id FROM link_owners
				WHERE user_id = ? AND is_primary = 1
			)`), targetID)
		if err != nil {
			return err
		}
	}

	// Remove any remaining co-ownership rows for this user (non-primary).
	// Primary rows are handled above: reassigned or cascade-deleted with the link.
	_, err = tx.ExecContext(ctx,
		tx.Rebind(`DELETE FROM link_owners WHERE user_id = ? AND is_primary = 0`), targetID)
	if err != nil {
		return err
	}

	// Delete the user. CASCADE handles api_tokens and sessions.
	_, err = tx.ExecContext(ctx, tx.Rebind(`DELETE FROM users WHERE id = ?`), targetID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CountAll returns the total number of users.
// Governing: SPEC-0016 REQ "Prometheus Metrics Endpoint", ADR-0016
func (s *UserStore) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.GetContext(ctx, &count, s.q(`SELECT COUNT(*) FROM users`))
	return count, err
}
