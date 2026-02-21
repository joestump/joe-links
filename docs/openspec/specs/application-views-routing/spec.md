# SPEC-0004: Application Views and Routing

## Overview

This specification defines the complete set of application routes, views (pages), shared layout components, and HTMX interaction patterns for joe-links. It covers the slug resolver (the core go-link redirect), the authenticated user dashboard, link CRUD views, tag browsing, admin views, and the landing page.

See ADR-0007 (Application Views and Routing), ADR-0001 (Technology Stack), SPEC-0001 (Core Web App), SPEC-0002 (Data Model), SPEC-0003 (UI Theming).

---

## Requirements

### Requirement: Route Registration and Priority

The chi router MUST register all named routes before the catch-all slug resolver (`GET /{slug}`). The following path prefixes MUST be reserved and MUST take precedence over slug resolution: `auth`, `static`, `dashboard`, `admin`. Route-level authorization MUST be enforced via chi middleware groups, not inside individual handlers.

#### Scenario: Reserved Prefix Takes Precedence

- **WHEN** a request arrives at `/dashboard`
- **THEN** the dashboard handler MUST be invoked; the slug resolver MUST NOT be invoked

#### Scenario: Static Asset Served

- **WHEN** a request arrives at `/static/css/app.css`
- **THEN** the embedded static file server MUST respond with the CSS; the slug resolver MUST NOT be invoked

#### Scenario: Slug Resolved After All Named Routes

- **WHEN** a request arrives at `/foobar` and `foobar` is not a registered named route
- **THEN** the slug resolver MUST be invoked

---

### Requirement: Landing Page (`GET /`)

The landing page MUST be served at `GET /` without authentication. If the requesting user has a valid session, the server MUST redirect them to `/dashboard`. If unauthenticated, the page MUST render a hero section explaining go-links, a "Sign in" call-to-action button linking to `/auth/login`, and a brief description of the service.

#### Scenario: Unauthenticated Root Visit

- **WHEN** an unauthenticated user visits `/`
- **THEN** the landing page MUST be rendered with a sign-in CTA

#### Scenario: Authenticated Root Visit

- **WHEN** an authenticated user visits `/`
- **THEN** the server MUST issue a `302 Found` redirect to `/dashboard`

---

### Requirement: User Dashboard (`GET /dashboard`)

The dashboard MUST be served at `GET /dashboard` and MUST require authentication. It MUST display all links the authenticated user owns or co-owns, a search/filter input, a tag filter control, and a "New Link" button. Links MUST be displayed with their slug, title (or URL if no title), description excerpt, tag chips, and edit/delete action controls. An empty state MUST be shown when the user has no links, with a prominent "Create your first link" prompt.

#### Scenario: Dashboard Shows Owned Links

- **WHEN** an authenticated user with existing links visits `/dashboard`
- **THEN** all links where the user appears in `link_owners` MUST be listed

#### Scenario: Dashboard Empty State

- **WHEN** an authenticated user with no links visits `/dashboard`
- **THEN** a friendly empty state with a link-creation CTA MUST be rendered

#### Scenario: Dashboard Search

- **WHEN** an authenticated user types in the search field (HTMX `hx-get` with debounce)
- **THEN** the link list MUST be replaced with a filtered fragment matching the query against slug, title, and description

#### Scenario: Dashboard Tag Filter

- **WHEN** an authenticated user clicks a tag chip in the filter row
- **THEN** the link list MUST be replaced with links tagged with the selected tag

---

### Requirement: New Link Form (`GET /dashboard/links/new` and `POST /dashboard/links`)

The new link form MUST be accessible via `GET /dashboard/links/new` (full-page fallback) and as an HTMX modal (`hx-get="/dashboard/links/new" hx-target="#modal"`). The form MUST include: slug (required, live-validated), URL (required), title (optional), description (optional), tags (optional, with autocomplete). Submission MUST go to `POST /dashboard/links`. On success, the browser MUST be redirected to `/dashboard` with a success toast. On error, the form MUST be re-rendered with inline validation messages.

#### Scenario: Successful Link Creation

- **WHEN** an authenticated user submits a valid slug and URL
- **THEN** the link MUST be created and the user MUST be redirected to `/dashboard`

#### Scenario: Live Slug Validation

- **WHEN** an authenticated user types in the slug field
- **THEN** an HTMX request MUST fire (debounced 300ms) to `GET /dashboard/links/validate-slug?slug=...` and render an inline availability indicator

