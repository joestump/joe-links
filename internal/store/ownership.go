// Governing: SPEC-0002 REQ "Multi-Ownership via link_owners", "Authorization Based on Ownership", ADR-0005
package store

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
)

var (
	ErrNotOwner              = errors.New("user is not an owner of this link")
	ErrPrimaryOwnerImmutable = errors.New("primary owner cannot be removed")
	ErrAlreadyOwner          = errors.New("user is already an owner of this link")
)

// OwnershipStore manages link_owners relationships.
// Governing: SPEC-0002 REQ "Multi-Ownership via link_owners", ADR-0005
type OwnershipStore struct {
	db *sqlx.DB
}

func NewOwnershipStore(db *sqlx.DB) *OwnershipStore {
	return &OwnershipStore{db: db}
}

// AddOwner adds userID as a co-owner (is_primary=false) of linkID.
// Returns ErrAlreadyOwner if already present.
func (s *OwnershipStore) AddOwner(linkID, userID string) error {
	_, err := s.db.Exec(
		`INSERT INTO link_owners (link_id, user_id, is_primary) VALUES (?, ?, 0)`,
		linkID, userID,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return ErrAlreadyOwner
		}
		return err
	}
	return nil
}

// AddPrimaryOwner adds userID as the primary owner during link creation.
func (s *OwnershipStore) AddPrimaryOwner(linkID, userID string) error {
	_, err := s.db.Exec(
		`INSERT INTO link_owners (link_id, user_id, is_primary) VALUES (?, ?, 1)`,
		linkID, userID,
	)
	return err
}

// RemoveOwner removes userID from link_owners. Returns ErrPrimaryOwnerImmutable if is_primary=1.
func (s *OwnershipStore) RemoveOwner(linkID, userID string) error {
	var isPrimary bool
	err := s.db.QueryRow(
		`SELECT is_primary FROM link_owners WHERE link_id = ? AND user_id = ?`,
		linkID, userID,
	).Scan(&isPrimary)
	if err == sql.ErrNoRows {
		return ErrNotOwner
	}
	if err != nil {
		return err
	}
	if isPrimary {
		return ErrPrimaryOwnerImmutable
	}
	_, err = s.db.Exec(`DELETE FROM link_owners WHERE link_id = ? AND user_id = ?`, linkID, userID)
	return err
}

// IsOwner returns true if userID is in link_owners for linkID.
func (s *OwnershipStore) IsOwner(linkID, userID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM link_owners WHERE link_id = ? AND user_id = ?`,
		linkID, userID,
	).Scan(&count)
	return count > 0, err
}

// ListOwners returns all user IDs that own linkID.
func (s *OwnershipStore) ListOwners(linkID string) ([]string, error) {
	var owners []string
	err := s.db.Select(&owners, `SELECT user_id FROM link_owners WHERE link_id = ?`, linkID)
	return owners, err
}

// OwnerInfo represents a link owner with their user details and primary status.
type OwnerInfo struct {
	User
	IsPrimary bool `db:"is_primary"`
}

// ListOwnerUsers returns full user records for all owners of a link.
func (s *OwnershipStore) ListOwnerUsers(linkID string) ([]*OwnerInfo, error) {
	var owners []*OwnerInfo
	err := s.db.Select(&owners, `
		SELECT u.*, lo.is_primary FROM users u
		INNER JOIN link_owners lo ON lo.user_id = u.id
		WHERE lo.link_id = ?
		ORDER BY lo.is_primary DESC, u.display_name ASC
	`, linkID)
	return owners, err
}

// isUniqueConstraintError checks whether err indicates a unique constraint violation.
// Works across SQLite, PostgreSQL, and MySQL.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || // SQLite & PostgreSQL
		strings.Contains(msg, "duplicate key") || // PostgreSQL
		strings.Contains(msg, "duplicate entry") // MySQL
}
