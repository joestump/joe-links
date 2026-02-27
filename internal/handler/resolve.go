// Governing: SPEC-0001 REQ "Short Link Resolution", REQ "HTMX Hypermedia Interactions", ADR-0001
// Governing: SPEC-0003 REQ "Theme Persistence via Cookie", ADR-0006
// Governing: SPEC-0004 REQ "Slug Resolver and 404 Page"
// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution", REQ "Variable Substitution and Redirect", ADR-0013
// Governing: SPEC-0010 REQ "Secure Link Resolution", REQ "Public Link Resolution", REQ "Private Link Resolution", ADR-0014
// Governing: SPEC-0016 REQ "Click Recording", REQ "Prometheus Metrics Endpoint", ADR-0016
package handler

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/metrics"
	"github.com/joestump/joe-links/internal/store"
)

// varPlaceholderRe matches $varname placeholders in URL templates.
// Governing: SPEC-0009 REQ "Variable Substitution and Redirect", ADR-0013
var varPlaceholderRe = regexp.MustCompile(`\$[a-z][a-z0-9_]*`)

// ResolveHandler handles short link slug resolution and redirection.
// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
type ResolveHandler struct {
	links      *store.LinkStore
	keywords   *store.KeywordStore
	ownership  *store.OwnershipStore
	clickCh    chan<- store.ClickEvent
}