#### Scenario: Slug Taken — Inline Error

- **WHEN** live slug validation returns a taken slug
- **THEN** a red inline indicator MUST appear beside the slug field without submitting the form

#### Scenario: Form Validation Error on Submit

- **WHEN** a user submits the new link form with an invalid slug or missing required fields
- **THEN** the form MUST be re-rendered with inline error messages for each invalid field

#### Scenario: Tag Autocomplete

- **WHEN** a user types in the tag input
- **THEN** an HTMX request MUST fire (debounced 200ms) to `GET /dashboard/tags/suggest?q=...` and render a dropdown of matching tags

---

### Requirement: Link Detail View (`GET /dashboard/links/{id}`)

A read-only detail page MUST be served at `GET /dashboard/links/{id}` for authenticated users who are owners or admins. It MUST display the full slug, URL (clickable), title, description, tags, and the list of co-owners. A copy button MUST copy the full go-link URL to the clipboard. Edit and Delete action buttons MUST be rendered for owners and admins.

#### Scenario: Detail View for Owner

- **WHEN** an authenticated owner visits `/dashboard/links/{id}`
- **THEN** the full link detail MUST be rendered with edit and delete controls visible

#### Scenario: Detail View Forbidden for Non-Owner

- **WHEN** a non-owner non-admin user visits `/dashboard/links/{id}`
- **THEN** the server MUST return `403 Forbidden`

#### Scenario: Copy Go-Link URL

- **WHEN** a user clicks the copy button
- **THEN** `navigator.clipboard.writeText(fullGoLinkURL)` MUST be invoked and a success toast MUST appear

---

### Requirement: Edit Link Form (`GET /dashboard/links/{id}/edit` and `PUT /dashboard/links/{id}`)

The edit form MUST be served at `GET /dashboard/links/{id}/edit` for owners and admins. The `slug` field MUST be rendered as read-only. All other fields (URL, title, description, tags) MUST be editable. Submission MUST go to `PUT /dashboard/links/{id}`. On success, the browser MUST be redirected to the link's detail page.

#### Scenario: Slug Read-Only on Edit

- **WHEN** an owner visits the edit form
- **THEN** the slug input MUST be rendered as a read-only/disabled field and MUST NOT be accepted in the PUT request body

#### Scenario: Successful Edit

- **WHEN** an owner submits valid edits
- **THEN** the link MUST be updated and the user MUST be redirected to `/dashboard/links/{id}`

#### Scenario: Edit by Non-Owner

- **WHEN** a non-owner non-admin user submits a PUT to `/dashboard/links/{id}`
- **THEN** the server MUST return `403 Forbidden`

---

### Requirement: Delete Link (`DELETE /dashboard/links/{id}`)

Link deletion MUST be initiated via HTMX with a confirmation modal (DaisyUI modal component). The DELETE request MUST only be issued after user confirmation. On success, the link row MUST be removed from the DOM via HTMX swap and a success toast MUST appear.

#### Scenario: Delete with Confirmation

- **WHEN** an owner clicks delete and confirms in the modal
- **THEN** `DELETE /dashboard/links/{id}` MUST be sent; on 200 response the link row MUST be removed from the DOM

#### Scenario: Delete Cancelled

- **WHEN** an owner clicks delete but dismisses the confirmation modal
- **THEN** no DELETE request MUST be sent and the link MUST remain in the list

#### Scenario: Delete by Non-Owner

- **WHEN** a non-owner sends `DELETE /dashboard/links/{id}`
- **THEN** the server MUST return `403 Forbidden`

---

### Requirement: Co-Owner Management

Co-owners MAY be added via `POST /dashboard/links/{id}/owners` and removed via `DELETE /dashboard/links/{id}/owners/{uid}`. Both endpoints MUST be accessible to link owners and admins. The primary owner (`is_primary = TRUE`) MUST NOT be removable. After add or remove, the owners section on the link detail page MUST be updated via HTMX swap.

#### Scenario: Add Co-Owner

- **WHEN** an owner submits a valid user email via the add co-owner form
- **THEN** the user MUST be added to `link_owners` and the owners list fragment MUST be re-rendered

#### Scenario: Remove Co-Owner

- **WHEN** an owner clicks remove on a co-owner
- **THEN** the user MUST be removed from `link_owners` and the owners list fragment MUST be re-rendered

