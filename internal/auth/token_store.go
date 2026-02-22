// Governing: SPEC-0006 REQ "Token Format", REQ "Token Storage", REQ "api_tokens Table", ADR-0009
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/joestump/joe-links/internal/store"
)

// TokenRecord represents a row in the api_tokens table.
type TokenRecord struct {
	ID         string       `db:"id"`
	UserID     string       `db:"user_id"`
	Name       string       `db:"name"`
	TokenHash  string       `db:"token_hash"`
	LastUsedAt sql.NullTime `db:"last_used_at"`
	ExpiresAt  sql.NullTime `db:"expires_at"`
	CreatedAt  time.Time    `db:"created_at"`
	RevokedAt  sql.NullTime `db:"revoked_at"`
}

// TokenStore defines operations for API token management.
type TokenStore interface {
	Create(ctx context.Context, userID, name, tokenHash string, expiresAt *time.Time) (*TokenRecord, error)
	GetByHash(ctx context.Context, hash string) (*TokenRecord, error)
	ListByUser(ctx context.Context, userID string) ([]*TokenRecord, error)
	Revoke(ctx context.Context, id, userID string) error
	UpdateLastUsed(ctx context.Context, id string) error
}

// SQLTokenStore is the sqlx-backed implementation of TokenStore.
type SQLTokenStore struct {
	db *sqlx.DB
}

// NewSQLTokenStore creates a new SQLTokenStore.
func NewSQLTokenStore(db *sqlx.DB) *SQLTokenStore {
	return &SQLTokenStore{db: db}
}

// q rebinds ? placeholders to the driver's native format ($1,$2,... for PostgreSQL).
func (s *SQLTokenStore) q(query string) string { return s.db.Rebind(query) }

// Create inserts a new API token record.
func (s *SQLTokenStore) Create(ctx context.Context, userID, name, tokenHash string, expiresAt *time.Time) (*TokenRecord, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	var exp sql.NullTime
	if expiresAt != nil {
		exp = sql.NullTime{Time: *expiresAt, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, s.q(`
		INSERT INTO api_tokens (id, user_id, name, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`), id, userID, name, tokenHash, exp, now)
	if err != nil {
		return nil, err
	}

	var rec TokenRecord
	err = s.db.GetContext(ctx, &rec, s.q(`SELECT * FROM api_tokens WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetByHash returns the token record matching the given hash, or store.ErrNotFound.
func (s *SQLTokenStore) GetByHash(ctx context.Context, hash string) (*TokenRecord, error) {
	var rec TokenRecord
	err := s.db.GetContext(ctx, &rec, s.q(`SELECT * FROM api_tokens WHERE token_hash = ?`), hash)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ListByUser returns all token records for the given user, ordered by creation time descending.
func (s *SQLTokenStore) ListByUser(ctx context.Context, userID string) ([]*TokenRecord, error) {
	var records []*TokenRecord
	err := s.db.SelectContext(ctx, &records, s.q(`
		SELECT * FROM api_tokens WHERE user_id = ? ORDER BY created_at DESC
	`), userID)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// Revoke marks a token as revoked. Returns store.ErrNotFound if the token does not exist
// or is not owned by the given user.
func (s *SQLTokenStore) Revoke(ctx context.Context, id, userID string) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, s.q(`
		UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND user_id = ?
	`), now, id, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return store.ErrNotFound
	}
	return nil
}

// UpdateLastUsed updates the last_used_at timestamp for the given token.
func (s *SQLTokenStore) UpdateLastUsed(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, s.q(`
		UPDATE api_tokens SET last_used_at = ? WHERE id = ?
	`), now, id)
	return err
}

// GenerateToken creates a new API token with the "jl_" prefix.
// It returns the plaintext token, its SHA-256 hash, and any error.
// Plaintext = "jl_" + base62-encoded 32 cryptographically random bytes.
// Hash = hex-encoded SHA-256 of the plaintext.
func GenerateToken() (plaintext, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}

	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	encoded := make([]byte, 0, 44)
	n := new(big.Int).SetBytes(b)
	base := big.NewInt(62)
	mod := new(big.Int)
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		encoded = append(encoded, alphabet[mod.Int64()])
	}
	// Reverse to get most-significant digit first.
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}

	plaintext = "jl_" + string(encoded)
	h := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(h[:])
	return
}

// HashToken returns the hex-encoded SHA-256 hash of a plaintext token.
func HashToken(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}
