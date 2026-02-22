package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/joestump/joe-links/internal/store"
	"github.com/joestump/joe-links/internal/testutil"
)

type resolveTestEnv struct {
	ls     *store.LinkStore
	ks     *store.KeywordStore
	rh     *ResolveHandler
	userID string
}

// newResolveTestEnv sets up a LinkStore, KeywordStore, and ResolveHandler backed
// by an in-memory SQLite database with all migrations applied.
func newResolveTestEnv(t *testing.T) *resolveTestEnv {
	t.Helper()
	db := testutil.NewTestDB(t)
	owns := store.NewOwnershipStore(db)
	tags := store.NewTagStore(db)
	ls := store.NewLinkStore(db, owns, tags)
	ks := store.NewKeywordStore(db)
	us := store.NewUserStore(db)

	u, err := us.Upsert(context.Background(), "test", "sub1", "test@example.com", "Test", "")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rh := NewResolveHandler(ls, ks, owns)
	return &resolveTestEnv{ls: ls, ks: ks, rh: rh, userID: u.ID}
}

// seedLink creates a link with the given slug and URL.
func (e *resolveTestEnv) seedLink(t *testing.T, slug, url string) {
	t.Helper()
	_, err := e.ls.Create(context.Background(), slug, url, e.userID, "", "", "")
	if err != nil {
		t.Fatalf("seed link %q: %v", slug, err)
	}
}

// seedKeyword creates a keyword with the given keyword string, URL template, and description.
func (e *resolveTestEnv) seedKeyword(t *testing.T, keyword, urlTemplate, description string) {
	t.Helper()
	_, err := e.ks.Create(context.Background(), keyword, urlTemplate, description)
	if err != nil {
		t.Fatalf("seed keyword %q: %v", keyword, err)
	}
}

// resolve builds a chi-routed request and records the response.
func (e *resolveTestEnv) resolve(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/{slug}*", e.rh.Resolve)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// resolveWithHost builds a chi-routed request with a custom Host header.
func (e *resolveTestEnv) resolveWithHost(t *testing.T, path, host string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/{slug}*", e.rh.Resolve)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = host
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestResolve_StaticLink(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "example", "https://example.com")

	w := env.resolve(t, "/example")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://example.com" {
		t.Errorf("Location = %q, want %q", loc, "https://example.com")
	}
}

func TestResolve_ExactMatchPriority(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "github", "https://github.com/$username")
	env.seedLink(t, "github/joestump", "https://joestump.dev")

	w := env.resolve(t, "/github/joestump")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://joestump.dev" {
		t.Errorf("Location = %q, want %q (exact match should win)", loc, "https://joestump.dev")
	}
}

func TestResolve_SingleVariable(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "github", "https://github.com/$username")

	w := env.resolve(t, "/github/joestump")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://github.com/joestump" {
		t.Errorf("Location = %q, want %q", loc, "https://github.com/joestump")
	}
}

func TestResolve_MultipleVariables(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "my-link", "https://example.com/?q=$query&page=$page")

	w := env.resolve(t, "/my-link/widgets/3")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://example.com/?q=widgets&page=3" {
		t.Errorf("Location = %q, want %q", loc, "https://example.com/?q=widgets&page=3")
	}
}

func TestResolve_ArityMismatch_TooFew(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "my-link", "https://example.com/?q=$query&page=$page")

	w := env.resolve(t, "/my-link/widgets")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestResolve_ArityMismatch_TooMany(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "github", "https://github.com/$username")

	w := env.resolve(t, "/github/joestump/extra")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestResolve_PrefixStaticLink(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "docs", "https://docs.example.com")

	w := env.resolve(t, "/docs/anything/here")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://docs.example.com" {
		t.Errorf("Location = %q, want %q", loc, "https://docs.example.com")
	}
}

func TestResolve_NoMatch_404(t *testing.T) {
	env := newResolveTestEnv(t)

	w := env.resolve(t, "/nonexistent")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestResolve_NoMatch_MultiSegment_404(t *testing.T) {
	env := newResolveTestEnv(t)

	w := env.resolve(t, "/nonexistent/path/here")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestResolve_PathEscape(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "search", "https://example.com/search?q=$term")

	w := env.resolve(t, "/search/hello%20world")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://example.com/search?q=hello%20world" {
		t.Errorf("Location = %q, want %q", loc, "https://example.com/search?q=hello%20world")
	}
}

func TestResolve_DuplicatePlaceholder(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedLink(t, "repeat", "https://example.com/$name/profile/$name")

	w := env.resolve(t, "/repeat/alice")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://example.com/alice/profile/alice" {
		t.Errorf("Location = %q, want %q", loc, "https://example.com/alice/profile/alice")
	}
}

func TestResolve_PathKeywordRouting(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedKeyword(t, "gh", "https://github.com/{slug}", "GitHub shortcut")

	w := env.resolve(t, "/gh/joestump")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://github.com/joestump" {
		t.Errorf("Location = %q, want %q", loc, "https://github.com/joestump")
	}
}

func TestResolve_PathKeywordRouting_MultiSegmentSlug(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedKeyword(t, "gh", "https://github.com/{slug}", "GitHub shortcut")

	w := env.resolve(t, "/gh/joestump/joe-links")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://github.com/joestump/joe-links" {
		t.Errorf("Location = %q, want %q", loc, "https://github.com/joestump/joe-links")
	}
}

func TestResolve_PathKeywordRouting_UnknownKeyword(t *testing.T) {
	env := newResolveTestEnv(t)
	// "gh" not seeded — should fall through to normal resolution and 404.
	w := env.resolve(t, "/gh/joestump")
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestResolve_HostKeywordRouting(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedKeyword(t, "go", "https://go.example.com/{slug}", "")

	// When Host header matches the keyword, host-header routing handles it.
	// fullPath = "slack", so {slug} = "slack".
	w := env.resolveWithHost(t, "/slack", "go")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://go.example.com/slack" {
		t.Errorf("Location = %q, want %q", loc, "https://go.example.com/slack")
	}
}

func TestResolve_PathKeywordRouting_SkipWhenHostIsKeyword(t *testing.T) {
	env := newResolveTestEnv(t)
	env.seedKeyword(t, "go", "https://go.example.com/{slug}", "")

	// With Host: go, path-based routing is skipped (parts[0] == host).
	// Host-header routing fires: fullPath = "go/slack", so {slug} = "go/slack".
	w := env.resolveWithHost(t, "/go/slack", "go")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc != "https://go.example.com/go/slack" {
		t.Errorf("Location = %q, want %q", loc, "https://go.example.com/go/slack")
	}
}
