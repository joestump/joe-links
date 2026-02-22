// Governing: SPEC-0012 REQ "Public Link Browser (GET /links)", REQ "Public Link Search"
package handler

import (
	"math"
	"net/http"
	"strconv"

	"github.com/joestump/joe-links/internal/auth"
	"github.com/joestump/joe-links/internal/store"
)

const defaultPageSize = 25

// PublicLinksPage is the template data for the public link browser.
// Governing: SPEC-0012 REQ "Public Link Browser (GET /links)"
type PublicLinksPage struct {
	BasePage
	Links          []*store.AdminLink
	Query          string
	Tag            string // unused; present for link_list partial compatibility
	Keyword        string // unused; present for link_list partial compatibility
	Page           int
	TotalPages     int
	Total          int
	HasPrev        bool
	HasNext        bool
	PrevPage       int
	NextPage       int
	ShowTitle      bool
	ShowOwner      bool
	ShowTags       bool
	ShowVisibility bool
	ShowActions    bool
}

// PublicLinksHandler serves the public link browser at GET /links.
// Governing: SPEC-0012 REQ "Public Link Browser (GET /links)"
type PublicLinksHandler struct {
	links    *store.LinkStore
	keywords *store.KeywordStore
}

// NewPublicLinksHandler creates a new PublicLinksHandler.
func NewPublicLinksHandler(ls *store.LinkStore, ks *store.KeywordStore) *PublicLinksHandler {
	return &PublicLinksHandler{links: ls, keywords: ks}
}

// Index renders the public link browser with search and pagination.
// Governing: SPEC-0012 REQ "Public Link Browser (GET /links)", REQ "Public Link Search"
func (h *PublicLinksHandler) Index(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	query := r.URL.Query().Get("q")

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	currentUserID := ""
	if user != nil {
		currentUserID = user.ID
	}

	links, total, err := h.links.ListPublic(r.Context(), currentUserID, query, page, defaultPageSize)
	if err != nil {
		http.Error(w, "could not load links", http.StatusInternalServerError)
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(defaultPageSize)))
	if totalPages < 1 {
		totalPages = 1
	}

	keyword := ""
	if kws, _ := h.keywords.List(r.Context()); len(kws) > 0 {
		keyword = kws[0].Keyword
	}

	data := PublicLinksPage{
		BasePage:       newBasePage(r, user),
		Links:          links,
		Query:          query,
		Keyword:        keyword,
		Page:           page,
		TotalPages:     totalPages,
		Total:          total,
		HasPrev:        page > 1,
		HasNext:        page < totalPages,
		PrevPage:       page - 1,
		NextPage:       page + 1,
		ShowOwner:      true,
		ShowTags:       true,
		ShowVisibility: true,
	}

	if isHTMX(r) {
		renderPageFragment(w, "links.html", "content", data)
		return
	}
	render(w, "links.html", data)
}
