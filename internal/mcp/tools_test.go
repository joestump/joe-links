// Governing: ADR-0018, SPEC-0018 REQ "Tool Inventory", REQ "Agent-Oriented Creation Defaults",
// REQ "Database Operation Standards", REQ "Observability"
package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/llm"
	"github.com/joestump/joe-links/internal/mcp"
	"github.com/joestump/joe-links/internal/metrics"
	"github.com/joestump/joe-links/internal/store"
	jltestutil "github.com/joestump/joe-links/internal/testutil"
)

// fullEnv extends testEnv with every store, for tool tests.
type fullEnv struct {
	Handler      http.Handler
	LinkStore    *store.LinkStore
	TagStore     *store.TagStore
	Ownership    *store.OwnershipStore
	UserStore    *store.UserStore
	TokenStore   *auth.SQLTokenStore
	KeywordStore *store.KeywordStore
	ClickStore   *store.ClickStore
}

func newFullEnv(t *testing.T, suggester llm.Suggester) *fullEnv {
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
	h := mcp.NewHandler(mcp.Deps{
		LinkStore:      ls,
		OwnershipStore: owns,
		TagStore:       tags,
		UserStore:      us,
		KeywordStore:   ks,
		ClickStore:     cs,
		Suggester:      suggester,
	}, bearer)

	return &fullEnv{Handler: h, LinkStore: ls, TagStore: tags, Ownership: owns, UserStore: us, TokenStore: ts, KeywordStore: ks, ClickStore: cs}
}

func seedUserToken(t *testing.T, env *fullEnv, email string) (*store.User, string) {
	t.Helper()
	ctx := context.Background()
	u, err := env.UserStore.Upsert(ctx, "test", "sub-"+email, email, "User "+email, "user")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	plaintext, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if _, err := env.TokenStore.Create(ctx, u.ID, "t", hash, nil); err != nil {
		t.Fatalf("create token: %v", err)
	}
	return u, plaintext
}

// toolResponse is the parsed result of a tools/call.
type toolResponse struct {
	IsError    bool
	Structured map[string]any
	ErrCode    string
	ErrMessage string
	Raw        map[string]any
}

