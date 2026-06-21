// Governing: SPEC-0005 REQ "Pagination"
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joestump/joe-links/internal/api"
)

// seedLinks creates n links owned by the user with zero-padded slugs so the
// (slug, id) ordering is deterministic across pages.
func seedLinks(t *testing.T, env *testEnv, userID string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		slug := fmt.Sprintf("link-%04d", i)
		_, err := env.LinkStore.Create(context.Background(), slug, "https://example.com/"+slug, userID, "", "", "")
		if err != nil {
			t.Fatalf("seed link %s: %v", slug, err)
		}
	}
}

func TestLinks_List_DefaultLimit(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)
	seedLinks(t, env, user.ID, 60)

	req := authRequest(httptest.NewRequest("GET", "/links", nil), token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var resp api.LinkListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Links) != 50 {
		t.Errorf("len(links) = %d, want 50 (default limit)", len(resp.Links))
	}
	if resp.NextCursor == nil {
		t.Errorf("next_cursor = nil, want non-nil (more results exist)")
	}
}

func TestLinks_List_LimitCapped(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)
	seedLinks(t, env, user.ID, 250)

	req := authRequest(httptest.NewRequest("GET", "/links?limit=999", nil), token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var resp api.LinkListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Links) != 200 {
		t.Errorf("len(links) = %d, want 200 (capped)", len(resp.Links))
	}
}

func TestLinks_List_InvalidLimitUsesDefault(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)
	seedLinks(t, env, user.ID, 60)

	for _, q := range []string{"limit=abc", "limit=-5", "limit=0"} {
		req := authRequest(httptest.NewRequest("GET", "/links?"+q, nil), token)
		rec := httptest.NewRecorder()
		env.Router.ServeHTTP(rec, req)

		var resp api.LinkListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("%s: decode: %v", q, err)
		}
		if len(resp.Links) != 50 {
			t.Errorf("%s: len(links) = %d, want 50 (default)", q, len(resp.Links))
		}
	}
}

func TestLinks_List_CursorRoundTrip(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)
	const total = 130
	seedLinks(t, env, user.ID, total)

	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		url := "/links?limit=50"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		req := authRequest(httptest.NewRequest("GET", url, nil), token)
		rec := httptest.NewRecorder()
		env.Router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("page %d: status = %d; body: %s", pages, rec.Code, rec.Body.String())
		}
		var resp api.LinkListResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("page %d: decode: %v", pages, err)
		}
		for _, l := range resp.Links {
			if seen[l.ID] {
				t.Fatalf("duplicate link %s across pages", l.Slug)
			}
			seen[l.ID] = true
		}
		pages++
		if pages > 10 {
			t.Fatalf("too many pages, possible infinite loop")
		}
		if resp.NextCursor == nil {
			if len(resp.Links) > 50 {
				t.Fatalf("last page had %d links, want <= 50", len(resp.Links))
			}
			break
		}
		cursor = *resp.NextCursor
	}

	if len(seen) != total {
		t.Errorf("collected %d unique links across pages, want %d", len(seen), total)
	}
	if pages != 3 {
		t.Errorf("pages = %d, want 3 (50+50+30)", pages)
	}
}

