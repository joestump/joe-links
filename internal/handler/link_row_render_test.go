package handler

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joestump/joe-links/internal/store"
)

// Governing: SPEC-0014 REQ "Abstract Link Widget" — verify the shared link_row
// partial and admin/profile renderers execute against their real data shapes.
func sampleAdminLink() *store.AdminLink {
	al := &store.AdminLink{
		Owners:    "Alice",
		Tags:      "go,docs",
		OwnerSlug: "alice",
		IsOwner:   true,
	}
	al.ID = "00000000-0000-0000-0000-000000000001"
	al.Slug = "example"
	al.URL = "https://example.com/$page"
	al.Title = "Example"
	al.Description = "An example link"
	al.Visibility = "public"
	al.CreatedAt = time.Now()
	return al
}

func TestLinkRowRenders_AdminAndDashboard(t *testing.T) {
	link := sampleAdminLink()

	cases := []struct {
		name      string
		path      string
		wantHrefs []string
	}{
		{"admin", "/admin/links/1/row", []string{"/admin/links/", "/admin/links/" + link.ID + "/confirm-delete"}},
		{"dashboard", "/dashboard", []string{"/dashboard/links/" + link.ID + "/stats", "/dashboard/links/" + link.ID + "/confirm-delete"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			ctx := AdminLinksPage{
				BasePage:       newBasePage(r, nil),
				ShowTitle:      true,
				ShowOwner:      true,
				ShowTags:       true,
				ShowVisibility: true,
				ShowActions:    true,
			}
			rr := httptest.NewRecorder()
			renderFragment(rr, "link_row", map[string]any{"Link": link, "Ctx": ctx})
			body := rr.Body.String()
			if !strings.Contains(body, `id="link-`+link.ID+`"`) {
				t.Fatalf("missing row id; body=%s", body)
			}
			// shared copy button + keyword prefix present
			if !strings.Contains(body, "Copy link") {
				t.Errorf("missing copy button")
			}
			for _, h := range tc.wantHrefs {
				if !strings.Contains(body, h) {
					t.Errorf("expected %q in body; body=%s", h, body)
				}
			}
		})
	}
}

func TestAdminEditRowRenders(t *testing.T) {
	link := sampleAdminLink()
	r := httptest.NewRequest("GET", "/admin/links/1/edit", nil)
	data := adminLinkRowData(r, link)
	rr := httptest.NewRecorder()
	renderPageFragment(rr, "admin/links.html", "admin_link_edit_row", data)
	body := rr.Body.String()
	if !strings.Contains(body, `id="link-`+link.ID+`"`) {
		t.Fatalf("edit row id mismatch; body=%s", body)
	}
	if !strings.Contains(body, `hx-target="#link-`+link.ID+`"`) {
		t.Errorf("edit row save/cancel should target #link-<id>; body=%s", body)
	}
}

func TestProfilePageRenders(t *testing.T) {
	link := sampleAdminLink()
	r := httptest.NewRequest("GET", "/u/alice", nil)
	data := ProfilePage{
		BasePage:       newBasePage(r, nil),
		ProfileUser:    &store.User{DisplayName: "Alice", DisplayNameSlug: "alice"},
		Links:          []*store.AdminLink{link},
		Page:           1,
		TotalPages:     1,
		TotalLinks:     1,
		ShowOwner:      true,
		ShowTags:       true,
		ShowVisibility: true,
	}
	rr := httptest.NewRecorder()
	render(rr, "profile.html", data)
	body := rr.Body.String()
	if !strings.Contains(body, "table") || !strings.Contains(body, link.Slug) {
		t.Fatalf("profile page should render link_list table with the link; body=%s", body)
	}
}

// Governing: SPEC-0014 — the dashboard renders link_list with []*store.Link (no
// AdminLink owner/tag fields). Guards must keep those fields un-evaluated so the
// dashboard never 500s on the shared partial.
func TestDashboardLinkListRenders_StoreLink(t *testing.T) {
	r := httptest.NewRequest("GET", "/dashboard", nil)
	l := &store.Link{
		ID:          "00000000-0000-0000-0000-0000000000aa",
		Slug:        "dash-link",
		URL:         "https://example.com",
		Title:       "Dash",
		Description: "d",
		Visibility:  "public",
		CreatedAt:   time.Now(),
	}
	data := DashboardPage{
		BasePage:    newBasePage(r, nil),
		Links:       []*store.Link{l},
		ShowActions: true, // matches DashboardHandler.Show; all other Show* false
	}
	rr := httptest.NewRecorder()
	renderFragment(rr, "link_list", data)
	body := rr.Body.String()
	if !strings.Contains(body, `id="link-`+l.ID+`"`) {
		t.Fatalf("dashboard link_list missing row; body=%s", body)
	}
	if !strings.Contains(body, "/dashboard/links/"+l.ID+"/stats") {
		t.Errorf("dashboard row should show stats action; body=%s", body)
	}
}
