// Governing: SPEC-0002 REQ "Link Store Interface", ADR-0005, ADR-0002
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	Visibility  string    `db:"visibility"` // Governing: SPEC-0010 REQ "Visibility Column on Links Table"
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// ShareRecord represents a row in the link_shares table.
// Governing: SPEC-0010 REQ "Link Shares Table"
type ShareRecord struct {
	LinkID    string    `db:"link_id"`
	UserID    string    `db:"user_id"`
	SharedBy  string    `db:"shared_by"`
	CreatedAt time.Time `db:"created_at"`
}

// LinkStore is the sqlx-backed implementation of LinkStoreIface.
// Governing: SPEC-0002 REQ "Link Store Interface"
type LinkStore struct {
	db   *sqlx.DB
	owns *OwnershipStore
	tags *TagStore
}

func NewLinkStore(db *sqlx.DB, owns *OwnershipStore, tags *TagStore) *LinkStore {
	return &LinkStore{db: db, owns: owns, tags: tags}
}

// q rebinds ? placeholders to the driver's native format ($1,$2,... for PostgreSQL).
func (s *LinkStore) q(query string) string { return s.db.Rebind(query) }

// agg returns a dialect-appropriate string aggregation expression with deduplication.
// PostgreSQL: STRING_AGG(DISTINCT col, ',') — SQLite/MySQL: GROUP_CONCAT(DISTINCT col)
func (s *LinkStore) aggDistinct(col string) string {
	if s.db.DriverName() == "postgres" {
		return "COALESCE(STRING_AGG(DISTINCT " + col + ", ','), '')"
	}
	return "COALESCE(GROUP_CONCAT(DISTINCT " + col + "), '')"
}

// Create inserts a new link and registers ownerID as the primary owner.
// Governing: SPEC-0010 REQ "Visibility Selector in Link Forms"
func (s *LinkStore) Create(ctx context.Context, slug, url, ownerID, title, description, visibility string) (*Link, error) {
	// Governing: SPEC-0002 REQ "Links Table" — reject over-length title/description before insert
	if err := ValidateLinkText(title, description); err != nil {
		return nil, err
	}
	if visibility == "" {
		visibility = "public"
	}
	id := uuid.New().String()
	now := time.Now().UTC()

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, tx.Rebind(`
		INSERT INTO links (id, slug, url, title, description, visibility, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`), id, slug, url, title, description, visibility, now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrSlugTaken
		}
		return nil, err
	}

	_, err = tx.ExecContext(ctx, tx.Rebind(`
		INSERT INTO link_owners (link_id, user_id, is_primary, created_at) VALUES (?, ?, 1, ?)
	`), id, ownerID, now)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id)
}

// CreateFull creates a link together with its primary owner, tags, and share
// grants in a single transaction: if any write fails, no partial link exists.
// Tag names are deduplicated by their upserted tag ID so duplicate spellings
// of the same tag cannot violate the link_tags primary key and roll back the
// write. Share user IDs must be pre-resolved to existing users by the caller.
// Governing: SPEC-0018 REQ "Database Operation Standards", ADR-0018
func (s *LinkStore) CreateFull(ctx context.Context, slug, url, ownerID, title, description, visibility string, tagNames, shareUserIDs []string, sharedBy string) (*Link, error) {
	// Governing: SPEC-0002 REQ "Links Table" — reject over-length title/description before insert
	if err := ValidateLinkText(title, description); err != nil {
		return nil, err
	}
	if visibility == "" {
		visibility = "public"
	}
	id := uuid.New().String()
	now := time.Now().UTC()

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create full link: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, tx.Rebind(`
		INSERT INTO links (id, slug, url, title, description, visibility, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`), id, slug, url, title, description, visibility, now, now)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("create full link: insert link: %w", err)
	}

	_, err = tx.ExecContext(ctx, tx.Rebind(`
		INSERT INTO link_owners (link_id, user_id, is_primary, created_at) VALUES (?, ?, 1, ?)
	`), id, ownerID, now)
	if err != nil {
		return nil, fmt.Errorf("create full link: insert owner: %w", err)
	}

	seenTags := make(map[string]bool, len(tagNames))
	for _, name := range tagNames {
		tag, err := s.tags.upsertTx(ctx, tx, name)
		if err != nil {
			return nil, fmt.Errorf("create full link: upsert tag %q: %w", name, err)
		}
		if seenTags[tag.ID] {
			continue
		}
		seenTags[tag.ID] = true
		_, err = tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO link_tags (link_id, tag_id) VALUES (?, ?)
		`), id, tag.ID)
		if err != nil {
			return nil, fmt.Errorf("create full link: insert link_tag: %w", err)
		}
	}

	seenShares := make(map[string]bool, len(shareUserIDs))
	for _, uid := range shareUserIDs {
		if seenShares[uid] {
			continue
		}
		seenShares[uid] = true
		_, err = tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO link_shares (link_id, user_id, shared_by) VALUES (?, ?, ?)
		`), id, uid, sharedBy)
		if err != nil {
			return nil, fmt.Errorf("create full link: insert share: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("create full link: commit: %w", err)
	}

	return s.GetByID(ctx, id)
}

