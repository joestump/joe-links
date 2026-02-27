// Governing: SPEC-0016 REQ "REST API Stats Endpoint", SPEC-0016 REQ "REST API Clicks Endpoint"
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joestump/joe-links/internal/store"
)

// -- GET /api/v1/links/{id}/stats --

func TestStats_Owner_OK(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "stats-owner@example.com", "user")
	token := seedToken(t, env, user.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "stats-link", "https://example.com", user.ID, "Stats", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	// Record a click.
	err = env.ClickStore.RecordClick(ctx, store.ClickEvent{
		LinkID: link.ID, UserID: user.ID, IPHash: "h1", UserAgent: "Test/1", Referrer: "https://ref.com",
	})
	if err != nil {
		t.Fatalf("record click: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID+"/stats", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		LinkID  string `json:"link_id"`
		Total   int64  `json:"total"`
		Last7d  int64  `json:"last_7d"`
		Last30d int64  `json:"last_30d"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.LinkID != link.ID {
		t.Errorf("link_id = %q, want %q", resp.LinkID, link.ID)
	}
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}
	if resp.Last7d != 1 {
		t.Errorf("last_7d = %d, want 1", resp.Last7d)
	}
	if resp.Last30d != 1 {
		t.Errorf("last_30d = %d, want 1", resp.Last30d)
	}
}

func TestStats_NonOwner_Forbidden(t *testing.T) {
	env := newTestEnv(t)
	owner := seedUser(t, env, "stats-owner2@example.com", "user")
	other := seedUser(t, env, "stats-other@example.com", "user")
	otherToken := seedToken(t, env, other.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "stats-forbidden", "https://example.com", owner.ID, "", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID+"/stats", nil)
	authRequest(req, otherToken)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestStats_Unauthenticated(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest("GET", "/links/some-id/stats", nil)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStats_NotFound(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "stats-nf@example.com", "user")
	token := seedToken(t, env, user.ID)

	req := httptest.NewRequest("GET", "/links/nonexistent-id/stats", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// -- GET /api/v1/links/{id}/clicks --

func TestClicks_OK_Structure(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "clicks-owner@example.com", "user")
	token := seedToken(t, env, user.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "clicks-link", "https://example.com", user.ID, "Clicks", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	err = env.ClickStore.RecordClick(ctx, store.ClickEvent{
		LinkID: link.ID, UserID: user.ID, IPHash: "h1", UserAgent: "Test/1", Referrer: "https://ref.com",
	})
	if err != nil {
		t.Fatalf("record click: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID+"/clicks", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Clicks []struct {
			ClickedAt time.Time `json:"clicked_at"`
			Referrer  *string   `json:"referrer"`
			User      *struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"user"`
		} `json:"clicks"`
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Clicks) != 1 {
		t.Fatalf("len(clicks) = %d, want 1", len(resp.Clicks))
	}
	c := resp.Clicks[0]
	if c.Referrer == nil {
		t.Error("referrer should not be nil for click with referrer")
	} else if *c.Referrer != "https://ref.com" {
		t.Errorf("referrer = %q, want %q", *c.Referrer, "https://ref.com")
	}
	if c.User == nil {
		t.Fatal("user should not be nil for authenticated click")
	}
	if c.User.ID != user.ID {
		t.Errorf("user.id = %q, want %q", c.User.ID, user.ID)
	}
	if resp.NextCursor != nil {
		t.Errorf("next_cursor should be nil when all results fit, got %q", *resp.NextCursor)
	}
}

func TestClicks_DefaultLimit(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "clicks-limit@example.com", "user")
	token := seedToken(t, env, user.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "clicks-dlimit", "https://example.com", user.ID, "", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	// Insert 3 clicks.
	for i := 0; i < 3; i++ {
		err = env.ClickStore.RecordClick(ctx, store.ClickEvent{
			LinkID: link.ID, UserID: "", IPHash: "h", UserAgent: "", Referrer: "",
		})
		if err != nil {
			t.Fatalf("record click %d: %v", i, err)
		}
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID+"/clicks", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Clicks     []json.RawMessage `json:"clicks"`
		NextCursor *string           `json:"next_cursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Default limit is 50, we only have 3 clicks so all should be returned.
	if len(resp.Clicks) != 3 {
		t.Errorf("len(clicks) = %d, want 3", len(resp.Clicks))
	}
	if resp.NextCursor != nil {
		t.Errorf("next_cursor should be nil, got %q", *resp.NextCursor)
	}
}

func TestClicks_CustomLimit(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "clicks-climit@example.com", "user")
	token := seedToken(t, env, user.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "clicks-climit", "https://example.com", user.ID, "", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	for i := 0; i < 5; i++ {
		err = env.ClickStore.RecordClick(ctx, store.ClickEvent{
			LinkID: link.ID, UserID: "", IPHash: "h", UserAgent: "", Referrer: "",
		})
		if err != nil {
			t.Fatalf("record click %d: %v", i, err)
		}
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID+"/clicks?limit=2", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Clicks     []json.RawMessage `json:"clicks"`
		NextCursor *string           `json:"next_cursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Clicks) != 2 {
		t.Errorf("len(clicks) = %d, want 2", len(resp.Clicks))
	}
}

