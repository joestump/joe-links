# Tasks: Application Views and Routing

> Generated from SPEC-0004. See [spec.md](./spec.md) and [design.md](./design.md).

## 1. Router Setup

- [ ] 1.1 Create `internal/server/router.go` — instantiate chi router, apply global middleware (SCS session, logging, recovery) (ADR-0001, ADR-0003)
- [ ] 1.2 Register static file server at `/static/*` using `go:embed` (SPEC-0001 REQ "Go HTTP Server")
- [ ] 1.3 Register public routes: `GET /`, `GET /auth/login`, `GET /auth/callback`, `POST /dashboard/theme` (REQ "Route Registration and Priority")
- [ ] 1.4 Register authenticated route group with `RequireAuth` middleware: all `/dashboard/*` routes (REQ "Route Registration and Priority")
- [ ] 1.5 Register admin route group with `RequireAuth` + `RequireAdmin` middleware: all `/admin/*` routes (REQ "Route Registration and Priority", REQ "Admin Dashboard")
- [ ] 1.6 Register slug catch-all **last**: `GET /{slug}` (REQ "Route Registration and Priority", REQ "Slug Resolver and 404 Page")
- [ ] 1.7 Verify reserved prefix precedence: request to `/dashboard` MUST not invoke slug resolver (REQ "Route Registration and Priority", Scenario "Reserved Prefix Takes Precedence")

## 2. Templates and Base Layout

- [ ] 2.1 Create `templates/layouts/base.html` — html, head (inline anti-flash script, CSS), navbar, `#modal`, `#toast-area`, page content slot, footer (REQ "Shared Base Layout")
- [ ] 2.2 Create `templates/layouts/minimal.html` — logo + content slot only, for landing and 404 (REQ "Shared Base Layout")
- [ ] 2.3 Implement navbar template partial — logo, nav links (Dashboard, Tags, conditional Admin), user avatar dropdown (sign out), theme toggle (REQ "Shared Base Layout", Scenario "Admin Nav Link Shown Only to Admins")
- [ ] 2.4 Implement toast partial — `hx-swap-oob="true"` target `#toast-area`; DaisyUI alert component; auto-dismiss (REQ "Shared Base Layout", Scenario "Toast Notification Delivered")
- [ ] 2.5 Embed all templates via `//go:embed templates/**/*.html` in `internal/server/` (SPEC-0001 REQ "Go HTTP Server")

## 3. Landing Page

- [ ] 3.1 Create `templates/landing.html` — hero section, sign-in CTA link to `/auth/login` (REQ "Landing Page", Scenario "Unauthenticated Root Visit")
- [ ] 3.2 Implement `GET /` handler — redirect to `/dashboard` if authenticated, render landing if not (REQ "Landing Page", Scenario "Authenticated Root Visit")

## 4. Dashboard and Link List

- [ ] 4.1 Create `templates/dashboard.html` and `templates/dashboard-fragment.html` — link list (cards or table rows), search bar, tag filter chips, "New Link" button, empty state (REQ "User Dashboard")
- [ ] 4.2 Implement `GET /dashboard` handler — call `LinkStore.ListByOwner`; support `?q=` search param and `?tag=` filter param; return fragment if `HX-Request` present (REQ "User Dashboard", Scenario "Dashboard Shows Owned Links")
- [ ] 4.3 Wire search bar: `hx-get="/dashboard" hx-trigger="input delay:400ms" hx-target="#link-list"` (REQ "User Dashboard", Scenario "Dashboard Search")
- [ ] 4.4 Wire tag filter chips: `hx-get="/dashboard?tag={slug}" hx-target="#link-list"` (REQ "User Dashboard", Scenario "Dashboard Tag Filter")

## 5. New Link Form

- [ ] 5.1 Create `templates/link-new.html` and `templates/link-new-modal.html` (modal fragment) — slug, URL, title, description, tag input fields with validation (REQ "New Link Form")
- [ ] 5.2 Implement `GET /dashboard/links/new` handler — render full-page form or modal fragment based on `HX-Request` (REQ "New Link Form")
- [ ] 5.3 Implement `POST /dashboard/links` handler — validate, call `LinkStore.Create`, redirect to `/dashboard` on success, re-render form with errors on failure (REQ "New Link Form", Scenario "Successful Link Creation")
- [ ] 5.4 Implement `GET /dashboard/links/validate-slug` HTMX endpoint — returns inline `<span>` fragment (green/red) (REQ "New Link Form", Scenario "Live Slug Validation", Scenario "Slug Taken — Inline Error")
- [ ] 5.5 Wire slug validation: `hx-get="/dashboard/links/validate-slug" hx-trigger="input delay:300ms" hx-target="#slug-status"` (REQ "New Link Form")
- [ ] 5.6 Implement tag suggest endpoint `GET /dashboard/tags/suggest?q=` — returns `<ul>` dropdown fragment from `TagStore.Suggest` (REQ "New Link Form", Scenario "Tag Autocomplete")
- [ ] 5.7 Wire tag autocomplete: `hx-get="/dashboard/tags/suggest" hx-trigger="input delay:200ms" hx-target="#tag-dropdown"` (REQ "New Link Form")

## 6. Link Detail View

- [ ] 6.1 Create `templates/link-detail.html` — slug, URL, title, description, tag chips, owners list, copy button, edit/delete controls (REQ "Link Detail View")
- [ ] 6.2 Implement `GET /dashboard/links/{id}` handler — load link with owners+tags; enforce owner-or-admin check (REQ "Link Detail View", Scenario "Detail View Forbidden for Non-Owner")
- [ ] 6.3 Implement copy button — `hx-on:click` calling `navigator.clipboard.writeText(...)` + OOB toast (REQ "Link Detail View", Scenario "Copy Go-Link URL")

