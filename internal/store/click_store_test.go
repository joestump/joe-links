// Governing: SPEC-0016 REQ "Click Data Schema", SPEC-0016 REQ "Click Recording"
package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

// newClickTestEnv creates a test DB with a user, link, and click store.
func newClickTestEnv(t *testing.T) (*store.ClickStore, *store.LinkStore, *store.UserStore, string, string) {
	t.Helper()
	db := testutil.NewTestDB(t)

	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	cs := store.NewClickStore(db)

	ctx := context.Background()
	u, err := us.Upsert(ctx, "test", "sub1", "click-test@example.com", "Click Tester", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	link, err := ls.Create(ctx, "click-link", "https://example.com", u.ID, "Test Link", "", "")
	if err != nil {
		t.Fatalf("seed link: %v", err)
	}

	return cs, ls, us, u.ID, link.ID
}

func TestRecordClick_HappyPath(t *testing.T) {
	cs, _, _, userID, linkID := newClickTestEnv(t)
	ctx := context.Background()

	err := cs.RecordClick(ctx, store.ClickEvent{
		LinkID:    linkID,
		UserID:    userID,
		IPHash:    "abc123",
		UserAgent: "TestBrowser/1.0",
		Referrer:  "https://referrer.com",
	})
	if err != nil {
		t.Fatalf("RecordClick: %v", err)
	}

	stats, err := cs.GetClickStats(ctx, linkID)
	if err != nil {
		t.Fatalf("GetClickStats: %v", err)
	}
	if stats.Total != 1 {
		t.Errorf("total = %d, want 1", stats.Total)
	}
}

func TestRecordClick_Anonymous(t *testing.T) {
	cs, _, _, _, linkID := newClickTestEnv(t)
	ctx := context.Background()

	err := cs.RecordClick(ctx, store.ClickEvent{
		LinkID:    linkID,
		UserID:    "", // anonymous
		IPHash:    "anon-hash",
		UserAgent: "Bot/1.0",
		Referrer:  "",
	})
	if err != nil {
		t.Fatalf("RecordClick anonymous: %v", err)
	}

	clicks, err := cs.ListRecentClicks(ctx, linkID, 10)
	if err != nil {
		t.Fatalf("ListRecentClicks: %v", err)
	}
	if len(clicks) != 1 {
		t.Fatalf("len(clicks) = %d, want 1", len(clicks))
	}
	if clicks[0].UserID != "" {
		t.Errorf("user_id = %q, want empty for anonymous click", clicks[0].UserID)
	}
	if clicks[0].DisplayName != "" {
		t.Errorf("display_name = %q, want empty for anonymous click", clicks[0].DisplayName)
	}
}

func TestRecordClick_Authenticated(t *testing.T) {
	cs, _, _, userID, linkID := newClickTestEnv(t)
	ctx := context.Background()

	err := cs.RecordClick(ctx, store.ClickEvent{
		LinkID:    linkID,
		UserID:    userID,
		IPHash:    "auth-hash",
		UserAgent: "Chrome/100",
		Referrer:  "",
	})
	if err != nil {
		t.Fatalf("RecordClick authenticated: %v", err)
	}

	clicks, err := cs.ListRecentClicks(ctx, linkID, 10)
	if err != nil {
		t.Fatalf("ListRecentClicks: %v", err)
	}
	if len(clicks) != 1 {
		t.Fatalf("len(clicks) = %d, want 1", len(clicks))
	}
	if clicks[0].UserID != userID {
		t.Errorf("user_id = %q, want %q", clicks[0].UserID, userID)
	}
	if clicks[0].DisplayName != "Click Tester" {
		t.Errorf("display_name = %q, want %q", clicks[0].DisplayName, "Click Tester")
	}
}

func TestRecordClick_UserAgentTruncated(t *testing.T) {
	cs, _, _, _, linkID := newClickTestEnv(t)
	ctx := context.Background()

	longUA := strings.Repeat("A", 600)
	err := cs.RecordClick(ctx, store.ClickEvent{
		LinkID:    linkID,
		UserID:    "",
		IPHash:    "ua-hash",
		UserAgent: longUA,
		Referrer:  "",
	})
	if err != nil {
		t.Fatalf("RecordClick with long UA: %v", err)
	}

	// Verify it succeeded (truncation is internal; we just confirm no error and row exists).
	stats, err := cs.GetClickStats(ctx, linkID)
	if err != nil {
		t.Fatalf("GetClickStats: %v", err)
	}
	if stats.Total != 1 {
		t.Errorf("total = %d, want 1", stats.Total)
	}
}

func TestRecordClick_ReferrerTruncated(t *testing.T) {
	cs, _, _, _, linkID := newClickTestEnv(t)
	ctx := context.Background()

	longRef := "https://example.com/" + strings.Repeat("x", 2100)
	err := cs.RecordClick(ctx, store.ClickEvent{
		LinkID:    linkID,
		UserID:    "",
		IPHash:    "ref-hash",
		UserAgent: "Bot/1.0",
		Referrer:  longRef,
	})
	if err != nil {
		t.Fatalf("RecordClick with long referrer: %v", err)
	}

	stats, err := cs.GetClickStats(ctx, linkID)
	if err != nil {
		t.Fatalf("GetClickStats: %v", err)
	}
	if stats.Total != 1 {
		t.Errorf("total = %d, want 1", stats.Total)
	}
}

func TestGetClickStats_NoClicks(t *testing.T) {
	cs, _, _, _, linkID := newClickTestEnv(t)
	ctx := context.Background()

	stats, err := cs.GetClickStats(ctx, linkID)
	if err != nil {
		t.Fatalf("GetClickStats: %v", err)
	}
	if stats.Total != 0 {
		t.Errorf("total = %d, want 0", stats.Total)
	}
	if stats.Last7d != 0 {
		t.Errorf("last_7d = %d, want 0", stats.Last7d)
	}
	if stats.Last30d != 0 {
		t.Errorf("last_30d = %d, want 0", stats.Last30d)
	}
}

func TestGetClickStats_NonExistentLink(t *testing.T) {
	db := testutil.NewTestDB(t)
	cs := store.NewClickStore(db)
	ctx := context.Background()

	stats, err := cs.GetClickStats(ctx, "nonexistent-link-id")
	if err != nil {
		t.Fatalf("GetClickStats for nonexistent link: %v", err)
	}
	if stats.Total != 0 {
		t.Errorf("total = %d, want 0", stats.Total)
	}
	if stats.Last7d != 0 {
		t.Errorf("last_7d = %d, want 0", stats.Last7d)
	}
	if stats.Last30d != 0 {
		t.Errorf("last_30d = %d, want 0", stats.Last30d)
	}
}

func TestGetClickStats_TimeWindows(t *testing.T) {
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	cs := store.NewClickStore(db)
	ctx := context.Background()

	u, err := us.Upsert(ctx, "test", "sub-tw", "tw@example.com", "TW User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	link, err := ls.Create(ctx, "tw-link", "https://example.com", u.ID, "", "", "")
	if err != nil {
		t.Fatalf("seed link: %v", err)
	}

	now := time.Now().UTC()

	// Insert clicks at specific times using raw SQL:
	// 1 click: 2 days ago (within 7d and 30d)
	// 1 click: 10 days ago (within 30d but not 7d)
	// 1 click: 60 days ago (outside both windows)
	times := []time.Time{
		now.AddDate(0, 0, -2),  // within 7d
		now.AddDate(0, 0, -10), // within 30d only
		now.AddDate(0, 0, -60), // outside both
	}

	for i, ts := range times {
		_, err := db.ExecContext(ctx,
			db.Rebind(`INSERT INTO link_clicks (id, link_id, user_id, ip_hash, user_agent, referrer, clicked_at) VALUES (?, ?, NULL, ?, '', '', ?)`),
			"click-tw-"+string(rune('a'+i)), link.ID, "hash", ts)
		if err != nil {
			t.Fatalf("insert click %d: %v", i, err)
		}
	}

	stats, err := cs.GetClickStats(ctx, link.ID)
	if err != nil {
		t.Fatalf("GetClickStats: %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("total = %d, want 3", stats.Total)
	}
	if stats.Last7d != 1 {
		t.Errorf("last_7d = %d, want 1", stats.Last7d)
	}
	if stats.Last30d != 2 {
		t.Errorf("last_30d = %d, want 2", stats.Last30d)
	}
}

func TestListRecentClicks_NewestFirst(t *testing.T) {
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	cs := store.NewClickStore(db)
	ctx := context.Background()

	u, err := us.Upsert(ctx, "test", "sub-nf", "nf@example.com", "NF User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	link, err := ls.Create(ctx, "nf-link", "https://example.com", u.ID, "", "", "")
	if err != nil {
		t.Fatalf("seed link: %v", err)
	}

	now := time.Now().UTC()
	// Insert 3 clicks at different times.
	for i := 0; i < 3; i++ {
		ts := now.Add(time.Duration(-i) * time.Hour)
		_, err := db.ExecContext(ctx,
			db.Rebind(`INSERT INTO link_clicks (id, link_id, user_id, ip_hash, user_agent, referrer, clicked_at) VALUES (?, ?, NULL, ?, '', '', ?)`),
			"click-nf-"+string(rune('a'+i)), link.ID, "hash", ts)
		if err != nil {
			t.Fatalf("insert click %d: %v", i, err)
		}
	}

	clicks, err := cs.ListRecentClicks(ctx, link.ID, 10)
	if err != nil {
		t.Fatalf("ListRecentClicks: %v", err)
	}
	if len(clicks) != 3 {
		t.Fatalf("len = %d, want 3", len(clicks))
	}
	// Newest first: click-nf-a (now) > click-nf-b (now-1h) > click-nf-c (now-2h)
	if !clicks[0].ClickedAt.After(clicks[1].ClickedAt) {
		t.Errorf("clicks not in newest-first order: %v <= %v", clicks[0].ClickedAt, clicks[1].ClickedAt)
	}
	if !clicks[1].ClickedAt.After(clicks[2].ClickedAt) {
		t.Errorf("clicks not in newest-first order: %v <= %v", clicks[1].ClickedAt, clicks[2].ClickedAt)
	}
}

func TestListRecentClicks_RespectsLimit(t *testing.T) {
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	cs := store.NewClickStore(db)
	ctx := context.Background()

	u, err := us.Upsert(ctx, "test", "sub-lim", "lim@example.com", "Lim User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	link, err := ls.Create(ctx, "lim-link", "https://example.com", u.ID, "", "", "")
	if err != nil {
		t.Fatalf("seed link: %v", err)
	}

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ts := now.Add(time.Duration(-i) * time.Minute)
		_, err := db.ExecContext(ctx,
			db.Rebind(`INSERT INTO link_clicks (id, link_id, user_id, ip_hash, user_agent, referrer, clicked_at) VALUES (?, ?, NULL, ?, '', '', ?)`),
			"click-lim-"+string(rune('a'+i)), link.ID, "hash", ts)
		if err != nil {
			t.Fatalf("insert click %d: %v", i, err)
		}
	}

	clicks, err := cs.ListRecentClicks(ctx, link.ID, 2)
	if err != nil {
		t.Fatalf("ListRecentClicks: %v", err)
	}
	if len(clicks) != 2 {
		t.Errorf("len = %d, want 2", len(clicks))
	}
}

func TestListRecentClicks_JoinsUserDisplayName(t *testing.T) {
	cs, _, _, userID, linkID := newClickTestEnv(t)
	ctx := context.Background()

	err := cs.RecordClick(ctx, store.ClickEvent{
		LinkID: linkID, UserID: userID, IPHash: "h1", UserAgent: "", Referrer: "",
	})
	if err != nil {
		t.Fatalf("RecordClick: %v", err)
	}

	clicks, err := cs.ListRecentClicks(ctx, linkID, 10)
	if err != nil {
		t.Fatalf("ListRecentClicks: %v", err)
	}
	if len(clicks) != 1 {
		t.Fatalf("len = %d, want 1", len(clicks))
	}
	if clicks[0].DisplayName != "Click Tester" {
		t.Errorf("display_name = %q, want %q", clicks[0].DisplayName, "Click Tester")
	}
}

func TestListRecentClicks_EmptyForNewLink(t *testing.T) {
	cs, _, _, _, linkID := newClickTestEnv(t)
	ctx := context.Background()

	clicks, err := cs.ListRecentClicks(ctx, linkID, 10)
	if err != nil {
		t.Fatalf("ListRecentClicks: %v", err)
	}
	if len(clicks) != 0 {
		t.Errorf("len = %d, want 0", len(clicks))
	}
}

func TestListRecentClicksBefore_ZeroBefore(t *testing.T) {
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	cs := store.NewClickStore(db)
	ctx := context.Background()

	u, err := us.Upsert(ctx, "test", "sub-zb", "zb@example.com", "ZB User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	link, err := ls.Create(ctx, "zb-link", "https://example.com", u.ID, "", "", "")
	if err != nil {
		t.Fatalf("seed link: %v", err)
	}

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		ts := now.Add(time.Duration(-i) * time.Minute)
		_, err := db.ExecContext(ctx,
			db.Rebind(`INSERT INTO link_clicks (id, link_id, user_id, ip_hash, user_agent, referrer, clicked_at) VALUES (?, ?, NULL, ?, '', '', ?)`),
			"click-zb-"+string(rune('a'+i)), link.ID, "hash", ts)
		if err != nil {
			t.Fatalf("insert click %d: %v", i, err)
		}
	}

	// Zero time => return from most recent.
	clicks, err := cs.ListRecentClicksBefore(ctx, link.ID, time.Time{}, 10)
	if err != nil {
		t.Fatalf("ListRecentClicksBefore: %v", err)
	}
	if len(clicks) != 3 {
		t.Errorf("len = %d, want 3", len(clicks))
	}
}

func TestListRecentClicksBefore_CursorFilters(t *testing.T) {
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	cs := store.NewClickStore(db)
	ctx := context.Background()

	u, err := us.Upsert(ctx, "test", "sub-cf", "cf@example.com", "CF User", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	link, err := ls.Create(ctx, "cf-link", "https://example.com", u.ID, "", "", "")
	if err != nil {
		t.Fatalf("seed link: %v", err)
	}

	now := time.Now().UTC()
	// Insert 3 clicks with distinct timestamps.
	t1 := now.Add(-3 * time.Hour)
	t2 := now.Add(-2 * time.Hour)
	t3 := now.Add(-1 * time.Hour)

	for i, ts := range []time.Time{t1, t2, t3} {
		_, err := db.ExecContext(ctx,
			db.Rebind(`INSERT INTO link_clicks (id, link_id, user_id, ip_hash, user_agent, referrer, clicked_at) VALUES (?, ?, NULL, ?, '', '', ?)`),
			"click-cf-"+string(rune('a'+i)), link.ID, "hash", ts)
		if err != nil {
			t.Fatalf("insert click %d: %v", i, err)
		}
	}

	// Query with before = t3 should return only t2 and t1.
	clicks, err := cs.ListRecentClicksBefore(ctx, link.ID, t3, 10)
	if err != nil {
		t.Fatalf("ListRecentClicksBefore: %v", err)
	}
	if len(clicks) != 2 {
		t.Fatalf("len = %d, want 2", len(clicks))
	}
}

func TestListRecentClicksBefore_EmptyResult(t *testing.T) {
	cs, _, _, _, linkID := newClickTestEnv(t)
	ctx := context.Background()

	clicks, err := cs.ListRecentClicksBefore(ctx, linkID, time.Now(), 10)
	if err != nil {
		t.Fatalf("ListRecentClicksBefore: %v", err)
	}
	if len(clicks) != 0 {
		t.Errorf("len = %d, want 0", len(clicks))
	}
}

func TestHashIP_SameIPSameDay(t *testing.T) {
	h1 := store.HashIP("192.168.1.1")
	h2 := store.HashIP("192.168.1.1")
	if h1 != h2 {
		t.Errorf("same IP produced different hashes: %q vs %q", h1, h2)
	}
}

func TestHashIP_DifferentIPs(t *testing.T) {
	h1 := store.HashIP("192.168.1.1")
	h2 := store.HashIP("10.0.0.1")
	if h1 == h2 {
		t.Errorf("different IPs produced same hash: %q", h1)
	}
}
