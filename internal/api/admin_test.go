package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joestump/joe-links/internal/api"
)

func TestAdmin_ListUsers_Forbidden_NonAdmin(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	req := httptest.NewRequest("GET", "/admin/users", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestAdmin_ListUsers_OK_Admin(t *testing.T) {
	env := newTestEnv(t)
	admin := seedUser(t, env, "admin@example.com", "admin")
	token := seedToken(t, env, admin.ID)

	// Seed a second user so there are at least 2.
	seedUser(t, env, "other@example.com", "user")

	req := httptest.NewRequest("GET", "/admin/users", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.UserListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Users) < 2 {
		t.Errorf("len(users) = %d, want >= 2", len(resp.Users))
	}
}

func TestAdmin_UpdateRole_OK(t *testing.T) {
	env := newTestEnv(t)
	admin := seedUser(t, env, "admin@example.com", "admin")
	token := seedToken(t, env, admin.ID)
	target := seedUser(t, env, "target@example.com", "user")

	body := `{"role":"admin"}`
	req := httptest.NewRequest("PUT", "/admin/users/"+target.ID+"/role", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Role != "admin" {
		t.Errorf("role = %q, want %q", resp.Role, "admin")
	}
}

func TestAdmin_UpdateRole_InvalidRole(t *testing.T) {
	env := newTestEnv(t)
	admin := seedUser(t, env, "admin@example.com", "admin")
	token := seedToken(t, env, admin.ID)
	target := seedUser(t, env, "target@example.com", "user")

	body := `{"role":"superadmin"}`
	req := httptest.NewRequest("PUT", "/admin/users/"+target.ID+"/role", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestAdmin_UpdateRole_Forbidden_NonAdmin(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	body := `{"role":"admin"}`
	req := httptest.NewRequest("PUT", "/admin/users/"+user.ID+"/role", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAdmin_ListLinks_Forbidden_NonAdmin(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	req := httptest.NewRequest("GET", "/admin/links", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAdmin_ListLinks_OK_Admin(t *testing.T) {
	env := newTestEnv(t)
	admin := seedUser(t, env, "admin@example.com", "admin")
	token := seedToken(t, env, admin.ID)

	req := httptest.NewRequest("GET", "/admin/links", nil)
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
	// Empty is OK; we just need the endpoint to work.
	if resp.Links == nil {
		t.Error("expected non-nil links array")
	}
}

func TestAdmin_Unauthenticated(t *testing.T) {
	env := newTestEnv(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/admin/users"},
		{"GET", "/admin/links"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rec := httptest.NewRecorder()
			env.Router.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}
