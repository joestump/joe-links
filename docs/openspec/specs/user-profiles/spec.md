# SPEC-0012: User Profiles and Public Link Browsing

## Overview

This specification defines public-facing views for browsing links and viewing user profiles in joe-links: a public link browser at `/links` that lists all public links across all users with search and pagination, and per-user profile pages at `/u/{display_name_slug}` showing a user's public links and identity.

Both views only show links with `visibility = 'public'` (see SPEC-0010 for visibility modes). Until visibility is implemented, all links are treated as public.

See ADR-0007 (Application Views and Routing), SPEC-0004 (Application Views and Routing), SPEC-0002 (Link Data Model), SPEC-0010 (Link Visibility Modes).

---

## Requirements

### Requirement: Public Link Browser (`GET /links`)

The application MUST serve a public link browser at `GET /links`. This page MUST NOT require authentication. It MUST display all links with `visibility = 'public'` across all users. Each link entry MUST show: slug (as a clickable go-link), title (or URL if no title), description excerpt (truncated to 150 characters with ellipsis), tag chips, and the link owner's display name (primary owner). The page MUST be paginated with a default page size of 25 links. Links MUST be ordered by `created_at DESC` (newest first).

#### Scenario: Unauthenticated User Browses Links

- **WHEN** an unauthenticated user visits `/links`
- **THEN** all public links MUST be displayed without requiring login

#### Scenario: Authenticated User Browses Links

- **WHEN** an authenticated user visits `/links`
- **THEN** the same public link list MUST be displayed with the standard navbar rendered

#### Scenario: Only Public Links Shown

- **WHEN** a link has `visibility = 'private'` or `visibility = 'secure'`
- **THEN** it MUST NOT appear in the `/links` view

#### Scenario: Pagination Controls

- **WHEN** more than 25 public links exist
- **THEN** the page MUST display pagination controls (previous/next) and the current page MUST show at most 25 links

---

### Requirement: Public Link Search

The public link browser MUST include a search input at the top of the page. The search MUST filter links by slug, URL, title, and description using a case-insensitive substring match. The search MUST be implemented via HTMX (`hx-get` with debounce of 300ms) that replaces the link list fragment. The search MUST maintain the public visibility filter — only links with `visibility = 'public'` MUST appear in results. An empty search result MUST display a friendly "No links found" message.

#### Scenario: Search by Slug

- **WHEN** a user types "jira" in the search input
- **THEN** the link list MUST be replaced with public links whose slug, URL, title, or description contains "jira" (case-insensitive)

#### Scenario: Search Returns No Results

- **WHEN** a search query matches no public links
- **THEN** a "No links found" message MUST be displayed

#### Scenario: Search Resets on Clear

- **WHEN** the user clears the search input
- **THEN** the full unfiltered public link list MUST be restored

---

### Requirement: Public Link Browser Route Priority

The route `GET /links` MUST be registered as a named route before the catch-all slug resolver. It MUST be treated as a reserved prefix — the slug `links` MUST NOT be claimable by users. The route MUST use `OptionalUser` middleware so that authenticated users see the navbar while unauthenticated users see the page without it.

#### Scenario: /links Route Takes Precedence

- **WHEN** a request arrives at `/links`
- **THEN** the public link browser MUST be invoked; the slug resolver MUST NOT treat "links" as a slug

#### Scenario: Slug "links" Is Reserved

- **WHEN** a user attempts to create a link with slug `links`
- **THEN** the application MUST reject it as a reserved slug

---

### Requirement: User Profile Page (`GET /u/{display_name_slug}`)

The application MUST serve per-user profile pages at `GET /u/{display_name_slug}`. The `display_name_slug` MUST be derived from the user's `display_name` by lowercasing, replacing spaces with hyphens, and stripping characters outside `[a-z0-9-]`. The page MUST NOT require authentication. The profile page MUST display: the user's display name as a heading, an avatar initial (first letter of display name, uppercase, rendered in a colored circle using DaisyUI avatar placeholder), and a list of the user's public links (links where the user appears in `link_owners` AND `visibility = 'public'`). Links MUST be displayed in the same format as the public link browser (slug, title, description excerpt, tags). The link list MUST be paginated with a default page size of 25. If the user has no public links, a "No public links" message MUST be displayed.