// GetBySlug returns the link matching slug, or ErrNotFound.
// Governing: SPEC-0002 REQ "Link Store Interface" — WHEN GetBySlug called with missing slug THEN returns sentinel ErrNotFound
func (s *LinkStore) GetBySlug(ctx context.Context, slug string) (*Link, error) {
	var l Link
	err := s.db.GetContext(ctx, &l, s.q(`SELECT * FROM links WHERE slug = ?`), slug)
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
	err := s.db.GetContext(ctx, &l, s.q(`SELECT * FROM links WHERE id = ?`), id)
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
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT l.* FROM links l
		INNER JOIN link_owners lo ON lo.link_id = l.id
		WHERE lo.user_id = ?
		ORDER BY l.slug ASC
	`), ownerID)
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

// ListAllPaginated returns links ordered by (slug, id), starting after the
// given keyset cursor. It fetches up to limit rows; pass cursorSlug/cursorID
// from the last row of the previous page (empty for the first page).
// Governing: SPEC-0005 REQ "Pagination"
func (s *LinkStore) ListAllPaginated(ctx context.Context, limit int, cursorSlug, cursorID string) ([]*Link, error) {
	var links []*Link
	if cursorSlug == "" && cursorID == "" {
		err := s.db.SelectContext(ctx, &links, s.q(`
			SELECT * FROM links
			ORDER BY slug ASC, id ASC
			LIMIT ?
		`), limit)
		return links, err
	}
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT * FROM links
		WHERE slug > ? OR (slug = ? AND id > ?)
		ORDER BY slug ASC, id ASC
		LIMIT ?
	`), cursorSlug, cursorSlug, cursorID, limit)
	return links, err
}

// ListByOwnerOrSharedPaginated returns links where userID is an owner or has a
// share record, ordered by (slug, id) and keyset-paginated.
// Governing: SPEC-0005 REQ "Pagination", SPEC-0010 REQ "REST API Visibility Field"
func (s *LinkStore) ListByOwnerOrSharedPaginated(ctx context.Context, userID string, limit int, cursorSlug, cursorID string) ([]*Link, error) {
	var links []*Link
	if cursorSlug == "" && cursorID == "" {
		err := s.db.SelectContext(ctx, &links, s.q(`
			SELECT DISTINCT l.* FROM links l
			LEFT JOIN link_owners lo ON lo.link_id = l.id AND lo.user_id = ?
			LEFT JOIN link_shares ls ON ls.link_id = l.id AND ls.user_id = ?
			WHERE lo.user_id IS NOT NULL OR ls.user_id IS NOT NULL
			ORDER BY l.slug ASC, l.id ASC
			LIMIT ?
		`), userID, userID, limit)
		return links, err
	}
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT DISTINCT l.* FROM links l
		LEFT JOIN link_owners lo ON lo.link_id = l.id AND lo.user_id = ?
		LEFT JOIN link_shares ls ON ls.link_id = l.id AND ls.user_id = ?
		WHERE (lo.user_id IS NOT NULL OR ls.user_id IS NOT NULL)
		  AND (l.slug > ? OR (l.slug = ? AND l.id > ?))
		ORDER BY l.slug ASC, l.id ASC
		LIMIT ?
	`), userID, userID, cursorSlug, cursorSlug, cursorID, limit)
	return links, err
}

// SearchByOwner returns links owned by userID whose slug, url, or description
// contain the search term (case-insensitive LIKE). Returns all owner links if q is empty.
// Governing: SPEC-0004 REQ "User Dashboard" — HTMX debounced search
func (s *LinkStore) SearchByOwner(ctx context.Context, ownerID, q string) ([]*Link, error) {
	if q == "" {
		return s.ListByOwner(ctx, ownerID)
	}
	var links []*Link
	pattern := "%" + q + "%"
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT l.* FROM links l
		INNER JOIN link_owners lo ON lo.link_id = l.id
		WHERE lo.user_id = ?
		  AND (l.slug LIKE ? OR l.url LIKE ? OR l.description LIKE ?)
		ORDER BY l.slug ASC
	`), ownerID, pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// SearchAll returns all links whose slug, url, or description contain the