func TestTags_List_Pagination(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	// Each link gets a distinct tag so every tag has link_count == 1.
	for i := 0; i < 60; i++ {
		slug := fmt.Sprintf("link-%04d", i)
		link, err := env.LinkStore.Create(context.Background(), slug, "https://example.com/"+slug, user.ID, "", "", "")
		if err != nil {
			t.Fatalf("create link: %v", err)
		}
		if err := env.LinkStore.SetTags(context.Background(), link.ID, []string{fmt.Sprintf("tag-%04d", i)}); err != nil {
			t.Fatalf("set tags: %v", err)
		}
	}

	// Default limit.
	req := authRequest(httptest.NewRequest("GET", "/tags", nil), token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var resp api.TagListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Tags) != 50 {
		t.Errorf("len(tags) = %d, want 50", len(resp.Tags))
	}
	if resp.NextCursor == nil {
		t.Fatalf("next_cursor = nil, want non-nil")
	}

	// Round-trip the cursor for the remaining 10.
	req2 := authRequest(httptest.NewRequest("GET", "/tags?cursor="+*resp.NextCursor, nil), token)
	rec2 := httptest.NewRecorder()
	env.Router.ServeHTTP(rec2, req2)
	var resp2 api.TagListResponse
	if err := json.NewDecoder(rec2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode page 2: %v", err)
	}
	if len(resp2.Tags) != 10 {
		t.Errorf("page 2 len(tags) = %d, want 10", len(resp2.Tags))
	}
	if resp2.NextCursor != nil {
		t.Errorf("page 2 next_cursor = %v, want nil (last page)", *resp2.NextCursor)
	}
}

func TestAdminUsers_List_Pagination(t *testing.T) {
	env := newTestEnv(t)
	admin := seedUser(t, env, "admin@example.com", "admin")
	token := seedToken(t, env, admin.ID)

	// admin is user #1; add 59 more for 60 total.
	for i := 0; i < 59; i++ {
		seedUser(t, env, fmt.Sprintf("user-%04d@example.com", i), "user")
	}

	req := authRequest(httptest.NewRequest("GET", "/admin/users?limit=999", nil), token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var resp api.UserListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// 60 users < 200 cap, so all returned and no next cursor.
	if len(resp.Users) != 60 {
		t.Errorf("len(users) = %d, want 60", len(resp.Users))
	}
	if resp.NextCursor != nil {
		t.Errorf("next_cursor = %v, want nil", *resp.NextCursor)
	}

	// Now page with limit=25 and round-trip.
	seen := map[string]bool{}
	cursor := ""
	pages := 0
	for {
		url := "/admin/users?limit=25"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		r := authRequest(httptest.NewRequest("GET", url, nil), token)
		w := httptest.NewRecorder()
		env.Router.ServeHTTP(w, r)
		var pr api.UserListResponse
		if err := json.NewDecoder(w.Body).Decode(&pr); err != nil {
			t.Fatalf("page %d decode: %v", pages, err)
		}
		for _, u := range pr.Users {
			if seen[u.ID] {
				t.Fatalf("duplicate user %s", u.Email)
			}
			seen[u.ID] = true
		}
		pages++
		if pr.NextCursor == nil {
			break
		}
		cursor = *pr.NextCursor
		if pages > 10 {
			t.Fatalf("infinite loop")
		}
	}
	if len(seen) != 60 {
		t.Errorf("collected %d users, want 60", len(seen))
	}
}

func TestAdminLinks_List_Pagination(t *testing.T) {
	env := newTestEnv(t)
	admin := seedUser(t, env, "admin@example.com", "admin")
	token := seedToken(t, env, admin.ID)
	seedLinks(t, env, admin.ID, 60)

	req := authRequest(httptest.NewRequest("GET", "/admin/links", nil), token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var resp api.LinkListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Links) != 50 {
		t.Errorf("len(links) = %d, want 50", len(resp.Links))
	}
	if resp.NextCursor == nil {
		t.Fatalf("next_cursor = nil, want non-nil")
	}

	req2 := authRequest(httptest.NewRequest("GET", "/admin/links?cursor="+*resp.NextCursor, nil), token)
	rec2 := httptest.NewRecorder()
	env.Router.ServeHTTP(rec2, req2)
	var resp2 api.LinkListResponse
	if err := json.NewDecoder(rec2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode page 2: %v", err)
	}
	if len(resp2.Links) != 10 {
		t.Errorf("page 2 len(links) = %d, want 10", len(resp2.Links))
	}
	if resp2.NextCursor != nil {
		t.Errorf("page 2 next_cursor = %v, want nil", *resp2.NextCursor)
	}
}
