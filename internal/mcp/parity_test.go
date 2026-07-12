// Authorization-parity matrix: every MCP tool must allow/deny exactly like
// its /api/v1 counterpart, because both surfaces share the same store calls.
// The matrix drives BOTH handlers over the SAME database and compares
// outcomes cell by cell.
//
// Governing: ADR-0018, SPEC-0018 REQ "Authorization Parity with the REST API",
// REQ "Structured Tool Errors"
package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joestump/joe-links/internal/api"
	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/mcp"
	"github.com/joestump/joe-links/internal/store"
	jltestutil "github.com/joestump/joe-links/internal/testutil"
)

// parityEnv drives the MCP handler and the REST router over one database.
type parityEnv struct {
	MCP  http.Handler
	REST http.Handler
	*fullEnv
}

func newParityEnv(t *testing.T) *parityEnv {
	t.Helper()
	db := jltestutil.NewTestDB(t)

	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	us := store.NewUserStore(db)
	ts := auth.NewSQLTokenStore(db)
	ks := store.NewKeywordStore(db)
	cs := store.NewClickStore(db)

	bearer := auth.NewBearerTokenMiddleware(ts, us)
	deps := mcp.Deps{
		LinkStore: ls, OwnershipStore: owns, TagStore: tags,
		UserStore: us, KeywordStore: ks, ClickStore: cs,
	}

	rest := api.NewAPIRouter(api.Deps{
		BearerMiddleware: bearer, TokenStore: ts,
		LinkStore: ls, OwnershipStore: owns, TagStore: tags,
		UserStore: us, KeywordStore: ks, ClickStore: cs,
	})

	env := &fullEnv{
		Handler: mcp.NewHandler(deps, bearer), LinkStore: ls, TagStore: tags,
		Ownership: owns, UserStore: us, TokenStore: ts, KeywordStore: ks, ClickStore: cs,
	}
	return &parityEnv{MCP: env.Handler, REST: rest, fullEnv: env}
}

// restCall issues an authenticated REST request and returns the status code.
func restCall(t *testing.T, env *parityEnv, token, method, path string, body any) int {
	t.Helper()
	var rdr *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = strings.NewReader(string(b))
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.REST.ServeHTTP(rec, req)
	return rec.Code
}

// principal is one row of the authorization matrix.
type principal struct {
	name  string
	token string
}

