// Governing: SPEC-0002 REQ "Link Store Interface", ADR-0005, ADR-0002
package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Link represents a row in the links table.
type Link struct {
	ID          string    `db:"id"`
	Slug        string    `db:"slug"`
	URL         string    `db:"url"`
	Title       string    `db:"title"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// LinkStore is the sqlx-backed implementation of LinkStoreIface.
// Governing: SPEC-0002 REQ "Link Store Interface"
type LinkStore struct {
	db    *sqlx.DB
	owns  *OwnershipStore
	tags  *TagStore
}

func NewLinkStore(db *sqlx.DB, owns *OwnershipStore, tags *TagStore) *LinkStore {
	return &LinkStore{db: db, owns: owns, tags: tags}
}

// Create inserts a new link and registers ownerID as the primary owner.
func (s *LinkStore) Create(ctx context.Context, slug, url, ownerID, title, description string) (*Link, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO links (id, slug, url, title, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, slug, url, title, description, now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrSlugTaken
		}
		return nil, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO link_owners (link_id, user_id, is_primary) VALUES (?, ?, 1)
	`, id, ownerID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id)
}

// GetBySlug returns the link matching slug, or ErrNotFound.
// Governing: SPEC-0002 REQ "Link Store Interface" — WHEN GetBySlug called with missing slug THEN returns sentinel ErrNotFound
func (s *LinkStore) GetBySlug(ctx context.Context, slug string) (*Link, error) {
	var l Link
	err := s.db.GetContext(ctx, &l, `SELECT * FROM links WHERE slug = ?`, slug)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// GetByID returns the link matching id, or ErrNotFound.
func (s *LinkStore) GetByID(ctx context.Context, id string) (*Link, error) {
	var l Link
	err := s.db.GetContext(ctx, &l, `SELECT * FROM links WHERE id = ?`, id)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// ListByOwner returns all links where userID appears in link_owners.
// Governing: SPEC-0002 REQ "Link Store Interface" — WHEN ListByOwner called with user ID THEN returns all links where user appears in link_owners
func (s *LinkStore) ListByOwner(ctx context.Context, ownerID string) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, `
		SELECT l.* FROM links l
		INNER JOIN link_owners lo ON lo.link_id = l.id
		WHERE lo.user_id = ?
		ORDER BY l.slug ASC
	`, ownerID)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// ListAll returns all links ordered by slug.
func (s *LinkStore) ListAll(ctx context.Context) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, `SELECT * FROM links ORDER BY slug ASC`)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// Update modifies an existing link's slug, url, title, and description.
func (s *LinkStore) Update(ctx context.Context, id, slug, url, title, description string) (*Link, error) {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE links SET slug = ?, url = ?, title = ?, description = ?, updated_at = ? WHERE id = ?
	`, slug, url, title, description, now, id)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrSlugTaken
		}
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// Delete removes a link by ID. CASCADE deletes handle link_owners and link_tags.
func (s *LinkStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM links WHERE id = ?`, id)
	return err
}

// AddOwner adds userID as a co-owner of linkID.
// Returns ErrDuplicateOwner if already present.
func (s *LinkStore) AddOwner(ctx context.Context, linkID, userID string) error {
	err := s.owns.AddOwner(linkID, userID)
	if err == ErrAlreadyOwner {
		return ErrDuplicateOwner
	}
	return err
}

// RemoveOwner removes userID from link_owners. Primary owners cannot be removed.
func (s *LinkStore) RemoveOwner(ctx context.Context, linkID, userID string) error {
	return s.owns.RemoveOwner(linkID, userID)
}

// SetTags replaces the tag set for a link. Tags are upserted by name.
func (s *LinkStore) SetTags(ctx context.Context, linkID string, tagNames []string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing tags for this link.
	_, err = tx.ExecContext(ctx, `DELETE FROM link_tags WHERE link_id = ?`, linkID)
	if err != nil {
		return err
	}

	// Upsert each tag and link it.
	for _, name := range tagNames {
		tag, err := s.tags.upsertTx(ctx, tx, name)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO link_tags (link_id, tag_id) VALUES (?, ?)
		`, linkID, tag.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ListTags returns all tags associated with a link.
func (s *LinkStore) ListTags(ctx context.Context, linkID string) ([]*Tag, error) {
	var tags []*Tag
	err := s.db.SelectContext(ctx, &tags, `
		SELECT t.* FROM tags t
		INNER JOIN link_tags lt ON lt.tag_id = t.id
		WHERE lt.link_id = ?
		ORDER BY t.name ASC
	`, linkID)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// ListByTag returns all links that have the given tag slug.
func (s *LinkStore) ListByTag(ctx context.Context, tagSlug string) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, `
		SELECT l.* FROM links l
		INNER JOIN link_tags lt ON lt.link_id = l.id
		INNER JOIN tags t ON t.id = lt.tag_id
		WHERE t.slug = ?
		ORDER BY l.slug ASC
	`, tagSlug)
	if err != nil {
		return nil, err
	}
	return links, nil
}
