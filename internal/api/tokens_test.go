package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joestump/joe-links/internal/api"
	"github.com/joestump/joe-links/internal/auth"
)

func TestTokens_List_OK(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	// Create an additional token for the user so there are at least 2.
	_, hash2, _ := auth.GenerateToken()
	_, err := env.TokenStore.Create(context.Background(), user.ID, "second-token", hash2, nil)
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}

	req := httptest.NewRequest("GET", "/tokens", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp api.TokenListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Tokens) < 2 {
		t.Errorf("len(tokens) = %d, want >= 2", len(resp.Tokens))
	}
}

func TestTokens_List_Unauthenticated(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest("GET", "/tokens", nil)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestTokens_Create_Created(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	body := `{"name":"my-api-token"}`
	req := httptest.NewRequest("POST", "/tokens", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp api.TokenCreatedResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected plaintext token in response")
	}
	if resp.Name != "my-api-token" {
		t.Errorf("name = %q, want %q", resp.Name, "my-api-token")
	}
	if len(resp.Token) < 10 || resp.Token[:3] != "jl_" {
		t.Errorf("token = %q, want jl_ prefix", resp.Token)
	}
}

func TestTokens_Create_MissingName(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	body := `{}`
	req := httptest.NewRequest("POST", "/tokens", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestTokens_Revoke_NoContent(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	// Create a token to revoke.
	_, hash, _ := auth.GenerateToken()
	rec2, err := env.TokenStore.Create(context.Background(), user.ID, "revoke-me", hash, nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/tokens/"+rec2.ID, nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestTokens_Revoke_NotFound(t *testing.T) {
	env := newTestEnv(t)
	user := seedUser(t, env, "alice@example.com", "user")
	token := seedToken(t, env, user.ID)

	req := httptest.NewRequest("DELETE", "/tokens/nonexistent-id", nil)
	authRequest(req, token)
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