#### Scenario: Remove Primary Owner Blocked

- **WHEN** any user attempts to remove the primary owner
- **THEN** the server MUST return `400 Bad Request` with an error message

---

### Requirement: Tag Browser (`GET /dashboard/tags` and `GET /dashboard/tags/{slug}`)

A tag browser MUST be served at `GET /dashboard/tags` showing all tags with link counts. Clicking a tag MUST navigate to `GET /dashboard/tags/{slug}` which renders a filtered link list. Both views MUST require authentication.

#### Scenario: Tag Browser Lists All Tags

- **WHEN** an authenticated user visits `/dashboard/tags`
- **THEN** all tags with at least one link MUST be displayed with their link counts

#### Scenario: Tag Detail Shows Filtered Links

- **WHEN** an authenticated user visits `/dashboard/tags/engineering`
- **THEN** all links tagged with `engineering` that the user owns or co-owns MUST be listed

#### Scenario: Tag with No Links

- **WHEN** a tag exists in `tags` but has no `link_tags` rows
- **THEN** it MUST NOT appear in the tag browser

---

### Requirement: Admin Dashboard (`GET /admin`, `/admin/users`, `/admin/links`)

Admin views MUST require the `admin` role, enforced by middleware. `GET /admin` MUST show summary statistics. `GET /admin/users` MUST list all users with their role, and role changes MUST be possible inline via HTMX `PUT /admin/users/{id}/role`. `GET /admin/links` MUST list all links across all users.

#### Scenario: Admin Accesses Admin Dashboard

- **WHEN** a user with role `admin` visits `/admin`
- **THEN** the admin dashboard MUST be rendered

#### Scenario: Non-Admin Blocked

- **WHEN** a user with role `user` accesses any `/admin/*` route
- **THEN** the middleware MUST return `403 Forbidden`

#### Scenario: Admin Changes User Role

- **WHEN** an admin submits a role change via `PUT /admin/users/{id}/role`
- **THEN** the user's role MUST be updated and the table row MUST be re-rendered via HTMX swap

---

### Requirement: Slug Resolver and 404 Page

`GET /{slug}` MUST be the last registered route. If the slug exists, the server MUST respond `302 Found` to the stored URL without authentication. If the slug does not exist, the server MUST render a friendly 404 page that includes the missing slug name, a "Create it now" button that pre-fills the slug in the new link form (requires auth; redirects to login if unauthenticated), and a search bar to find similarly-named links.

#### Scenario: Known Slug Redirects

- **WHEN** a request arrives at `/jira` and the slug `jira` exists
- **THEN** the server MUST respond `302 Found` with `Location` set to the stored URL

#### Scenario: Unknown Slug Renders 404

- **WHEN** a request arrives at `/foobar` and no link with slug `foobar` exists
- **THEN** the server MUST respond with a 404 page mentioning the slug and offering to create it

#### Scenario: 404 Create-It-Now

- **WHEN** an unauthenticated user clicks "Create it now" on the 404 page
- **THEN** they MUST be redirected to `/auth/login` with a redirect parameter that returns them to the new link form pre-filled with the slug

---

### Requirement: Shared Base Layout

All pages MUST use a base HTML layout template embedded via `go:embed`. Authenticated pages MUST include a navbar with: logo/wordmark, navigation links (Dashboard, Tags, Admin if role=admin), user avatar with a dropdown (sign out), and a theme toggle button. The layout MUST include an `id="modal"` target div for HTMX modal injection. The layout MUST include an `id="toast-area"` target for HTMX out-of-band toast notifications.

#### Scenario: Navbar Rendered for Authenticated User

- **WHEN** an authenticated user accesses any dashboard page
- **THEN** the navbar MUST include the user's display name and the theme toggle

#### Scenario: Admin Nav Link Shown Only to Admins

- **WHEN** a user with role `admin` views the navbar
- **THEN** an "Admin" link to `/admin` MUST be rendered

#### Scenario: Admin Nav Link Hidden for Non-Admins

- **WHEN** a user with role `user` views the navbar
- **THEN** no "Admin" link MUST appear in the navbar

#### Scenario: Toast Notification Delivered

- **WHEN** a handler returns an HTMX response with `HX-Reswap: outerHTML` on `#toast-area`
- **THEN** a toast notification MUST appear without a full page reload