## 7. Edit Link Form

- [ ] 7.1 Create `templates/link-edit.html` — pre-populated form; slug rendered read-only/disabled (REQ "Edit Link Form", Scenario "Slug Read-Only on Edit")
- [ ] 7.2 Implement `GET /dashboard/links/{id}/edit` handler — load link, enforce ownership (REQ "Edit Link Form")
- [ ] 7.3 Implement `PUT /dashboard/links/{id}` handler — validate, call `LinkStore.Update` (URL/title/desc/tags only), redirect to detail page on success (REQ "Edit Link Form", Scenario "Successful Edit")
- [ ] 7.4 Enforce ownership in PUT handler — return 403 for non-owners and non-admins (REQ "Edit Link Form", Scenario "Edit by Non-Owner")

## 8. Delete Link

- [ ] 8.1 Create `templates/link-confirm-delete.html` — DaisyUI `<dialog>` modal with confirm/cancel buttons (REQ "Delete Link")
- [ ] 8.2 Implement `GET /dashboard/links/{id}/confirm-delete` HTMX endpoint — returns modal fragment (REQ "Delete Link")
- [ ] 8.3 Implement `DELETE /dashboard/links/{id}` handler — enforce ownership, call `LinkStore.Delete`, return OOB toast + empty swap (REQ "Delete Link", Scenario "Delete with Confirmation", Scenario "Delete by Non-Owner")
- [ ] 8.4 Wire delete flow: confirm button `hx-delete="/dashboard/links/{id}" hx-target="#link-row-{id}" hx-swap="outerHTML"` (REQ "Delete Link")

## 9. Co-Owner Management

- [ ] 9.1 Create `templates/owners-list.html` partial — avatars/names, add-co-owner form, remove buttons (REQ "Co-Owner Management")
- [ ] 9.2 Implement `POST /dashboard/links/{id}/owners` — look up user by email, call `LinkStore.AddOwner`, return owners-list fragment (REQ "Co-Owner Management", Scenario "Add Co-Owner")
- [ ] 9.3 Implement `DELETE /dashboard/links/{id}/owners/{uid}` — call `LinkStore.RemoveOwner`; handle `ErrPrimaryOwnerImmutable` with 400 (REQ "Co-Owner Management", Scenario "Remove Primary Owner Blocked")

## 10. Tag Browser

- [ ] 10.1 Create `templates/tags.html` — grid of tag chips with link counts (REQ "Tag Browser")
- [ ] 10.2 Implement `GET /dashboard/tags` handler — call `TagStore.ListWithCounts`, filter out zero-count tags (REQ "Tag Browser", Scenario "Tag with No Links")
- [ ] 10.3 Create `templates/tag-detail.html` and fragment — filtered link list (REQ "Tag Browser")
- [ ] 10.4 Implement `GET /dashboard/tags/{slug}` handler — call `LinkStore.ListByTag`, filter to user-visible links (REQ "Tag Browser", Scenario "Tag Detail Shows Filtered Links")

## 11. Admin Views

- [ ] 11.1 Create `templates/admin.html` — summary stats (user count, link count) (REQ "Admin Dashboard", Scenario "Admin Accesses Admin Dashboard")
- [ ] 11.2 Implement `GET /admin` handler — aggregate stats query
- [ ] 11.3 Create `templates/admin-users.html` and row fragment — table with role toggle per row (REQ "Admin Dashboard")
- [ ] 11.4 Implement `GET /admin/users` handler (REQ "Admin Dashboard")
- [ ] 11.5 Implement `PUT /admin/users/{id}/role` HTMX handler — update role, return row fragment (REQ "Admin Dashboard", Scenario "Admin Changes User Role")
- [ ] 11.6 Create `templates/admin-links.html` — all-links table with owner column (REQ "Admin Dashboard")
- [ ] 11.7 Implement `GET /admin/links` handler — `LinkStore.ListAll` (admin only) (REQ "Admin Dashboard")
- [ ] 11.8 Verify `RequireAdmin` middleware blocks `user` role with 403 (REQ "Admin Dashboard", Scenario "Non-Admin Blocked")

## 12. Slug Resolver

- [ ] 12.1 Implement `GET /{slug}` handler — call `LinkStore.GetBySlug`; issue `302 Found` on hit; render 404 on `ErrNotFound` (REQ "Slug Resolver and 404 Page", Scenario "Known Slug Redirects")
- [ ] 12.2 Create `templates/404.html` — missing slug name, "Create it now" CTA with pre-filled slug, search bar (REQ "Slug Resolver and 404 Page", Scenario "Unknown Slug Renders 404")
- [ ] 12.3 Implement "Create it now" redirect — unauthenticated → `/auth/login?redirect=/dashboard/links/new?slug={slug}` (REQ "Slug Resolver and 404 Page", Scenario "404 Create-It-Now")

## 13. Auth Flow Wiring

- [ ] 13.1 Implement `RequireAuth` middleware — check SCS session for `user_id`; redirect to `/auth/login?redirect={current_path}` if missing (SPEC-0001 REQ "Role-Based Access Control")
- [ ] 13.2 Implement `RequireAdmin` middleware — check `role == "admin"` from session; return 403 if not (REQ "Admin Dashboard", Scenario "Non-Admin Blocked")
- [ ] 13.3 After successful OIDC callback, redirect to `redirect` query param if present, else `/dashboard` (SPEC-0001 REQ "OIDC-Only Authentication")
