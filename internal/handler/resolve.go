// Governing: SPEC-0001 REQ "Short Link Resolution", REQ "HTMX Hypermedia Interactions", ADR-0001
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
// Governing: SPEC-0004 REQ "Slug Resolver and 404 Page"
// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution", REQ "Variable Substitution and Redirect", ADR-0013
package handler

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

// varPlaceholderRe matches $varname placeholders in URL templates.
// Governing: SPEC-0009 REQ "Variable Substitution and Redirect", ADR-0013
var varPlaceholderRe = regexp.MustCompile(`\$[a-z][a-z0-9_]*`)

// ResolveHandler handles short link slug resolution and redirection.
type ResolveHandler struct {
	links    *store.LinkStore
	keywords *store.KeywordStore
}

// NewResolveHandler creates a new ResolveHandler.
func NewResolveHandler(ls *store.LinkStore, ks *store.KeywordStore) *ResolveHandler {
	return &ResolveHandler{links: ls, keywords: ks}
}

type notFoundPage struct {
	BasePage
	User  *store.User
	Slug  string
	Flash *Flash
}

// Resolve looks up a slug and redirects to the target URL, or renders a 404 page.
// Governing: SPEC-0001 REQ "HTMX Hypermedia Interactions"
// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution", ADR-0013
func (h *ResolveHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	// Extract the full path after the leading "/".
	// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution", ADR-0013
	fullPath := strings.TrimPrefix(r.URL.Path, "/")
	if fullPath == "" {
		h.render404(w, r, "")
		return
	}

	// Governing: ADR-0011 — check if request host is a registered keyword.
	host := strings.SplitN(r.Host, ":", 2)[0]
	kw, kwErr := h.keywords.GetByKeyword(r.Context(), host)
	if kwErr == nil {
		// Substitute {slug} in the URL template and redirect.
		target := strings.ReplaceAll(kw.URLTemplate, "{slug}", fullPath)
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	// kwErr == store.ErrNotFound → fall through to normal slug resolution

	// Step 1: Try exact slug match on the full path.
	// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution" — exact match wins
	link, err := h.links.GetBySlug(r.Context(), fullPath)
	if err == nil {
		h.redirect(w, r, link.URL)
		return
	}

	// Step 2: Try progressively shorter prefixes for multi-segment paths.
	// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution", ADR-0013
	segments := strings.Split(fullPath, "/")
	if len(segments) > 1 {
		for i := len(segments) - 1; i >= 1; i-- {
			prefix := strings.Join(segments[:i], "/")
			link, err := h.links.GetBySlug(r.Context(), prefix)
			if err != nil {
				continue
			}

			remaining := segments[i:]

			// Check if URL contains $varname placeholders.
			// Governing: SPEC-0009 REQ "Variable Substitution and Redirect", ADR-0013
			placeholders := varPlaceholderRe.FindAllString(link.URL, -1)
			if len(placeholders) == 0 {
				// Static link — redirect as-is.
				h.redirect(w, r, link.URL)
				return
			}

			// Deduplicate placeholders preserving order of first appearance.
			seen := make(map[string]bool)
			var unique []string
			for _, p := range placeholders {
				if !seen[p] {
					seen[p] = true
					unique = append(unique, p)
				}
			}

			// Arity check: remaining segments must equal unique placeholder count.
			if len(remaining) != len(unique) {
				h.render404(w, r, fullPath)
				return
			}

			// Substitute positionally with url.PathEscape.
			target := link.URL
			for j, placeholder := range unique {
				target = strings.ReplaceAll(target, placeholder, url.PathEscape(remaining[j]))
			}

			h.redirect(w, r, target)
			return
		}
	}

	// No match found → 404.
	h.render404(w, r, fullPath)
}

// redirect issues a 302 redirect, handling HTMX requests with HX-Redirect header.
func (h *ResolveHandler) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// render404 renders the 404 page for a missing slug.
func (h *ResolveHandler) render404(w http.ResponseWriter, r *http.Request, slug string) {
	user := auth.UserFromContext(r.Context())
	w.WriteHeader(http.StatusNotFound)
	data := notFoundPage{BasePage: BasePage{Theme: themeFromRequest(r), User: user}, User: user, Slug: slug}
	if isHTMX(r) {
		renderPageFragment(w, "404.html", "content", data)
		return
	}
	render(w, "404.html", data)
}
