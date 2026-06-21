package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joestump/joe-links/internal/api"
)

// Governing: SPEC-0005 REQ "API Response Structures" — GET /tags/{slug}/links must
// return the same link shape as other endpoints, including owners, tags, and visibility.
func TestTags_ListLinks_FullResponseShape(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)
	ctx := context.Background()

	link, err := env.LinkStore.Create(ctx, "docs", "https://example.com", user.ID, "Docs", "team docs", "private")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	if err := env.LinkStore.SetTags(ctx, link.ID, []string{"work"}); err != nil {
		t.Fatalf("set tags: %v", err)
	}

	req := httptest.NewRequest("GET", "/tags/work/links", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.LinkListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(resp.Links))
	}

	got := resp.Links[0]
	if got.Visibility != "private" {
		t.Errorf("visibility = %q, want %q", got.Visibility, "private")
	}
	if len(got.Tags) != 1 || got.Tags[0] != "work" {
		t.Errorf("tags = %v, want [work]", got.Tags)
	}
	if len(got.Owners) != 1 {
		t.Fatalf("len(owners) = %d, want 1", len(got.Owners))
	}
	if got.Owners[0].Email != "alice@example.com" || !got.Owners[0].IsPrimary {
		t.Errorf("owner = %+v, want primary alice@example.com", got.Owners[0])
	}
}