// Governing: SPEC-0018 REQ "Authorization Parity with the REST API"
// Scenarios: Non-owner mutation denied / Visibility respected in listing
func TestAuthorizationParityWithREST(t *testing.T) {
	env := newParityEnv(t)
	ctx := context.Background()

	owner, ownerTok := seedUserToken(t, env.fullEnv, "owner@example.com")
	coowner, coTok := seedUserToken(t, env.fullEnv, "coowner@example.com")
	recipient, recTok := seedUserToken(t, env.fullEnv, "recipient@example.com")
	_, strangerTok := seedUserToken(t, env.fullEnv, "stranger@example.com")
	adminUser, adminTok := seedUserToken(t, env.fullEnv, "admin@example.com")
	if _, err := env.UserStore.UpdateRole(ctx, adminUser.ID, "admin"); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	principals := []principal{
		{"owner", ownerTok}, {"co-owner", coTok}, {"share-recipient", recTok},
		{"stranger", strangerTok}, {"admin", adminTok},
	}

	// expected[op] per principal order above.
	expected := map[string][]bool{
		"read":          {true, true, true, false, true},
		"update":        {true, true, false, false, true},
		"stats":         {true, true, false, false, true},
		"manage-shares": {true, true, false, false, true},
		"delete":        {true, true, false, false, true},
	}

	// Each cell gets a fresh secure link with co-owner + share grant so
	// mutations cannot contaminate other cells.
	var seq int
	freshLink := func(t *testing.T) *store.Link {
		t.Helper()
		seq++
		slug := fmt.Sprintf("parity-%d", seq)
		link, err := env.LinkStore.CreateFull(ctx, slug, "https://example.com/"+slug,
			owner.ID, "Parity", "", "secure", nil, []string{recipient.ID}, owner.ID)
		if err != nil {
			t.Fatalf("seed link: %v", err)
		}
		if err := env.LinkStore.AddOwner(ctx, link.ID, coowner.ID); err != nil {
			t.Fatalf("seed co-owner: %v", err)
		}
		return link
	}

	// mcpAllowed reports whether the MCP tool call succeeded (no isError).
	mcpAllowed := func(t *testing.T, token, tool string, args map[string]any) bool {
		resp := callTool(t, env.fullEnv, token, tool, args)
		if resp.IsError && resp.ErrCode != "forbidden" {
			t.Fatalf("mcp %s: unexpected error code %s (%s)", tool, resp.ErrCode, resp.ErrMessage)
		}
		return !resp.IsError
	}

	type op struct {
		name string
		mcp  func(t *testing.T, token string, link *store.Link) bool
		rest func(t *testing.T, token string, link *store.Link) int
	}

	ops := []op{
		{
			name: "read",
			mcp: func(t *testing.T, tok string, l *store.Link) bool {
				return mcpAllowed(t, tok, "get_link", map[string]any{"link": l.ID})
			},
			rest: func(t *testing.T, tok string, l *store.Link) int {
				return restCall(t, env, tok, http.MethodGet, "/links/"+l.ID, nil)
			},
		},
		{
			name: "update",
			mcp: func(t *testing.T, tok string, l *store.Link) bool {
				return mcpAllowed(t, tok, "update_link", map[string]any{"link": l.ID, "title": "changed"})
			},
			rest: func(t *testing.T, tok string, l *store.Link) int {
				return restCall(t, env, tok, http.MethodPut, "/links/"+l.ID,
					map[string]any{"url": l.URL, "title": "changed"})
			},
		},
		{
			name: "stats",
			mcp: func(t *testing.T, tok string, l *store.Link) bool {
				return mcpAllowed(t, tok, "get_link_stats", map[string]any{"link": l.ID})
			},
			rest: func(t *testing.T, tok string, l *store.Link) int {
				return restCall(t, env, tok, http.MethodGet, "/links/"+l.ID+"/stats", nil)
			},
		},
		{
			name: "manage-shares",
			mcp: func(t *testing.T, tok string, l *store.Link) bool {
				return mcpAllowed(t, tok, "share_link", map[string]any{"link": l.ID, "email": "stranger@example.com"})
			},
			rest: func(t *testing.T, tok string, l *store.Link) int {
				return restCall(t, env, tok, http.MethodPost, "/links/"+l.ID+"/shares",
					map[string]any{"email": "stranger@example.com"})
			},
		},
		{
			name: "delete",
			mcp: func(t *testing.T, tok string, l *store.Link) bool {
				return mcpAllowed(t, tok, "delete_link", map[string]any{"link": l.ID})
			},
			rest: func(t *testing.T, tok string, l *store.Link) int {
				return restCall(t, env, tok, http.MethodDelete, "/links/"+l.ID, nil)
			},
		},
	}

	for _, o := range ops {
		for i, p := range principals {
			t.Run(o.name+"/"+p.name, func(t *testing.T) {
				// Independent links so mutations don't leak between cells.
				mcpLink := freshLink(t)
				restLink := freshLink(t)

				gotMCP := o.mcp(t, p.token, mcpLink)
				restStatus := o.rest(t, p.token, restLink)
				gotREST := restStatus >= 200 && restStatus < 300

				want := expected[o.name][i]
				if gotMCP != want {
					t.Errorf("MCP %s as %s = %v, want %v", o.name, p.name, gotMCP, want)
				}
				if gotREST != want {
					t.Errorf("REST %s as %s = %v (status %d), want %v", o.name, p.name, gotREST, restStatus, want)
				}
				// The parity property itself.
				if gotMCP != gotREST {
					t.Errorf("PARITY VIOLATION: %s as %s — MCP allowed=%v, REST allowed=%v (status %d)",
						o.name, p.name, gotMCP, gotREST, restStatus)
				}
			})
		}
	}
}

// Governing: SPEC-0018 REQ "Structured Tool Errors"
// Scenario: Duplicate slug — and the session stays usable after any tool error.
func TestErrorContractAndSessionSurvival(t *testing.T) {
	env := newParityEnv(t)
	_, token := seedUserToken(t, env.fullEnv, "agent@example.com")

	t.Run("not_found code", func(t *testing.T) {
		resp := callTool(t, env.fullEnv, token, "get_link", map[string]any{"link": "no-such-slug"})
		if !resp.IsError || resp.ErrCode != "not_found" {
			t.Fatalf("want not_found, got isError=%v code=%s", resp.IsError, resp.ErrCode)
		}
		if !strings.Contains(resp.ErrMessage, "no-such-slug") {
			t.Errorf("message should name the ref: %q", resp.ErrMessage)
		}
	})

	t.Run("error results are tool-level, session usable after", func(t *testing.T) {
		create := callTool(t, env.fullEnv, token, "create_link", map[string]any{"slug": "keeper", "url": "https://example.com"})
		if create.IsError {
			t.Fatalf("create: %s", create.ErrMessage)
		}
		dup := callTool(t, env.fullEnv, token, "create_link", map[string]any{"slug": "keeper", "url": "https://example.com/2"})
		if !dup.IsError || dup.ErrCode != "duplicate_slug" {
			t.Fatalf("want duplicate_slug tool error, got isError=%v code=%s", dup.IsError, dup.ErrCode)
		}
		// The failure was a tool result, not a protocol error — the very next
		// call on the same token must work.
		after := callTool(t, env.fullEnv, token, "get_link", map[string]any{"link": "keeper"})
		if after.IsError {
			t.Fatalf("session unusable after tool error: %s %s", after.ErrCode, after.ErrMessage)
		}
	})

	t.Run("error payload is machine-readable JSON", func(t *testing.T) {
		resp := callTool(t, env.fullEnv, token, "create_link", map[string]any{"slug": "Bad Slug", "url": "https://example.com"})
		if !resp.IsError {
			t.Fatal("expected validation error")
		}
		if resp.ErrCode != "validation_failed" || resp.ErrMessage == "" {
			t.Errorf("payload must carry {code,message}: code=%q message=%q", resp.ErrCode, resp.ErrMessage)
		}
	})
}