// search term (case-insensitive LIKE). Returns all links if q is empty.
// Governing: SPEC-0004 REQ "User Dashboard" — HTMX debounced search (admin view)
func (s *LinkStore) SearchAll(ctx context.Context, q string) ([]*Link, error) {
	if q == "" {
		return s.ListAll(ctx)
	}
	var links []*Link
	pattern := "%" + q + "%"
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT * FROM links
		WHERE slug LIKE ? OR url LIKE ? OR description LIKE ?
		ORDER BY slug ASC
	`), pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// ListByOwnerAndTag returns links owned by userID that have the given tag slug.
// Governing: SPEC-0004 REQ "User Dashboard" — tag filter
func (s *LinkStore) ListByOwnerAndTag(ctx context.Context, ownerID, tagSlug string) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT l.* FROM links l
		INNER JOIN link_owners lo ON lo.link_id = l.id
		INNER JOIN link_tags lt ON lt.link_id = l.id
		INNER JOIN tags t ON t.id = lt.tag_id
		WHERE lo.user_id = ? AND t.slug = ?
		ORDER BY l.slug ASC
	`), ownerID, tagSlug)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// Update modifies an existing link's url, title, description, and visibility.
// Governing: SPEC-0001 REQ "Short Link Management" — slug is immutable after creation.
// Governing: SPEC-0010 REQ "Visibility Selector in Link Forms"
func (s *LinkStore) Update(ctx context.Context, id, url, title, description, visibility string) (*Link, error) {
	// Governing: SPEC-0002 REQ "Links Table" — reject over-length title/description before update
	if err := ValidateLinkText(title, description); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, s.q(`
		UPDATE links SET url = ?, title = ?, description = ?, visibility = ?, updated_at = ? WHERE id = ?
	`), url, title, description, visibility, now, id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// UpdateVisibility sets the visibility field on a link.
// Governing: SPEC-0010 REQ "Visibility Column on Links Table", REQ "Admin Visibility Override"
func (s *LinkStore) UpdateVisibility(ctx context.Context, id, visibility string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, s.q(`UPDATE links SET visibility = ?, updated_at = ? WHERE id = ?`),
		visibility, now, id)
	return err
}

// ListByOwnerOrShared returns links where userID is an owner or has a share record.
// Governing: SPEC-0010 REQ "REST API Visibility Field"
func (s *LinkStore) ListByOwnerOrShared(ctx context.Context, userID string) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT DISTINCT l.* FROM links l
		LEFT JOIN link_owners lo ON lo.link_id = l.id AND lo.user_id = ?
		LEFT JOIN link_shares ls ON ls.link_id = l.id AND ls.user_id = ?
		WHERE lo.user_id IS NOT NULL OR ls.user_id IS NOT NULL
		ORDER BY l.slug ASC
	`), userID, userID)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// ListByURL returns links whose URL exactly matches the given string.
// Admins see all matches; regular users see owned, shared, or public links.
func (s *LinkStore) ListByURL(ctx context.Context, url, userID string, isAdmin bool) ([]*Link, error) {
	var links []*Link
	if isAdmin {
		err := s.db.SelectContext(ctx, &links, s.q(`
			SELECT * FROM links WHERE url = ? ORDER BY created_at DESC
		`), url)
		return links, err
	}
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT DISTINCT l.* FROM links l
		LEFT JOIN link_owners lo ON lo.link_id = l.id AND lo.user_id = ?
		LEFT JOIN link_shares ls ON ls.link_id = l.id AND ls.user_id = ?
		WHERE l.url = ? AND (l.visibility = 'public' OR lo.user_id IS NOT NULL OR ls.user_id IS NOT NULL)
		ORDER BY l.created_at DESC
	`), userID, userID, url)
	return links, err
}

