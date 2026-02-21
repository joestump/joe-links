// Governing: SPEC-0002 REQ "Link Store Interface", ADR-0005
package store

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var tagSlugStripRe = regexp.MustCompile(`[^a-z0-9-]`)

// Tag represents a row in the tags table.
type Tag struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Slug      string    `db:"slug"`
	CreatedAt time.Time `db:"created_at"`
}

// TagStore is the sqlx-backed implementation of TagStoreIface.
// Governing: SPEC-0002 REQ "Link Store Interface", ADR-0005
type TagStore struct {
	db *sqlx.DB
}

func NewTagStore(db *sqlx.DB) *TagStore {
	return &TagStore{db: db}
}

// DeriveTagSlug derives a URL-safe slug from a tag name:
// lowercase, replace spaces/underscores with hyphens, strip non-[a-z0-9-].
// Governing: ADR-0005 (tag slug derivation)
func DeriveTagSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = tagSlugStripRe.ReplaceAllString(s, "")
	// Collapse consecutive hyphens.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

// Upsert creates a tag if it doesn't exist (by slug), or returns the existing one.
func (s *TagStore) Upsert(ctx context.Context, name string) (*Tag, error) {
	slug := DeriveTagSlug(name)

	var existing Tag
	err := s.db.GetContext(ctx, &existing, `SELECT * FROM tags WHERE slug = ?`, slug)
	if err == nil {
		return &existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tags (id, name, slug, created_at) VALUES (?, ?, ?, ?)
	`, id, strings.TrimSpace(name), slug, now)
	if err != nil {
		// Race condition: another goroutine inserted first. Re-fetch.
		if isUniqueConstraintError(err) {
			err = s.db.GetContext(ctx, &existing, `SELECT * FROM tags WHERE slug = ?`, slug)
			if err != nil {
				return nil, err
			}
			return &existing, nil
		}
		return nil, err
	}

	return &Tag{ID: id, Name: strings.TrimSpace(name), Slug: slug, CreatedAt: now}, nil
}

// upsertTx is the transactional variant used by LinkStore.SetTags.
func (s *TagStore) upsertTx(ctx context.Context, tx *sqlx.Tx, name string) (*Tag, error) {
	slug := DeriveTagSlug(name)

	var existing Tag
	err := tx.GetContext(ctx, &existing, `SELECT * FROM tags WHERE slug = ?`, slug)
	if err == nil {
		return &existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tags (id, name, slug, created_at) VALUES (?, ?, ?, ?)
	`, id, strings.TrimSpace(name), slug, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			err = tx.GetContext(ctx, &existing, `SELECT * FROM tags WHERE slug = ?`, slug)
			if err != nil {
				return nil, err
			}
			return &existing, nil
		}
		return nil, err
	}

	return &Tag{ID: id, Name: strings.TrimSpace(name), Slug: slug, CreatedAt: now}, nil
}

// GetBySlug returns the tag matching slug, or ErrNotFound.
func (s *TagStore) GetBySlug(ctx context.Context, slug string) (*Tag, error) {
	var t Tag
	err := s.db.GetContext(ctx, &t, `SELECT * FROM tags WHERE slug = ?`, slug)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListAll returns all tags ordered by name.
func (s *TagStore) ListAll(ctx context.Context) ([]*Tag, error) {
	var tags []*Tag
	err := s.db.SelectContext(ctx, &tags, `SELECT * FROM tags ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	return tags, nil
}
