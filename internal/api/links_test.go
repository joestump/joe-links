package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joestump/joe-links/internal/api"
)

func TestLinks_List_OK(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	// Create a link so the list isn't empty.
	_, err := env.LinkStore.Create(context.Background(), "test-link", "https://example.com", user.ID, "Test", "", "")
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	req := httptest.NewRequest("GET", "/links", nil)
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
		t.Errorf("len(links) = %d, want 1", len(resp.Links))
	}
}

func TestLinks_List_Unauthenticated(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest("GET", "/links", nil)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLinks_Create_Created(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	body := `{"slug":"my-new-link","url":"https://example.com","title":"New Link","description":"A new link"}`
	req := httptest.NewRequest("POST", "/links", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp api.LinkResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != "my-new-link" {
		t.Errorf("slug = %q, want %q", resp.Slug, "my-new-link")
	}
	if resp.URL != "https://example.com" {
		t.Errorf("url = %q, want %q", resp.URL, "https://example.com")
	}
	if len(resp.Owners) != 1 {
		t.Errorf("len(owners) = %d, want 1", len(resp.Owners))
	}
}

func TestLinks_Create_DuplicateSlug(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	// Create first link.
	_, err := env.LinkStore.Create(context.Background(), "dup-slug", "https://a.com", user.ID, "", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	body := `{"slug":"dup-slug","url":"https://b.com"}`
	req := httptest.NewRequest("POST", "/links", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestLinks_Create_Unauthenticated(t *testing.T) {
	env := newTestEnv(t)
	body := `{"slug":"no-auth","url":"https://example.com"}`
	req := httptest.NewRequest("POST", "/links", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLinks_Get_Found(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	link, err := env.LinkStore.Create(context.Background(), "get-me", "https://example.com", user.ID, "Get Me", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID, nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.LinkResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != "get-me" {
		t.Errorf("slug = %q, want %q", resp.Slug, "get-me")
	}
}

func TestLinks_Get_NotFound(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	req := httptest.NewRequest("GET", "/links/nonexistent-id", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestLinks_Get_Forbidden_NotOwner(t *testing.T) {
	env := newTestEnv(t)
	owner := seedUser(t, env, "owner@example.com", "user")
	other := seedUser(t, env, "other@example.com", "user")
	otherToken := seedToken(t, env, other.ID)

	link, err := env.LinkStore.Create(context.Background(), "private-link", "https://example.com", owner.ID, "", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID, nil)
	authRequest(req, otherToken)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLinks_Update_OK(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	link, err := env.LinkStore.Create(context.Background(), "update-me", "https://old.com", user.ID, "Old", "Old desc", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	body := `{"url":"https://new.com","title":"New","description":"New desc"}`
	req := httptest.NewRequest("PUT", "/links/"+link.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.LinkResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.URL != "https://new.com" {
		t.Errorf("url = %q, want %q", resp.URL, "https://new.com")
	}
}

func TestLinks_Update_Forbidden_NotOwner(t *testing.T) {
	env := newTestEnv(t)
	owner := seedUser(t, env, "owner@example.com", "user")
	other := seedUser(t, env, "other@example.com", "user")
	otherToken := seedToken(t, env, other.ID)

	link, err := env.LinkStore.Create(context.Background(), "no-update", "https://example.com", owner.ID, "", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	body := `{"url":"https://hacked.com"}`
	req := httptest.NewRequest("PUT", "/links/"+link.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, otherToken)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLinks_Delete_NoContent(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	link, err := env.LinkStore.Create(context.Background(), "delete-me", "https://example.com", user.ID, "", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/links/"+link.ID, nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestLinks_Delete_NotFound(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	req := httptest.NewRequest("DELETE", "/links/nonexistent-id", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestLinks_Delete_Unauthenticated(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest("DELETE", "/links/some-id", nil)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// Governing: SPEC-0009 REQ "API Representation", ADR-0013
func TestLinks_Create_VariableURL_Passthrough(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	body := `{"slug":"github","url":"https://github.com/$username","title":"GitHub"}`
	req := httptest.NewRequest("POST", "/links", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp api.LinkResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.URL != "https://github.com/$username" {
		t.Errorf("url = %q, want %q — API must return template as-is", resp.URL, "https://github.com/$username")
	}
}

// Governing: SPEC-0009 REQ "Variable Placeholder Syntax", ADR-0013
func TestLinks_Create_DuplicateVariable_Rejected(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	body := `{"slug":"dup-var","url":"https://example.com/$foo/$foo"}`
	req := httptest.NewRequest("POST", "/links", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// Governing: SPEC-0009 REQ "API Representation", ADR-0013
func TestLinks_Get_VariableURL_Passthrough(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	link, err := env.LinkStore.Create(context.Background(), "var-link", "https://example.com/$query/$page", user.ID, "Var Link", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	req := httptest.NewRequest("GET", "/links/"+link.ID, nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.LinkResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.URL != "https://example.com/$query/$page" {
		t.Errorf("url = %q, want %q — API must return template as-is", resp.URL, "https://example.com/$query/$page")
	}
}