#### Scenario: User Profile Rendered

- **WHEN** a visitor navigates to `/u/alice-smith`
- **THEN** the profile page MUST display the user whose `display_name_slug` is "alice-smith", their avatar initial, and their public links

#### Scenario: User Not Found

- **WHEN** a visitor navigates to `/u/nonexistent-user`
- **THEN** the server MUST return a `404 Not Found` page

#### Scenario: Only Public Links on Profile

- **WHEN** a user has links with `visibility = 'private'` or `visibility = 'secure'`
- **THEN** those links MUST NOT appear on the user's profile page

#### Scenario: Profile Pagination

- **WHEN** a user has more than 25 public links
- **THEN** the profile page MUST display pagination controls

---

### Requirement: Display Name Slug Derivation and Lookup

The application MUST derive a URL-safe slug from each user's `display_name` for use in profile URLs. The derivation MUST: convert to lowercase, replace whitespace sequences with a single hyphen, strip all characters outside `[a-z0-9-]`, and collapse consecutive hyphens. The application SHOULD store the derived `display_name_slug` as a column on the `users` table for efficient lookup. The `display_name_slug` MUST be updated whenever the user's `display_name` changes. If two users would produce the same slug, the application MUST append a numeric suffix (e.g., `alice-smith-2`) to maintain uniqueness.

#### Scenario: Slug Derived from Display Name

- **WHEN** a user has `display_name = "Alice Smith"`
- **THEN** the derived `display_name_slug` MUST be `"alice-smith"`

#### Scenario: Special Characters Stripped

- **WHEN** a user has `display_name = "Joe O'Brien III"`
- **THEN** the derived `display_name_slug` MUST be `"joe-obrien-iii"`

#### Scenario: Duplicate Slug Suffixed

- **WHEN** two users would both derive the slug `"alice-smith"`
- **THEN** the second user MUST receive `"alice-smith-2"` as their `display_name_slug`

#### Scenario: Slug Updated on Name Change

- **WHEN** a user changes their `display_name`
- **THEN** their `display_name_slug` MUST be recalculated

---

### Requirement: User Profile Route Priority

The route prefix `/u/` MUST be registered as a named route before the catch-all slug resolver. The slug prefix `u` MUST be added to the reserved slugs list. The route MUST use `OptionalUser` middleware for consistent navbar rendering.

#### Scenario: /u/ Route Takes Precedence

- **WHEN** a request arrives at `/u/alice`
- **THEN** the user profile handler MUST be invoked; the slug resolver MUST NOT be invoked

#### Scenario: Slug "u" Is Reserved

- **WHEN** a user attempts to create a link with slug `u`
- **THEN** the application MUST reject it as a reserved slug

---

### Requirement: Owner Name Linking in Public Views

In the public link browser, each link's owner display name MUST be rendered as a hyperlink to the owner's profile page (`/u/{display_name_slug}`). If a link has multiple owners, only the primary owner's name MUST be displayed in the list view.

#### Scenario: Owner Name Links to Profile

- **WHEN** a link owned by "Alice Smith" is displayed in the public link browser
- **THEN** "Alice Smith" MUST be rendered as a link to `/u/alice-smith`

#### Scenario: Multiple Owners — Primary Shown

- **WHEN** a link has a primary owner "Alice" and co-owner "Bob"
- **THEN** only "Alice" MUST be displayed as the owner in the public link browser

---

### Requirement: Database Migration for display_name_slug

The application MUST add a `display_name_slug` column to the `users` table via a goose migration. The column MUST be `TEXT NOT NULL DEFAULT ''` with a UNIQUE index. The migration MUST populate the column for all existing users by deriving the slug from their current `display_name`. The migration MUST handle duplicate slugs by appending numeric suffixes.

#### Scenario: Migration Populates Existing Users

- **WHEN** the migration runs on an existing database with users
- **THEN** every user MUST have a non-empty `display_name_slug` value derived from their `display_name`

#### Scenario: Unique Index Enforced

- **WHEN** a duplicate `display_name_slug` would be inserted
- **THEN** the database MUST reject the insert via the unique index
