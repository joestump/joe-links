// Governing: SPEC-0001 REQ "Short Link Management", ADR-0002
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Link struct {
	ID          string    `db:"id"`
	Slug        string    `db:"slug"`
	URL         string    `db:"url"`
	OwnerID     string    `db:"owner_id"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type LinkStore struct {
	db *sqlx.DB
}

func NewLinkStore(db *sqlx.DB) *LinkStore {
	return &LinkStore{db: db}
}

func (s *LinkStore) GetBySlug(ctx context.Context, slug string) (*Link, error) {
	var l Link
	err := s.db.GetContext(ctx, &l, `SELECT * FROM links WHERE slug = ?`, slug)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *LinkStore) GetByID(ctx context.Context, id string) (*Link, error) {
	var l Link
	err := s.db.GetContext(ctx, &l, `SELECT * FROM links WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *LinkStore) ListByOwner(ctx context.Context, ownerID string) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, `SELECT * FROM links WHERE owner_id = ? ORDER BY slug ASC`, ownerID)
	if err != nil {
		return nil, err
	}
	return links, nil
}

func (s *LinkStore) ListAll(ctx context.Context) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, `SELECT * FROM links ORDER BY slug ASC`)
	if err != nil {
		return nil, err
	}
	return links, nil
}

func (s *LinkStore) Create(ctx context.Context, slug, url, ownerID, description string) (*Link, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO links (id, slug, url, owner_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, slug, url, ownerID, description, now, now)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *LinkStore) Update(ctx context.Context, id, slug, url, description string) (*Link, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE links SET slug = ?, url = ?, description = ?, updated_at = ? WHERE id = ?
	`, slug, url, description, now, id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

func (s *LinkStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM links WHERE id = ?`, id)
	return err
}
