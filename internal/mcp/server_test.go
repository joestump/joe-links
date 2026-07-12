// Governing: ADR-0018, SPEC-0018 REQ "MCP Endpoint", REQ "Bearer Token Authentication"
package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/mcp"
	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

// testEnv wires the MCP handler over real stores on an in-memory database,
// mirroring the /api/v1 test harness.
type testEnv struct {
	Handler    http.Handler
	UserStore  *store.UserStore
	TokenStore *auth.SQLTokenStore
	LinkStore  *store.LinkStore
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db := testutil.NewTestDB(t)

	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	ts := auth.NewSQLTokenStore(db)
	cs := store.NewClickStore(db)

	bearer := auth.NewBearerTokenMiddleware(ts, us)
	h := mcp.NewHandler(mcp.Deps{
		LinkStore:      ls,
		OwnershipStore: owns,
		TagStore:       tags,
		UserStore:      us,
		KeywordStore:   store.NewKeywordStore(db),
		ClickStore:     cs,
	}, bearer)

	return &testEnv{Handler: h, UserStore: us, TokenStore: ts, LinkStore: ls}
}

func seedUserAndToken(t *testing.T, env *testEnv, email string) (*store.User, string) {
	t.Helper()
	ctx := context.Background()
	u, err := env.UserStore.Upsert(ctx, "test", "sub-"+email, email, "Test User", "user")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	plaintext, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if _, err := env.TokenStore.Create(ctx, u.ID, "test-token", hash, nil); err != nil {
		t.Fatalf("create token: %v", err)
	}
	return u, plaintext
}

// rpc builds a JSON-RPC request body for the streamable endpoint.
func rpc(id int, method string, params any) []byte {
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	b, _ := json.Marshal(msg)
	return b
}

func initializeParams() map[string]any {
	return map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test-client", "version": "0.0.0"},
	}
}

// post issues a Streamable HTTP POST to the handler.
func post(t *testing.T, env *testEnv, token string, body []byte, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	env.Handler.ServeHTTP(rec, req)
	return rec
}

// rpcResult unwraps the JSON-RPC "result" member from a response body.
func rpcResult(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envlp struct {
		Result map[string]any  `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envlp); err != nil {
		t.Fatalf("parse JSON-RPC response: %v\nbody: %s", err, rec.Body.String())
	}
	if len(envlp.Error) > 0 {
		t.Fatalf("unexpected JSON-RPC error: %s", envlp.Error)
	}
	return envlp.Result
}

// Governing: SPEC-0018 REQ "Bearer Token Authentication"
// Scenario: Missing or invalid token
func TestUnauthenticatedRejected(t *testing.T) {
	env := newTestEnv(t)

	cases := map[string]func(*http.Request){
		"no header":        nil,
		"malformed":        func(r *http.Request) { r.Header.Set("Authorization", "Bearer") },
		"unknown token":    func(r *http.Request) { r.Header.Set("Authorization", "Bearer jl_notreal") },
		"basic not bearer": func(r *http.Request) { r.Header.Set("Authorization", "Basic Zm9vOmJhcg==") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rec := post(t, env, "", rpc(1, "initialize", initializeParams()), mutate)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
				t.Fatalf("WWW-Authenticate = %q, want Bearer challenge", got)
			}
		})
	}
}

// Governing: SPEC-0018 REQ "Bearer Token Authentication"
// Scenario: Token revocation takes immediate effect
func TestRevokedTokenRejected(t *testing.T) {
	env := newTestEnv(t)
	u, token := seedUserAndToken(t, env, "agent@example.com")

	// Works before revocation.
	rec := post(t, env, token, rpc(1, "initialize", initializeParams()), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("pre-revocation status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	recs, err := env.TokenStore.ListByUser(context.Background(), u.ID)
	if err != nil || len(recs) != 1 {
		t.Fatalf("list tokens: %v (n=%d)", err, len(recs))
	}
	if err := env.TokenStore.Revoke(context.Background(), recs[0].ID, u.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	rec = post(t, env, token, rpc(2, "initialize", initializeParams()), nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("post-revocation status = %d, want 401", rec.Code)
	}
}

// Governing: SPEC-0018 REQ "Bearer Token Authentication"
// Scenario: Cookie-bearing request
func TestSessionCookieIgnored(t *testing.T) {
	env := newTestEnv(t)
	rec := post(t, env, "", rpc(1, "initialize", initializeParams()), func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "session", Value: "some-live-session"})
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for cookie-only request", rec.Code)
	}
}

// Governing: SPEC-0018 REQ "MCP Endpoint"
// Scenario: Initialize handshake
func TestInitializeHandshake(t *testing.T) {
	env := newTestEnv(t)
	_, token := seedUserAndToken(t, env, "agent@example.com")

	rec := post(t, env, token, rpc(1, "initialize", initializeParams()), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	result := rpcResult(t, rec)
	serverInfo, _ := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "joe-links" {
		t.Fatalf("serverInfo.name = %v, want joe-links", serverInfo["name"])
	}
	if v, ok := serverInfo["version"].(string); !ok || v == "" {
		t.Fatalf("serverInfo.version missing: %v", serverInfo["version"])
	}
	if result["protocolVersion"] == "" {
		t.Fatal("protocolVersion missing from initialize result")
	}
}

// Governing: SPEC-0018 REQ "MCP Endpoint"
// Scenario: Restart transparency — stateless requests need no prior session.
func TestStatelessToolsListWithoutInitialize(t *testing.T) {
	env := newTestEnv(t)
	_, token := seedUserAndToken(t, env, "agent@example.com")

	rec := post(t, env, token, rpc(7, "tools/list", nil), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	result := rpcResult(t, rec)
	if _, ok := result["tools"]; !ok {
		t.Fatalf("tools missing from result: %v", result)
	}
}

// Governing: SPEC-0018 "Security Requirements" — headers on every response.
func TestSecurityHeaders(t *testing.T) {
	env := newTestEnv(t)
	_, token := seedUserAndToken(t, env, "agent@example.com")

	for name, tok := range map[string]string{"authenticated": token, "unauthenticated": ""} {
		t.Run(name, func(t *testing.T) {
			rec := post(t, env, tok, rpc(1, "initialize", initializeParams()), nil)
			want := map[string]string{
				"Content-Security-Policy": "default-src 'none'",
				"X-Frame-Options":         "DENY",
				"X-Content-Type-Options":  "nosniff",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
			}
			for h, v := range want {
				if got := rec.Header().Get(h); got != v {
					t.Errorf("%s = %q, want %q", h, got, v)
				}
			}
		})
	}
}

// Governing: SPEC-0018 "Security Requirements" — Request Body Size Limits.
func TestBodySizeLimit(t *testing.T) {
	env := newTestEnv(t)
	_, token := seedUserAndToken(t, env, "agent@example.com")

	huge := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"pad":%q}}`,
		strings.Repeat("x", 2<<20))
	rec := post(t, env, token, []byte(huge), nil)
	if rec.Code < 400 {
		t.Fatalf("status = %d, want an error status for a 2MB body", rec.Code)
	}
}