// NewResolveHandler creates a new ResolveHandler.
// If clickCh is nil, click recording is disabled.
func NewResolveHandler(ls *store.LinkStore, ks *store.KeywordStore, os *store.OwnershipStore, clickCh chan<- store.ClickEvent) *ResolveHandler {
	return &ResolveHandler{links: ls, keywords: ks, ownership: os, clickCh: clickCh}
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
	// Governing: SPEC-0016 REQ "Prometheus Metrics Endpoint", ADR-0016
	start := time.Now()
	defer func() { metrics.RedirectDuration.Observe(time.Since(start).Seconds()) }()

	// Extract the full path after the leading "/".
	// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution", ADR-0013
	fullPath := strings.TrimPrefix(r.URL.Path, "/")
	if fullPath == "" {
		metrics.RedirectsTotal.WithLabelValues("not_found").Inc()
		h.render404(w, r, "")
		return
	}

	host := strings.SplitN(r.Host, ":", 2)[0]

	// Path-based keyword routing: /{keyword}/{slug} on the main server.
	// The browser extension redirects to {baseURL}/{keyword}/{slug} when the
	// keyword hostname isn't the server itself (Firefox fallback).
	// Governing: SPEC-0008 REQ "Search Interception and Redirect"
	parts := strings.SplitN(fullPath, "/", 2)
	if len(parts) == 2 && parts[1] != "" && parts[0] != host {
		if kw, err := h.keywords.GetByKeyword(r.Context(), parts[0]); err == nil {
			target := strings.ReplaceAll(kw.URLTemplate, "{slug}", parts[1])
			metrics.RedirectsTotal.WithLabelValues("found").Inc()
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
	}

	// Governing: ADR-0011 — check if request host is a registered keyword.
	kw, kwErr := h.keywords.GetByKeyword(r.Context(), host)
	if kwErr == nil {
		// Substitute {slug} in the URL template and redirect.
		target := strings.ReplaceAll(kw.URLTemplate, "{slug}", fullPath)
		metrics.RedirectsTotal.WithLabelValues("found").Inc()
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	// kwErr == store.ErrNotFound → fall through to normal slug resolution

	// Step 1: Try exact slug match on the full path.
	// Governing: SPEC-0009 REQ "Multi-Segment Path Resolution" — exact match wins
	link, err := h.links.GetBySlug(r.Context(), fullPath)
	if err == nil {
		// Governing: SPEC-0010 REQ "Secure Link Resolution", REQ "Public Link Resolution", REQ "Private Link Resolution"
		if !h.checkVisibility(w, r, link) {
			return
		}
		metrics.RedirectsTotal.WithLabelValues("found").Inc()
		h.redirect(w, r, link.ID, link.URL)
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

			// Governing: SPEC-0010 REQ "Secure Link Resolution"
			if !h.checkVisibility(w, r, link) {
				return
			}

			// Check if URL contains $varname placeholders.
			// Governing: SPEC-0009 REQ "Variable Substitution and Redirect", ADR-0013
			placeholders := varPlaceholderRe.FindAllString(link.URL, -1)
			if len(placeholders) == 0 {
				// Static link — redirect as-is.
				metrics.RedirectsTotal.WithLabelValues("found").Inc()
				h.redirect(w, r, link.ID, link.URL)
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
				metrics.RedirectsTotal.WithLabelValues("not_found").Inc()
				h.render404(w, r, fullPath)
				return
			}

			// Substitute positionally with url.PathEscape.
			target := link.URL
			for j, placeholder := range unique {
				target = strings.ReplaceAll(target, placeholder, url.PathEscape(remaining[j]))
			}

			metrics.RedirectsTotal.WithLabelValues("found").Inc()
			h.redirect(w, r, link.ID, target)
			return
		}
	}

	// No match found → 404.
	metrics.RedirectsTotal.WithLabelValues("not_found").Inc()
	h.render404(w, r, fullPath)
}

// checkVisibility enforces visibility rules for a link.
// Returns true if the request is allowed to proceed to redirect.
// Returns false if it has already written a response (login redirect or 403).
// Governing: SPEC-0010 REQ "Secure Link Resolution", REQ "Public Link Resolution", REQ "Private Link Resolution", REQ "Admin Visibility Override"
func (h *ResolveHandler) checkVisibility(w http.ResponseWriter, r *http.Request, link *store.Link) bool {
	switch link.Visibility {
	case "public", "private":
		// Governing: SPEC-0010 REQ "Public Link Resolution" — 302 for anyone
		// Governing: SPEC-0010 REQ "Private Link Resolution" — 302 for anyone who knows the slug
		return true
	case "secure":
		user := auth.UserFromContext(r.Context())
		if user == nil {
			// Governing: SPEC-0010 REQ "Secure Link Resolution" — redirect to login with return URL
			returnURL := r.URL.RequestURI()
			http.Redirect(w, r, "/auth/login?redirect="+url.QueryEscape(returnURL), http.StatusFound)
			return false
		}
		// Governing: SPEC-0010 REQ "Admin Visibility Override" — admins always authorized
		if user.IsAdmin() {
			return true
		}
		// Check if user is an owner/co-owner
		isOwner, err := h.ownership.IsOwner(link.ID, user.ID)
		if err == nil && isOwner {
			return true
		}
		// Check link_shares
		hasShare, err := h.links.HasShare(r.Context(), link.ID, user.ID)
		if err == nil && hasShare {
			return true
		}
		// Not authorized
		h.render403(w, r)
		return false
	default:
		// Unknown visibility — treat as public
		return true
	}
}

// render403 renders a 403 Forbidden page.
// Governing: SPEC-0010 REQ "Secure Link Resolution"
func (h *ResolveHandler) render403(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	w.WriteHeader(http.StatusForbidden)
	data := notFoundPage{BasePage: newBasePage(r, user), User: user, Slug: ""}
	if isHTMX(r) {
		renderPageFragment(w, "403.html", "content", data)
		return
	}
	render(w, "403.html", data)
}

// redirect issues a 302 redirect, handling HTMX requests with HX-Redirect header.
// It also fires a non-blocking click event if the click channel is configured.
// Governing: SPEC-0016 REQ "Click Recording", ADR-0016
func (h *ResolveHandler) redirect(w http.ResponseWriter, r *http.Request, linkID, target string) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)
	} else {
		http.Redirect(w, r, target, http.StatusFound)
	}

	if h.clickCh != nil {
		var userID string
		if u := auth.UserFromContext(r.Context()); u != nil {
			userID = u.ID
		}
		ua := r.UserAgent()
		if len(ua) > 512 {
			ua = ua[:512]
		}
		// Governing: SPEC-0016 REQ "Click Data Schema" — strip query/fragment to prevent token leakage
		ref := r.Referer()
		if ref != "" {
			if u, err := url.Parse(ref); err == nil {
				u.RawQuery = ""
				u.Fragment = ""
				ref = u.String()
			}
		}
		if len(ref) > 2048 {
			ref = ref[:2048]
		}
		select {
		case h.clickCh <- store.ClickEvent{
			LinkID:    linkID,
			UserID:    userID,
			IPHash:    store.HashIP(realIP(r)),
			UserAgent: ua,
			Referrer:  ref,
		}:
		default: // Governing: SPEC-0016 REQ "Click Recording"
			log.Printf("analytics: click channel full, dropping event for link %s", linkID)
		}
	}
}

// realIP extracts the client IP from r.RemoteAddr (port stripped).
// Chi's middleware.RealIP already rewrites r.RemoteAddr from X-Real-IP / X-Forwarded-For.
func realIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// render404 renders the 404 page for a missing slug.
func (h *ResolveHandler) render404(w http.ResponseWriter, r *http.Request, slug string) {
	user := auth.UserFromContext(r.Context())
	w.WriteHeader(http.StatusNotFound)
	data := notFoundPage{BasePage: newBasePage(r, user), User: user, Slug: slug}
	if isHTMX(r) {
		renderPageFragment(w, "404.html", "content", data)
		return
	}
	render(w, "404.html", data)
}
