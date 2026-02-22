// Governing: ADR-0011 REQ "Keyword Host Discovery"
package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Keyword represents a row in the keywords table.
type Keyword struct {
	ID          string    `db:"id"`
	Keyword     string    `db:"keyword"`
	URLTemplate string    `db:"url_template"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
}

// KeywordStore is the sqlx-backed store for keyword operations.
// Governing: ADR-0011 REQ "Keyword Host Discovery"
type KeywordStore struct {
	db *sqlx.DB
}

func NewKeywordStore(db *sqlx.DB) *KeywordStore {
	return &KeywordStore{db: db}
}

// q rebinds ? placeholders to the driver's native format ($1,$2,... for PostgreSQL).
func (s *KeywordStore) q(query string) string { return s.db.Rebind(query) }

// List returns all keywords ordered by keyword name.
func (s *KeywordStore) List(ctx context.Context) ([]*Keyword, error) {
	var keywords []*Keyword
	err := s.db.SelectContext(ctx, &keywords, `SELECT * FROM keywords ORDER BY keyword ASC`)
	if err != nil {
		return nil, err
	}
	return keywords, nil
}

// GetByID returns the keyword matching the given ID, or ErrNotFound.
func (s *KeywordStore) GetByID(ctx context.Context, id string) (*Keyword, error) {
	var k Keyword
	err := s.db.GetContext(ctx, &k, s.q(`SELECT * FROM keywords WHERE id = ?`), id)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// GetByKeyword returns the keyword matching the given keyword string, or ErrNotFound.
func (s *KeywordStore) GetByKeyword(ctx context.Context, keyword string) (*Keyword, error) {
	var k Keyword
	err := s.db.GetContext(ctx, &k, s.q(`SELECT * FROM keywords WHERE keyword = ?`), keyword)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// Create inserts a new keyword and returns it.
func (s *KeywordStore) Create(ctx context.Context, keyword, urlTemplate, description string) (*Keyword, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, s.q(`
		INSERT INTO keywords (id, keyword, url_template, description, created_at) VALUES (?, ?, ?, ?, ?)
	`), id, keyword, urlTemplate, description, now)
	if err != nil {
		return nil, err
	}
	return &Keyword{ID: id, Keyword: keyword, URLTemplate: urlTemplate, Description: description, CreatedAt: now}, nil
}

// Update updates an existing keyword and returns it.
func (s *KeywordStore) Update(ctx context.Context, id, keyword, urlTemplate, description string) (*Keyword, error) {
	result, err := s.db.ExecContext(ctx, s.q(`
		UPDATE keywords SET keyword = ?, url_template = ?, description = ? WHERE id = ?
	`), keyword, urlTemplate, description, id)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, ErrNotFound
	}
	// Re-fetch to get the full row including created_at.
	var k Keyword
	err = s.db.GetContext(ctx, &k, s.q(`SELECT * FROM keywords WHERE id = ?`), id)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// Delete removes a keyword by ID.
func (s *KeywordStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, s.q(`DELETE FROM keywords WHERE id = ?`), id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