// Delete removes a link by ID. CASCADE deletes handle link_owners and link_tags.
func (s *LinkStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, s.q(`DELETE FROM links WHERE id = ?`), id)
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
	defer func() { _ = tx.Rollback() }()

	// Clear existing tags for this link.
	_, err = tx.ExecContext(ctx, tx.Rebind(`DELETE FROM link_tags WHERE link_id = ?`), linkID)
	if err != nil {
		return err
	}

	// Upsert each tag and link it.
	for _, name := range tagNames {
		tag, err := s.tags.upsertTx(ctx, tx, name)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO link_tags (link_id, tag_id) VALUES (?, ?)
		`), linkID, tag.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ListTags returns all tags associated with a link.
func (s *LinkStore) ListTags(ctx context.Context, linkID string) ([]*Tag, error) {
	var tags []*Tag
	err := s.db.SelectContext(ctx, &tags, s.q(`
		SELECT t.* FROM tags t
		INNER JOIN link_tags lt ON lt.tag_id = t.id
		WHERE lt.link_id = ?
		ORDER BY t.name ASC
	`), linkID)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// AdminLink is a Link augmented with owner display names and tag names for admin views.
// Governing: SPEC-0011 REQ "Admin Links Screen", ADR-0005
type AdminLink struct {
	Link
	Owners    string `db:"owners"`     // comma-separated owner display names
	Tags      string `db:"tags"`       // comma-separated tag names
	OwnerSlug string `db:"owner_slug"` // primary owner's display_name_slug (populated in public views)
	IsOwner   bool   `db:"is_owner"`   // true if the querying user is an owner (populated in public views)
}

// TagList returns tag names as a slice for template iteration.
func (a *AdminLink) TagList() []string {
	if a.Tags == "" {
		return nil
	}
	return strings.Split(a.Tags, ",")
}

// ListAllAdmin returns all links with owner display names and tags joined, ordered by slug.
// Supports optional search query filtering by slug, URL, title, or owner display name.
// Governing: SPEC-0011 REQ "Admin Links Screen"
func (s *LinkStore) ListAllAdmin(ctx context.Context, q string) ([]*AdminLink, error) {
	query := fmt.Sprintf(`
		SELECT l.*,
			%s AS owners,
			%s AS tags
		FROM links l
		LEFT JOIN link_owners lo ON lo.link_id = l.id
		LEFT JOIN users u ON u.id = lo.user_id
		LEFT JOIN link_tags lt ON lt.link_id = l.id
		LEFT JOIN tags t ON t.id = lt.tag_id`,
		s.aggDistinct("u.display_name"),
		s.aggDistinct("t.name"),
	)
	var args []interface{}
	if q != "" {
		pattern := "%" + q + "%"
		query += ` WHERE l.slug LIKE ? OR l.url LIKE ? OR l.title LIKE ? OR u.display_name LIKE ?`
		args = append(args, pattern, pattern, pattern, pattern)
	}
	query += ` GROUP BY l.id ORDER BY l.slug ASC`

	var links []*AdminLink
	err := s.db.SelectContext(ctx, &links, s.q(query), args...)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// GetAdminLink returns a single link with owner display names and tags joined.
// Governing: SPEC-0011 REQ "Admin Inline Link Editing"
func (s *LinkStore) GetAdminLink(ctx context.Context, id string) (*AdminLink, error) {
	var link AdminLink
	err := s.db.GetContext(ctx, &link, s.q(fmt.Sprintf(`
		SELECT l.*,
			%s AS owners,
			%s AS tags
		FROM links l
		LEFT JOIN link_owners lo ON lo.link_id = l.id
		LEFT JOIN users u ON u.id = lo.user_id
		LEFT JOIN link_tags lt ON lt.link_id = l.id
		LEFT JOIN tags t ON t.id = lt.tag_id
		WHERE l.id = ?
		GROUP BY l.id`,
		s.aggDistinct("u.display_name"),
		s.aggDistinct("t.name"),
	)), id)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// ListByTag returns all links that have the given tag slug.
func (s *LinkStore) ListByTag(ctx context.Context, tagSlug string) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT l.* FROM links l
		INNER JOIN link_tags lt ON lt.link_id = l.id
		INNER JOIN tags t ON t.id = lt.tag_id
		WHERE t.slug = ?
		ORDER BY l.slug ASC
	`), tagSlug)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// ListPublic returns paginated public links, optionally filtered by search query.
