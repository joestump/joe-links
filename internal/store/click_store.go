// Governing: SPEC-0016 REQ "Click Data Schema", REQ "Click Recording", ADR-0016
package store

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ClickEvent represents a single click to be recorded.
type ClickEvent struct {
	LinkID    string
	UserID    string // empty string = anonymous
	IPHash    string // caller computes this
	UserAgent string
	Referrer  string
}

// ClickStats holds aggregate click counts for a link.
type ClickStats struct {
	Total  int64
	Last7d int64
	Last30d int64
}

// RecentClick represents a single click with optional user info.
type RecentClick struct {
	ClickedAt   time.Time `db:"clicked_at"`
	Referrer    string    `db:"referrer"`
	UserID      string    `db:"user_id"`
	DisplayName string    `db:"display_name"`
}

// ClickStore is the sqlx-backed store for click tracking operations.
type ClickStore struct {
	db *sqlx.DB
}

// NewClickStore creates a new ClickStore.
func NewClickStore(db *sqlx.DB) *ClickStore {
	return &ClickStore{db: db}
}

// q rebinds ? placeholders to the driver's native format.
func (s *ClickStore) q(query string) string { return s.db.Rebind(query) }

// RecordClick inserts a click event row.
// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
func (s *ClickStore) RecordClick(ctx context.Context, e ClickEvent) error {
	id := uuid.New().String()
	now := time.Now().UTC()

	// Truncate user_agent to 512 chars, referrer to 2048.
	ua := e.UserAgent
	if len(ua) > 512 {
		ua = ua[:512]
	}
	ref := e.Referrer
	if len(ref) > 2048 {
		ref = ref[:2048]
	}

	var userID interface{}
	if e.UserID != "" {
		userID = e.UserID
	}

	_, err := s.db.ExecContext(ctx, s.q(`
		INSERT INTO link_clicks (id, link_id, user_id, ip_hash, user_agent, referrer, clicked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`), id, e.LinkID, userID, e.IPHash, ua, ref, now)
	return err
}

// GetClickStats returns total, 7d, and 30d click counts for a link.
// Governing: SPEC-0016 REQ "Click Data Schema", ADR-0016
func (s *ClickStore) GetClickStats(ctx context.Context, linkID string) (ClickStats, error) {
	var stats ClickStats
	now := time.Now().UTC()
	since7d := now.AddDate(0, 0, -7)
	since30d := now.AddDate(0, 0, -30)

	err := s.db.GetContext(ctx, &stats.Total,
		s.q(`SELECT COUNT(*) FROM link_clicks WHERE link_id = ?`), linkID)
	if err != nil {
		return stats, err
	}

	err = s.db.GetContext(ctx, &stats.Last7d,
		s.q(`SELECT COUNT(*) FROM link_clicks WHERE link_id = ? AND clicked_at >= ?`), linkID, since7d)
	if err != nil {
		return stats, err
	}

	err = s.db.GetContext(ctx, &stats.Last30d,
		s.q(`SELECT COUNT(*) FROM link_clicks WHERE link_id = ? AND clicked_at >= ?`), linkID, since30d)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

// ListRecentClicks returns the most recent N clicks for a link, joining users for display_name.
// Governing: SPEC-0016 REQ "Click Data Schema", ADR-0016
func (s *ClickStore) ListRecentClicks(ctx context.Context, linkID string, limit int) ([]RecentClick, error) {
	var clicks []RecentClick
	err := s.db.SelectContext(ctx, &clicks, s.q(`
		SELECT c.clicked_at,
		       COALESCE(c.referrer, '') AS referrer,
		       COALESCE(c.user_id, '') AS user_id,
		       COALESCE(u.display_name, '') AS display_name
		FROM link_clicks c
		LEFT JOIN users u ON u.id = c.user_id
		WHERE c.link_id = ?
		ORDER BY c.clicked_at DESC
		LIMIT ?
	`), linkID, limit)
	if err != nil {
		return nil, err
	}
	return clicks, nil
}

// HashIP computes SHA-256(ip + ":" + YYYYMMDD_UTC) for the current day.
// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
func HashIP(ip string) string {
	salt := time.Now().UTC().Format("20060102")
	h := sha256.Sum256([]byte(ip + ":" + salt))
	return fmt.Sprintf("%x", h)
}