func TestClicks_NextCursor(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "clicks-cursor@example.com", "user")
	token := seedToken(t, env, user.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "clicks-cursor", "https://example.com", user.ID, "", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	// Insert 3 clicks.
	for i := 0; i < 3; i++ {
		err = env.ClickStore.RecordClick(ctx, store.ClickEvent{
			LinkID: link.ID, UserID: "", IPHash: "h", UserAgent: "", Referrer: "",
		})
		if err != nil {
			t.Fatalf("record click %d: %v", i, err)
		}
	}

	// Request with limit=2 should return next_cursor.
	req := httptest.NewRequest("GET", "/links/"+link.ID+"/clicks?limit=2", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Clicks     []json.RawMessage `json:"clicks"`
		NextCursor *string           `json:"next_cursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Clicks) != 2 {
		t.Errorf("len(clicks) = %d, want 2", len(resp.Clicks))
	}
	if resp.NextCursor == nil {
		t.Fatal("next_cursor should be present when more results exist")
	}

	// Paginate with the cursor — should return the remaining click(s) with no next_cursor.
	req2 := httptest.NewRequest("GET", "/links/"+link.ID+"/clicks?limit=2&before="+*resp.NextCursor, nil)
	authRequest(req2, token)
	rec2 := httptest.NewRecorder()
	env.Router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("page 2 status = %d, want %d; body: %s", rec2.Code, http.StatusOK, rec2.Body.String())
	}

	var resp2 struct {
		Clicks     []json.RawMessage `json:"clicks"`
		NextCursor *string           `json:"next_cursor"`
	}
	if err := json.NewDecoder(rec2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode page 2: %v", err)
	}
	if len(resp2.Clicks) != 1 {
		t.Errorf("page 2 len(clicks) = %d, want 1", len(resp2.Clicks))
	}
	if resp2.NextCursor != nil {
		t.Errorf("page 2 next_cursor should be nil on last page, got %q", *resp2.NextCursor)
	}
}

func TestClicks_InvalidBefore_BadRequest(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "clicks-bad-before@example.com", "user")
	token := seedToken(t, env, user.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "clicks-bad-before", "https://example.com", user.ID, "", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID+"/clicks?before=not-a-timestamp", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestClicks_AnonymousClick_UserNull(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "clicks-anon@example.com", "user")
	token := seedToken(t, env, user.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "clicks-anon", "https://example.com", user.ID, "", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	// Record an anonymous click (no user ID).
	err = env.ClickStore.RecordClick(ctx, store.ClickEvent{
		LinkID: link.ID, UserID: "", IPHash: "anon", UserAgent: "", Referrer: "",
	})
	if err != nil {
		t.Fatalf("record click: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID+"/clicks", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Clicks []struct {
			User *struct {
				ID string `json:"id"`
			} `json:"user"`
			Referrer *string `json:"referrer"`
		} `json:"clicks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Clicks) != 1 {
		t.Fatalf("len(clicks) = %d, want 1", len(resp.Clicks))
	}
	if resp.Clicks[0].User != nil {
		t.Error("user should be null for anonymous click")
	}
	if resp.Clicks[0].Referrer != nil {
		t.Error("referrer should be null when empty")
	}
}

func TestClicks_NonOwner_Forbidden(t *testing.T) {
	env := newTestEnv(t)
	owner := seedUser(t, env, "clicks-own@example.com", "user")
	other := seedUser(t, env, "clicks-oth@example.com", "user")
	otherToken := seedToken(t, env, other.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "clicks-forbidden", "https://example.com", owner.ID, "", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID+"/clicks", nil)
	authRequest(req, otherToken)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestClicks_Unauthenticated(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest("GET", "/links/some-id/clicks", nil)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestClicks_NotFound(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "clicks-nf@example.com", "user")
	token := seedToken(t, env, user.ID)

	req := httptest.NewRequest("GET", "/links/nonexistent-id/clicks", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