// Returns the matching links and total count for pagination.
// Governing: SPEC-0012 REQ "Public Link Browser (GET /links)", REQ "Public Link Search"
// ListPublic returns paginated public links as AdminLink rows, optionally filtered by query.
// currentUserID is used to set IsOwner; pass "" for unauthenticated callers.
// Governing: SPEC-0012 REQ "Public Link Browser (GET /links)", REQ "Public Link Search"
func (s *LinkStore) ListPublic(ctx context.Context, currentUserID, q string, page, perPage int) ([]*AdminLink, int, error) {
	baseWhere := `WHERE l.visibility = 'public'`
	var args []interface{}
	if q != "" {
		pattern := "%" + q + "%"
		baseWhere += ` AND (l.slug LIKE ? OR l.url LIKE ? OR l.title LIKE ? OR l.description LIKE ?)`
		args = append(args, pattern, pattern, pattern, pattern)
	}

	// Count total matching rows.
	countQuery := `SELECT COUNT(DISTINCT l.id) FROM links l ` + baseWhere
	var total int
	if err := s.db.GetContext(ctx, &total, s.q(countQuery), args...); err != nil {
		return nil, 0, err
	}

	// Fetch paginated results.
	// COALESCE(MAX(u.*), '') is used instead of bare u.* so that PostgreSQL's
	// strict GROUP BY is satisfied; the JOIN on is_primary=1 ensures at most
	// one owner row per link, so MAX() returns the same value as the bare column.
	offset := (page - 1) * perPage
	query := fmt.Sprintf(`
		SELECT l.*,
		       COALESCE(MAX(u.display_name), '') AS owners,
		       COALESCE(MAX(u.display_name_slug), '') AS owner_slug,
		       %s AS tags,
		       CASE WHEN EXISTS(
		           SELECT 1 FROM link_owners lo2 WHERE lo2.link_id = l.id AND lo2.user_id = ?
		       ) THEN 1 ELSE 0 END AS is_owner
		FROM links l
		LEFT JOIN link_owners lo ON lo.link_id = l.id AND lo.is_primary = 1
		LEFT JOIN users u ON u.id = lo.user_id
		LEFT JOIN link_tags lt ON lt.link_id = l.id
		LEFT JOIN tags t ON t.id = lt.tag_id
		`+baseWhere+`
		GROUP BY l.id
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?`,
		s.aggDistinct("t.name"),
	)
	fetchArgs := append([]interface{}{currentUserID}, args...)
	fetchArgs = append(fetchArgs, perPage, offset)

	var links []*AdminLink
	if err := s.db.SelectContext(ctx, &links, s.q(query), fetchArgs...); err != nil {
		return nil, 0, err
	}
	return links, total, nil
}

// ListSharedWithUser returns links shared with the given user via link_shares.
// Governing: SPEC-0010 REQ "Dashboard Visibility Filtering"
func (s *LinkStore) ListSharedWithUser(ctx context.Context, userID string) ([]*Link, error) {
	var links []*Link
	err := s.db.SelectContext(ctx, &links, s.q(`
		SELECT l.* FROM links l
		INNER JOIN link_shares ls ON ls.link_id = l.id
		WHERE ls.user_id = ?
		ORDER BY l.slug ASC
	`), userID)
	if err != nil {
		return nil, err
	}
	return links, nil
}