// callTool posts a tools/call and parses the result envelope.
func callTool(t *testing.T, env *fullEnv, token, name string, args map[string]any) *toolResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(string(body)))
	req.Host = "go.example.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tools/call %s: HTTP %d: %s", name, rec.Code, rec.Body.String())
	}

	var envlp struct {
		Result map[string]any  `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envlp); err != nil {
		t.Fatalf("parse response: %v\n%s", err, rec.Body.String())
	}
	if len(envlp.Error) > 0 {
		t.Fatalf("JSON-RPC error calling %s: %s", name, envlp.Error)
	}

	resp := &toolResponse{Raw: envlp.Result}
	resp.IsError, _ = envlp.Result["isError"].(bool)
	resp.Structured, _ = envlp.Result["structuredContent"].(map[string]any)
	if resp.IsError {
		if contents, ok := envlp.Result["content"].([]any); ok && len(contents) > 0 {
			if c0, ok := contents[0].(map[string]any); ok {
				if text, ok := c0["text"].(string); ok {
					var te struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					}
					if err := json.Unmarshal([]byte(text), &te); err == nil {
						resp.ErrCode, resp.ErrMessage = te.Code, te.Message
					} else {
						resp.ErrMessage = text
					}
				}
			}
		}
	}
	return resp
}

func toolNames(t *testing.T, env *fullEnv, token string) []string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.Handler.ServeHTTP(rec, req)
	var envlp struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envlp); err != nil {
		t.Fatalf("parse tools/list: %v", err)
	}
	names := make([]string, 0, len(envlp.Result.Tools))
	for _, tool := range envlp.Result.Tools {
		if tool.InputSchema == nil {
			t.Errorf("tool %s has no input schema", tool.Name)
		}
		names = append(names, tool.Name)
	}
	return names
}

// Governing: SPEC-0018 REQ "Tool Inventory"
// Scenario: Tool discovery
func TestToolInventory(t *testing.T) {
	base := []string{
		"create_link", "get_link", "list_links", "update_link", "delete_link",
		"share_link", "unshare_link", "add_co_owner", "get_link_stats", "list_keywords",
	}

	t.Run("without LLM", func(t *testing.T) {
		env := newFullEnv(t, nil)
		_, token := seedUserToken(t, env, "a@example.com")
		got := toolNames(t, env, token)
		want := map[string]bool{}
		for _, n := range base {
			want[n] = true
		}
		if len(got) != len(base) {
			t.Fatalf("tools = %v, want exactly %v", got, base)
		}
		for _, n := range got {
			if !want[n] {
				t.Errorf("unexpected tool %s", n)
			}
		}
		for _, n := range got {
			if n == "suggest_link_metadata" {
				t.Error("suggest_link_metadata must be absent without LLM config")
			}
		}
	})

	t.Run("with LLM", func(t *testing.T) {
		env := newFullEnv(t, fakeSuggester{})
		_, token := seedUserToken(t, env, "a@example.com")
		got := toolNames(t, env, token)
		if len(got) != len(base)+1 {
			t.Fatalf("tools = %v, want %d incl. suggest_link_metadata", got, len(base)+1)
		}
		found := false
		for _, n := range got {
			if n == "suggest_link_metadata" {
				found = true
			}
		}
		if !found {
			t.Error("suggest_link_metadata missing with LLM configured")
		}
	})
}

// Governing: SPEC-0018 REQ "Agent-Oriented Creation Defaults"
// Scenarios: Default is private / Share-with-me in one call / Unknown share recipient
func TestCreateLinkDefaultsAndSharing(t *testing.T) {
	env := newFullEnv(t, nil)
	_, token := seedUserToken(t, env, "agent@example.com")
	joe, _ := seedUserToken(t, env, "joe@example.com")

	t.Run("default is private with short URL", func(t *testing.T) {
		resp := callTool(t, env, token, "create_link", map[string]any{
			"slug": "retro-notes", "url": "https://example.com/retro",
		})
		if resp.IsError {
			t.Fatalf("unexpected error: %s %s", resp.ErrCode, resp.ErrMessage)
		}
		if v := resp.Structured["visibility"]; v != "private" {
			t.Errorf("visibility = %v, want private", v)
		}
		if su := resp.Structured["short_url"]; su != "http://go.example.com/retro-notes" {
			t.Errorf("short_url = %v", su)
		}
	})

	t.Run("share_with implies secure and grants access", func(t *testing.T) {
		resp := callTool(t, env, token, "create_link", map[string]any{
			"slug": "shared-doc", "url": "https://example.com/doc",
			"share_with": []string{"joe@example.com"},
		})
		if resp.IsError {
			t.Fatalf("unexpected error: %s %s", resp.ErrCode, resp.ErrMessage)
		}
		if v := resp.Structured["visibility"]; v != "secure" {
			t.Errorf("visibility = %v, want secure", v)
		}
		shared, _ := resp.Structured["shared_with"].([]any)
		if len(shared) != 1 || shared[0] != "joe@example.com" {
			t.Errorf("shared_with = %v", shared)
		}
		link, err := env.LinkStore.GetBySlug(context.Background(), "shared-doc")
		if err != nil {
			t.Fatalf("link not created: %v", err)
		}
		has, err := env.LinkStore.HasShare(context.Background(), link.ID, joe.ID)
		if err != nil || !has {
			t.Errorf("share grant missing (has=%v err=%v)", has, err)
		}
	})

	t.Run("unknown share recipient fails atomically", func(t *testing.T) {
		resp := callTool(t, env, token, "create_link", map[string]any{
			"slug": "doomed", "url": "https://example.com/x",
			"share_with": []string{"nobody@example.com"},
		})
		if !resp.IsError || resp.ErrCode != "unknown_user" {
			t.Fatalf("want unknown_user error, got isError=%v code=%s", resp.IsError, resp.ErrCode)
		}
		if !strings.Contains(resp.ErrMessage, "nobody@example.com") {
			t.Errorf("error must name the unknown email: %s", resp.ErrMessage)
		}
		if _, err := env.LinkStore.GetBySlug(context.Background(), "doomed"); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("link must not exist after failed share resolution, got err=%v", err)
		}
	})

	t.Run("duplicate slug", func(t *testing.T) {
		resp := callTool(t, env, token, "create_link", map[string]any{
			"slug": "retro-notes", "url": "https://example.com/other",
		})
		if !resp.IsError || resp.ErrCode != "duplicate_slug" {
			t.Fatalf("want duplicate_slug, got isError=%v code=%s", resp.IsError, resp.ErrCode)
		}
	})

	t.Run("validation errors", func(t *testing.T) {
		for name, args := range map[string]map[string]any{
			"bad slug":      {"slug": "Bad_Slug", "url": "https://example.com"},
			"reserved slug": {"slug": "mcp", "url": "https://example.com"},
			"empty url":     {"slug": "ok-slug", "url": ""},
			"dup variable":  {"slug": "ok-slug2", "url": "https://x.com/$a/$a"},
			"bad viz":       {"slug": "ok-slug3", "url": "https://x.com", "visibility": "sneaky"},
		} {
			resp := callTool(t, env, token, "create_link", args)
			if !resp.IsError || resp.ErrCode != "validation_failed" {
				t.Errorf("%s: want validation_failed, got isError=%v code=%s", name, resp.IsError, resp.ErrCode)
			}
		}
	})

	// Governing: SPEC-0018 REQ "Observability" — scenario: Tool call metrics
	t.Run("metrics counted", func(t *testing.T) {
		if got := testutil.ToFloat64(metrics.MCPToolCallsTotal.WithLabelValues("create_link", "success")); got < 2 {
			t.Errorf("create_link success counter = %v, want >= 2", got)
		}
		if got := testutil.ToFloat64(metrics.MCPToolCallsTotal.WithLabelValues("create_link", "error")); got < 1 {
			t.Errorf("create_link error counter = %v, want >= 1", got)
		}
	})
}

// Governing: SPEC-0018 REQ "Tool Inventory" — get/update/delete round trip.
func TestLinkLifecycleTools(t *testing.T) {
	env := newFullEnv(t, nil)
	_, token := seedUserToken(t, env, "agent@example.com")
	_, otherToken := seedUserToken(t, env, "other@example.com")

	created := callTool(t, env, token, "create_link", map[string]any{
		"slug": "life", "url": "https://example.com/1", "title": "One",
		"tags": []string{"Jira", "jira", "ops"}, "visibility": "public",
	})
	if created.IsError {
		t.Fatalf("create: %s %s", created.ErrCode, created.ErrMessage)
	}
	tags, _ := created.Structured["tags"].([]any)
	if len(tags) != 2 {
		t.Errorf("duplicate tag spellings must collapse: tags = %v", tags)
	}

	t.Run("get by slug and by id", func(t *testing.T) {
		byslug := callTool(t, env, token, "get_link", map[string]any{"link": "life"})
		if byslug.IsError {
			t.Fatalf("get by slug: %s", byslug.ErrMessage)
		}
		id, _ := byslug.Structured["id"].(string)
		byid := callTool(t, env, token, "get_link", map[string]any{"link": id})
		if byid.IsError || byid.Structured["slug"] != "life" {
			t.Fatalf("get by id failed: %v", byid.Structured)
		}
	})

	t.Run("partial update leaves other fields", func(t *testing.T) {
		resp := callTool(t, env, token, "update_link", map[string]any{
			"link": "life", "title": "Renamed",
		})
		if resp.IsError {
			t.Fatalf("update: %s %s", resp.ErrCode, resp.ErrMessage)
		}
		if resp.Structured["title"] != "Renamed" || resp.Structured["url"] != "https://example.com/1" {
			t.Errorf("overlay wrong: %v", resp.Structured)
		}
	})

	t.Run("non-owner cannot update or delete", func(t *testing.T) {
		up := callTool(t, env, otherToken, "update_link", map[string]any{"link": "life", "title": "Hax"})
		if !up.IsError || up.ErrCode != "forbidden" {
			t.Errorf("update: want forbidden, got %v %s", up.IsError, up.ErrCode)
		}
		del := callTool(t, env, otherToken, "delete_link", map[string]any{"link": "life"})
		if !del.IsError || del.ErrCode != "forbidden" {
			t.Errorf("delete: want forbidden, got %v %s", del.IsError, del.ErrCode)
		}
	})

	t.Run("owner deletes", func(t *testing.T) {
		resp := callTool(t, env, token, "delete_link", map[string]any{"link": "life"})
		if resp.IsError {
			t.Fatalf("delete: %s", resp.ErrMessage)
		}
		if resp.Structured["deleted"] != true {
			t.Errorf("deleted = %v", resp.Structured["deleted"])
		}
		get := callTool(t, env, token, "get_link", map[string]any{"link": "life"})
		if !get.IsError || get.ErrCode != "not_found" {
			t.Errorf("after delete: want not_found, got %v %s", get.IsError, get.ErrCode)
		}
	})
}

// Governing: SPEC-0018 REQ "Tool Inventory" — share/unshare/co-owner tools.
func TestShareAndCoOwnerTools(t *testing.T) {
	env := newFullEnv(t, nil)
	_, token := seedUserToken(t, env, "agent@example.com")
	_, joeToken := seedUserToken(t, env, "joe@example.com")

	create := callTool(t, env, token, "create_link", map[string]any{
		"slug": "secret", "url": "https://example.com/s", "visibility": "secure",
	})
	if create.IsError {
		t.Fatalf("create: %s", create.ErrMessage)
	}

	t.Run("recipient cannot read before share, can after", func(t *testing.T) {
		before := callTool(t, env, joeToken, "get_link", map[string]any{"link": "secret"})
		if !before.IsError || before.ErrCode != "forbidden" {
			t.Fatalf("pre-share read: want forbidden, got %v %s", before.IsError, before.ErrCode)
		}
		share := callTool(t, env, token, "share_link", map[string]any{"link": "secret", "email": "joe@example.com"})
		if share.IsError {
			t.Fatalf("share: %s %s", share.ErrCode, share.ErrMessage)
		}
		after := callTool(t, env, joeToken, "get_link", map[string]any{"link": "secret"})
		if after.IsError {
			t.Fatalf("post-share read: %s %s", after.ErrCode, after.ErrMessage)
		}
		// Share recipients don't see the share roster (owners only).
		if _, present := after.Structured["shared_with"]; present {
			t.Errorf("share recipient must not see the share roster")
		}
	})

	t.Run("duplicate share rejected", func(t *testing.T) {
		resp := callTool(t, env, token, "share_link", map[string]any{"link": "secret", "email": "joe@example.com"})
		if !resp.IsError || resp.ErrCode != "duplicate_share" {
			t.Errorf("want duplicate_share, got %v %s", resp.IsError, resp.ErrCode)
		}
	})

	t.Run("recipient cannot manage shares", func(t *testing.T) {
		resp := callTool(t, env, joeToken, "share_link", map[string]any{"link": "secret", "email": "joe@example.com"})
		if !resp.IsError || resp.ErrCode != "forbidden" {
			t.Errorf("want forbidden, got %v %s", resp.IsError, resp.ErrCode)
		}
	})

	t.Run("unshare revokes", func(t *testing.T) {
		resp := callTool(t, env, token, "unshare_link", map[string]any{"link": "secret", "email": "joe@example.com"})
		if resp.IsError {
			t.Fatalf("unshare: %s", resp.ErrMessage)
		}
		read := callTool(t, env, joeToken, "get_link", map[string]any{"link": "secret"})
		if !read.IsError || read.ErrCode != "forbidden" {
			t.Errorf("post-unshare read: want forbidden, got %v %s", read.IsError, read.ErrCode)
		}
	})

	t.Run("co-owner add and duplicate", func(t *testing.T) {
		resp := callTool(t, env, token, "add_co_owner", map[string]any{"link": "secret", "email": "joe@example.com"})
		if resp.IsError {
			t.Fatalf("add_co_owner: %s %s", resp.ErrCode, resp.ErrMessage)
		}
		owners, _ := resp.Structured["owners"].([]any)
		if len(owners) != 2 {
			t.Errorf("owners = %v, want 2", owners)
		}
		dup := callTool(t, env, token, "add_co_owner", map[string]any{"link": "secret", "email": "joe@example.com"})
		if !dup.IsError || dup.ErrCode != "duplicate_owner" {
			t.Errorf("want duplicate_owner, got %v %s", dup.IsError, dup.ErrCode)
		}
		// Co-owner can now update.
		up := callTool(t, env, joeToken, "update_link", map[string]any{"link": "secret", "title": "Ours"})
		if up.IsError {
			t.Errorf("co-owner update: %s %s", up.ErrCode, up.ErrMessage)
		}
	})

	t.Run("unknown email named", func(t *testing.T) {
		resp := callTool(t, env, token, "share_link", map[string]any{"link": "secret", "email": "ghost@example.com"})
		if !resp.IsError || resp.ErrCode != "unknown_user" || !strings.Contains(resp.ErrMessage, "ghost@example.com") {
			t.Errorf("got %v %s %q", resp.IsError, resp.ErrCode, resp.ErrMessage)
		}
	})
}

// Governing: SPEC-0018 REQ "Authorization Parity with the REST API"
// Scenario: Visibility respected in listing
func TestListLinksFilters(t *testing.T) {
	env := newFullEnv(t, nil)
	_, token := seedUserToken(t, env, "agent@example.com")
	_, otherToken := seedUserToken(t, env, "other@example.com")

	mk := func(tok, slug, viz string, share []string) {
		args := map[string]any{"slug": slug, "url": "https://example.com/" + slug, "visibility": viz}
		if share != nil {
			args["share_with"] = share
		}
		if resp := callTool(t, env, tok, "create_link", args); resp.IsError {
			t.Fatalf("create %s: %s", slug, resp.ErrMessage)
		}
	}
	mk(token, "mine-pub", "public", nil)
	mk(token, "mine-priv", "private", nil)
	mk(otherToken, "theirs-pub", "public", nil)
	mk(otherToken, "theirs-priv", "private", nil)
	mk(otherToken, "theirs-secure", "secure", []string{"agent@example.com"})

	names := func(resp *toolResponse) map[string]bool {
		got := map[string]bool{}
		links, _ := resp.Structured["links"].([]any)
		for _, l := range links {
			m, _ := l.(map[string]any)
			got[m["slug"].(string)] = true
		}
		return got
	}

	t.Run("mine", func(t *testing.T) {
		resp := callTool(t, env, token, "list_links", map[string]any{})
		got := names(resp)
		if !got["mine-pub"] || !got["mine-priv"] || len(got) != 2 {
			t.Errorf("mine = %v", got)
		}
	})

	t.Run("shared", func(t *testing.T) {
		resp := callTool(t, env, token, "list_links", map[string]any{"filter": "shared"})
		got := names(resp)
		if !got["theirs-secure"] || len(got) != 1 {
			t.Errorf("shared = %v", got)
		}
	})

	t.Run("public excludes others' private and secure", func(t *testing.T) {
		resp := callTool(t, env, token, "list_links", map[string]any{"filter": "public"})
		got := names(resp)
		if !got["mine-pub"] || !got["theirs-pub"] {
			t.Errorf("public missing public links: %v", got)
		}
		if got["theirs-priv"] || got["theirs-secure"] {
			t.Errorf("public leaked non-public links: %v", got)
		}
	})

	t.Run("search mine", func(t *testing.T) {
		resp := callTool(t, env, token, "list_links", map[string]any{"q": "mine-pub"})
		got := names(resp)
		if !got["mine-pub"] || len(got) != 1 {
			t.Errorf("q= results: %v", got)
		}
	})

	t.Run("bad filter", func(t *testing.T) {
		resp := callTool(t, env, token, "list_links", map[string]any{"filter": "everything"})
		if !resp.IsError || resp.ErrCode != "validation_failed" {
			t.Errorf("want validation_failed, got %v %s", resp.IsError, resp.ErrCode)
		}
	})
}

// Governing: SPEC-0018 REQ "Tool Inventory" — get_link_stats.
func TestGetLinkStats(t *testing.T) {
	env := newFullEnv(t, nil)
	_, token := seedUserToken(t, env, "agent@example.com")
	_, otherToken := seedUserToken(t, env, "other@example.com")

	create := callTool(t, env, token, "create_link", map[string]any{"slug": "counted", "url": "https://example.com/c"})
	if create.IsError {
		t.Fatalf("create: %s", create.ErrMessage)
	}
	id, _ := create.Structured["id"].(string)

	if err := env.ClickStore.RecordClick(context.Background(), store.ClickEvent{LinkID: id, Referrer: "https://news.ycombinator.com/"}); err != nil {
		t.Fatalf("record click: %v", err)
	}

	resp := callTool(t, env, token, "get_link_stats", map[string]any{"link": "counted"})
	if resp.IsError {
		t.Fatalf("stats: %s %s", resp.ErrCode, resp.ErrMessage)
	}
	if total, _ := resp.Structured["total"].(float64); total != 1 {
		t.Errorf("total = %v, want 1", resp.Structured["total"])
	}
	recent, _ := resp.Structured["recent"].([]any)
	if len(recent) != 1 {
		t.Fatalf("recent = %v", recent)
	}
	r0, _ := recent[0].(map[string]any)
	if r0["referrer"] != "https://news.ycombinator.com/" {
		t.Errorf("referrer = %v", r0["referrer"])
	}

	denied := callTool(t, env, otherToken, "get_link_stats", map[string]any{"link": "counted"})
	if !denied.IsError || denied.ErrCode != "forbidden" {
		t.Errorf("stranger stats: want forbidden, got %v %s", denied.IsError, denied.ErrCode)
	}
}

// fakeSuggester returns canned suggestions; slug intentionally invalid in one mode.
type fakeSuggester struct {
	badSlug bool
	err     error
}

func (f fakeSuggester) Suggest(ctx context.Context, req llm.SuggestRequest) (*llm.SuggestResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	slug := "suggested-slug"
	if f.badSlug {
		slug = "Invalid Slug!"
	}
	return &llm.SuggestResponse{Slug: slug, Title: "Suggested", Description: "From LLM", Tags: []string{"ai"}}, nil
}

// Governing: SPEC-0018 REQ "Conditional Suggestion Tool" + SPEC-0017 parity.
func TestSuggestTool(t *testing.T) {
	t.Run("maps suggestions", func(t *testing.T) {
		env := newFullEnv(t, fakeSuggester{})
		_, token := seedUserToken(t, env, "a@example.com")
		resp := callTool(t, env, token, "suggest_link_metadata", map[string]any{"url": "https://example.com/page"})
		if resp.IsError {
			t.Fatalf("suggest: %s %s", resp.ErrCode, resp.ErrMessage)
		}
		if resp.Structured["slug"] != "suggested-slug" || resp.Structured["title"] != "Suggested" {
			t.Errorf("structured = %v", resp.Structured)
		}
	})

	t.Run("invalid slug blanked like REST", func(t *testing.T) {
		env := newFullEnv(t, fakeSuggester{badSlug: true})
		_, token := seedUserToken(t, env, "a@example.com")
		resp := callTool(t, env, token, "suggest_link_metadata", map[string]any{"url": "https://example.com/page"})
		if resp.IsError {
			t.Fatalf("suggest: %s", resp.ErrMessage)
		}
		if slug, present := resp.Structured["slug"]; present && slug != "" {
			t.Errorf("invalid slug must be blanked, got %v", slug)
		}
	})

	t.Run("provider error", func(t *testing.T) {
		env := newFullEnv(t, fakeSuggester{err: errors.New("boom")})
		_, token := seedUserToken(t, env, "a@example.com")
		resp := callTool(t, env, token, "suggest_link_metadata", map[string]any{"url": "https://example.com/page"})
		if !resp.IsError || resp.ErrCode != "llm_error" {
			t.Errorf("want llm_error, got %v %s", resp.IsError, resp.ErrCode)
		}
	})
}

// Governing: SPEC-0018 REQ "Tool Inventory" — list_keywords.
func TestListKeywords(t *testing.T) {
	env := newFullEnv(t, nil)
	_, token := seedUserToken(t, env, "a@example.com")

	if _, err := env.KeywordStore.Create(context.Background(), "jira", "https://jira.example.com/browse/{slug}", "Jira tickets"); err != nil {
		t.Fatalf("seed keyword: %v", err)
	}

	resp := callTool(t, env, token, "list_keywords", map[string]any{})
	if resp.IsError {
		t.Fatalf("list_keywords: %s", resp.ErrMessage)
	}
	kws, _ := resp.Structured["keywords"].([]any)
	if len(kws) != 1 || kws[0] != "jira" {
		t.Errorf("keywords = %v", kws)
	}
}