// HasShare checks if user has a link_shares record.
// Governing: SPEC-0010 REQ "Link Shares Table"
func (s *LinkStore) HasShare(ctx context.Context, linkID, userID string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		s.q(`SELECT COUNT(*) FROM link_shares WHERE link_id = ? AND user_id = ?`), linkID, userID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// AddShare creates a link_shares record.
// Governing: SPEC-0010 REQ "Link Shares Table"
func (s *LinkStore) AddShare(ctx context.Context, linkID, userID, sharedBy string) error {
	_, err := s.db.ExecContext(ctx, s.q(`
		INSERT INTO link_shares (link_id, user_id, shared_by) VALUES (?, ?, ?)
	`), linkID, userID, sharedBy)
	return err
}

// RemoveShare deletes a link_shares record.
// Governing: SPEC-0010 REQ "Link Shares Table"
func (s *LinkStore) RemoveShare(ctx context.Context, linkID, userID string) error {
	_, err := s.db.ExecContext(ctx, s.q(`
		DELETE FROM link_shares WHERE link_id = ? AND user_id = ?
	`), linkID, userID)
	return err
}

// ListPublicByOwner returns public links owned by userID as AdminLink rows
// (owner display name, owner slug, tags) so the shared link_list partial can
// render them identically to the public link browser. Paginated.
// currentUserID is used to set IsOwner; pass "" for unauthenticated callers.
// Returns the links, total count, and any error.
// Governing: SPEC-0012 REQ "User Profile Page (GET /u/{display_name_slug})"
// Governing: SPEC-0014 REQ "Abstract Link Widget" — same shape as ListPublic
func (s *LinkStore) ListPublicByOwner(ctx context.Context, userID, currentUserID string, page, perPage int) ([]*AdminLink, int, error) {
	// Count total matching links
	var total int
	err := s.db.GetContext(ctx, &total, s.q(`
		SELECT COUNT(DISTINCT l.id) FROM links l
		JOIN link_owners lo ON lo.link_id = l.id AND lo.is_primary = 1
		WHERE l.visibility = 'public' AND lo.user_id = ?
	`), userID)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	var links []*AdminLink
	err = s.db.SelectContext(ctx, &links, s.q(fmt.Sprintf(`
		SELECT l.*,
		       COALESCE(MAX(u.display_name), '') AS owners,
		       COALESCE(MAX(u.display_name_slug), '') AS owner_slug,
		       %s AS tags,
		       CASE WHEN EXISTS(
		           SELECT 1 FROM link_owners lo2 WHERE lo2.link_id = l.id AND lo2.user_id = ?
		       ) THEN 1 ELSE 0 END AS is_owner
		FROM links l
		JOIN link_owners lo ON lo.link_id = l.id AND lo.is_primary = 1
		JOIN users u ON u.id = lo.user_id
		LEFT JOIN link_tags lt ON lt.link_id = l.id
		LEFT JOIN tags t ON t.id = lt.tag_id
		WHERE l.visibility = 'public'
		  AND lo.user_id = ?
		GROUP BY l.id
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?
	`, s.aggDistinct("t.name"))), currentUserID, userID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}

	return links, total, nil
}

// CountAll returns the total number of links.
// Governing: SPEC-0016 REQ "Prometheus Metrics Endpoint", ADR-0016
func (s *LinkStore) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.GetContext(ctx, &count, s.q(`SELECT COUNT(*) FROM links`))
	return count, err
}

// ListShares returns all users with access to a link.
// Governing: SPEC-0010 REQ "Link Shares Table"
func (s *LinkStore) ListShares(ctx context.Context, linkID string) ([]ShareRecord, error) {
	var shares []ShareRecord
	err := s.db.SelectContext(ctx, &shares, s.q(`
		SELECT * FROM link_shares WHERE link_id = ? ORDER BY created_at ASC
	`), linkID)
	if err != nil {
		return nil, err
	}
	return shares, nil
}
