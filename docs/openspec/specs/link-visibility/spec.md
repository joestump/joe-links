# SPEC-0010: Link Visibility Modes

## Overview

This specification defines per-link visibility modes for joe-links: `public`, `private`, and `secure`. The `visibility` column on the `links` table controls who can discover and follow a link. A new `link_shares` table enables fine-grained access grants for secure links. The slug resolver, dashboard views, REST API, and link create/edit forms are all modified to respect visibility.

See ADR-0014 (Link Visibility Modes), ADR-0005 (Data Model), ADR-0003 (OIDC Auth), SPEC-0002 (Link Data Model), SPEC-0004 (Application Views and Routing), SPEC-0005 (REST API Layer).

---

## Requirements

### Requirement: Visibility Column on Links Table

The application MUST add a `visibility` TEXT column to the `links` table via a goose migration. The column MUST have a `NOT NULL` constraint and a default value of `'public'`. Valid values MUST be `'public'`, `'private'`, and `'secure'`. The application layer MUST validate that only these three values are accepted on link creation and update. Existing links MUST automatically receive `visibility = 'public'` with no manual migration action.

#### Scenario: Default Visibility Is Public

- **WHEN** a new link is created without specifying a visibility value
- **THEN** the link's `visibility` MUST be set to `'public'`

#### Scenario: Existing Links Remain Public

- **WHEN** the migration runs on a database with existing links
- **THEN** all existing links MUST have `visibility = 'public'`

#### Scenario: Invalid Visibility Rejected

- **WHEN** a link is created or updated with `visibility = 'hidden'` or any value other than `public`, `private`, or `secure`
- **THEN** the application MUST return a validation error and MUST NOT persist the change

---

### Requirement: Link Shares Table

The application MUST create a `link_shares` table via a goose migration with the following columns: `link_id` (TEXT, FK to `links.id`, ON DELETE CASCADE), `user_id` (TEXT, FK to `users.id`, ON DELETE CASCADE), `shared_by` (TEXT, FK to `users.id`), `created_at` (DATETIME, NOT NULL, DEFAULT CURRENT_TIMESTAMP). The composite `(link_id, user_id)` MUST be the primary key. The `link_shares` table MUST have a composite index on `(link_id, user_id)` for efficient access checks.

#### Scenario: Share Record Created

- **WHEN** a link owner shares a secure link with another user
- **THEN** a row MUST be inserted into `link_shares` with the link's ID, the target user's ID, and the sharing user's ID

#### Scenario: Duplicate Share Rejected

- **WHEN** a link owner attempts to share a link with a user who already has a `link_shares` record for that link
- **THEN** the application MUST return an error and MUST NOT insert a duplicate row

#### Scenario: Link Deletion Cascades Shares

- **WHEN** a link is deleted
- **THEN** all `link_shares` rows for that link MUST be deleted via CASCADE

#### Scenario: User Deletion Cascades Shares

- **WHEN** a user is deleted
- **THEN** all `link_shares` rows where the user is the `user_id` MUST be deleted via CASCADE

---

### Requirement: Public Link Resolution

Links with `visibility = 'public'` MUST behave exactly as links behave today. The slug resolver (`GET /{slug}`) MUST redirect any request to the stored URL with `302 Found`, regardless of whether the user is authenticated. No additional database queries MUST be required beyond the existing slug lookup.

#### Scenario: Public Link Redirects Unauthenticated User

- **WHEN** an unauthenticated user navigates to `/{slug}` for a public link
- **THEN** the server MUST respond with `302 Found` and `Location` set to the stored URL

#### Scenario: Public Link Redirects Authenticated User

- **WHEN** an authenticated user navigates to `/{slug}` for a public link
- **THEN** the server MUST respond with `302 Found` and `Location` set to the stored URL

---

### Requirement: Private Link Resolution

Links with `visibility = 'private'` MUST redirect for anyone who knows the slug. The slug resolver MUST redirect any request (authenticated or not) to the stored URL with `302 Found`. Private links MUST NOT appear in public browsing views (SPEC-0012) or in other users' dashboard searches. The only difference from public links is discoverability, not access.

#### Scenario: Private Link Redirects Unauthenticated User

- **WHEN** an unauthenticated user navigates to `/{slug}` for a private link
- **THEN** the server MUST respond with `302 Found` and `Location` set to the stored URL

#### Scenario: Private Link Hidden from Public Browser

- **WHEN** a private link exists
- **THEN** it MUST NOT appear in the public link browser (`GET /links`) or on user profile pages (`GET /u/{slug}`)

#### Scenario: Private Link Visible to Owner on Dashboard

- **WHEN** a link owner views their dashboard
- **THEN** their private links MUST appear in their link list

---

### Requirement: Secure Link Resolution

Links with `visibility = 'secure'` MUST require authentication and explicit authorization before redirecting. The slug resolver MUST check the following access rules in order:

1. If the user is NOT authenticated, the server MUST redirect to `/auth/login` with a `return_url` query parameter set to `/{slug}` so the user returns to the link after login.
2. If the user IS authenticated, the server MUST check whether the user is an owner or co-owner (exists in `link_owners`) OR has a `link_shares` record for the link.
3. If the user IS authorized, the server MUST respond with `302 Found` to the stored URL.
4. If the user is NOT authorized, the server MUST respond with `403 Forbidden`.

Admins MUST always be authorized to access secure links, regardless of ownership or share records.

#### Scenario: Secure Link — Unauthenticated User

- **WHEN** an unauthenticated user navigates to `/{slug}` for a secure link
- **THEN** the server MUST redirect to `/auth/login?return_url=/{slug}`

#### Scenario: Secure Link — Owner Redirected

- **WHEN** an authenticated user who is an owner of the secure link navigates to `/{slug}`
- **THEN** the server MUST respond with `302 Found` to the stored URL

#### Scenario: Secure Link — Shared User Redirected

- **WHEN** an authenticated user who has a `link_shares` record for the link navigates to `/{slug}`
- **THEN** the server MUST respond with `302 Found` to the stored URL

#### Scenario: Secure Link — Unauthorized User Denied

- **WHEN** an authenticated user who is NOT an owner, co-owner, or shared user navigates to `/{slug}` for a secure link
- **THEN** the server MUST respond with `403 Forbidden`

#### Scenario: Secure Link — Admin Always Authorized

- **WHEN** an authenticated admin navigates to `/{slug}` for a secure link (even without ownership or share records)
- **THEN** the server MUST respond with `302 Found` to the stored URL

---

### Requirement: Dashboard Visibility Filtering

The user dashboard (`GET /dashboard`) MUST filter links based on visibility:

- **Public links**: MUST be shown to the owner in their own link list.
- **Private links**: MUST be shown only to owners and co-owners. MUST NOT appear in other users' search results or tag filters.
- **Secure links**: MUST be shown to owners, co-owners, and users with a `link_shares` record. A "Shared with me" section or filter SHOULD be available so users can find secure links shared with them.

The dashboard search and tag filter MUST respect visibility — searching for a private or secure link by another owner MUST NOT return results.

#### Scenario: Owner Sees All Own Links

- **WHEN** a user views their dashboard
- **THEN** all of their links (public, private, and secure) MUST appear in their link list

#### Scenario: Shared Secure Link Appears on Dashboard

- **WHEN** a user has a `link_shares` record for a secure link
- **THEN** that link SHOULD appear in a "Shared with me" section or be accessible via a filter

#### Scenario: Non-Owner Cannot Search Private Link

- **WHEN** user A searches for a slug that belongs to user B's private link
- **THEN** the search MUST NOT return that link

---

### Requirement: Admin Visibility Override

Admin views (`/admin/links`, `/api/v1/admin/links`) MUST display ALL links regardless of their visibility setting. Each link's visibility MUST be displayed as a badge or label in the admin links table. Admins MUST be able to change a link's visibility via the admin inline edit (SPEC-0011).

#### Scenario: Admin Sees All Links

- **WHEN** an admin visits `/admin/links`
- **THEN** all links MUST be displayed, including private and secure links

#### Scenario: Admin Sees Visibility Badge

- **WHEN** a link has `visibility = 'secure'`
- **THEN** the admin links table MUST display a "Secure" badge or label for that link

---

### Requirement: REST API Visibility Field

All link resources returned by the REST API (`/api/v1/links`, `/api/v1/links/{id}`) MUST include a `visibility` field in the JSON response. The `POST /api/v1/links` and `PUT /api/v1/links/{id}` endpoints MUST accept an optional `visibility` field in the request body. If omitted on creation, it MUST default to `'public'`. The API MUST enforce the same visibility-based access rules as the web UI: `GET /api/v1/links` for non-admin users MUST return only links the user owns, co-owns, or has been shared.

#### Scenario: API Link Response Includes Visibility

- **WHEN** `GET /api/v1/links/{id}` is called for a link with `visibility = 'private'`
- **THEN** the response MUST include `"visibility": "private"`

#### Scenario: API Create Link with Visibility

- **WHEN** `POST /api/v1/links` is called with `{"slug": "internal-tool", "url": "...", "visibility": "secure"}`
- **THEN** the link MUST be created with `visibility = 'secure'`

#### Scenario: API Create Link — Default Visibility

- **WHEN** `POST /api/v1/links` is called without a `visibility` field
- **THEN** the link MUST be created with `visibility = 'public'`

---

### Requirement: Link Share Management Endpoints

The application MUST provide endpoints for managing shares on secure links:

- `POST /dashboard/links/{id}/shares` — add a user to `link_shares` for the link. The request body MUST include an `email` field to identify the target user. Only link owners, co-owners, and admins MUST be authorized to add shares.
- `DELETE /dashboard/links/{id}/shares/{uid}` — remove a user from `link_shares` for the link. Only link owners, co-owners, and admins MUST be authorized to remove shares.

Both endpoints MUST support HTMX responses for inline updates on the link detail page.

#### Scenario: Add Share

- **WHEN** an owner submits `POST /dashboard/links/{id}/shares` with `email=bob@example.com`
- **THEN** a `link_shares` row MUST be created for the link and the user matching that email, and the shares list on the link detail page MUST be re-rendered

#### Scenario: Add Share — User Not Found

- **WHEN** an owner submits a share request with an email that does not match any user
- **THEN** the server MUST return a validation error indicating the user was not found

#### Scenario: Remove Share

- **WHEN** an owner submits `DELETE /dashboard/links/{id}/shares/{uid}`
- **THEN** the `link_shares` row MUST be deleted and the shares list MUST be re-rendered

#### Scenario: Non-Owner Cannot Manage Shares

- **WHEN** a user who is NOT an owner, co-owner, or admin attempts to add or remove a share
- **THEN** the server MUST return `403 Forbidden`

---

### Requirement: Link Share Management API Endpoints

The REST API MUST provide endpoints for managing link shares:

- `POST /api/v1/links/{id}/shares` — add a user to `link_shares`. Request body MUST include `email`. Only owners, co-owners, and admins MAY call this endpoint.
- `DELETE /api/v1/links/{id}/shares/{uid}` — remove a user from `link_shares`. Only owners, co-owners, and admins MAY call this endpoint.
- `GET /api/v1/links/{id}/shares` — list all `link_shares` records for the link. Only owners, co-owners, and admins MAY call this endpoint.

#### Scenario: API Add Share

- **WHEN** `POST /api/v1/links/{id}/shares` is called with `{"email": "bob@example.com"}`
- **THEN** a `link_shares` row MUST be created and the response MUST return `201 Created` with the share record

#### Scenario: API List Shares

- **WHEN** `GET /api/v1/links/{id}/shares` is called by an owner
- **THEN** the response MUST include all users with `link_shares` records for the link, with `user_id`, `email`, `display_name`, and `shared_by` fields

#### Scenario: API Remove Share

- **WHEN** `DELETE /api/v1/links/{id}/shares/{uid}` is called by an owner
- **THEN** the `link_shares` row MUST be deleted and the response MUST return `204 No Content`

---

### Requirement: Visibility Selector in Link Forms

The link create form (`GET /dashboard/links/new`) and link edit form (`GET /dashboard/links/{id}/edit`) MUST include a visibility selector. The selector MUST present three options: "Public" (default), "Private", and "Secure". Each option SHOULD include a brief description of its behavior. The selected visibility MUST be submitted with the form and persisted to the `visibility` column.

#### Scenario: Create Form Shows Visibility Selector

- **WHEN** a user opens the new link form
- **THEN** a visibility selector MUST be displayed with "Public" selected by default

#### Scenario: Edit Form Shows Current Visibility

- **WHEN** a user opens the edit form for a link with `visibility = 'secure'`
- **THEN** the visibility selector MUST show "Secure" as the selected option

#### Scenario: Visibility Updated on Edit

- **WHEN** a user changes a link's visibility from "Public" to "Private" and saves
- **THEN** the link's `visibility` column MUST be updated to `'private'`

---

### Requirement: Share Management Panel on Link Detail

The link detail page (`GET /dashboard/links/{id}`) MUST display a "Shared with" panel when the link has `visibility = 'secure'`. The panel MUST list all users with `link_shares` records for the link, showing their display name and email. The panel MUST include an "Add User" input (email field with search/autocomplete) and a "Remove" button for each shared user. The panel SHOULD be hidden when the link's visibility is not `secure`, but SHOULD appear immediately when visibility is changed to `secure` in the edit form.

#### Scenario: Share Panel Shown for Secure Link

- **WHEN** a user views the detail page of a secure link they own
- **THEN** a "Shared with" panel MUST be displayed listing all shared users

#### Scenario: Share Panel Hidden for Public Link

- **WHEN** a user views the detail page of a public link
- **THEN** the "Shared with" panel MUST NOT be displayed

#### Scenario: Share Panel Shows After Visibility Change

- **WHEN** a user changes a link's visibility to "Secure" and saves
- **THEN** the link detail page MUST display the "Shared with" panel

---

### Requirement: Database Migration

The application MUST implement the schema changes via goose migrations. The migration MUST:

1. Add `visibility TEXT NOT NULL DEFAULT 'public'` to the `links` table.
2. Create the `link_shares` table with the schema defined in this spec.
3. Create a composite index on `link_shares(link_id, user_id)`.

The down migration MUST drop the `link_shares` table and remove the `visibility` column from `links`.

#### Scenario: Migration Adds Visibility Column

- **WHEN** the up migration runs
- **THEN** the `links` table MUST have a `visibility` column with default `'public'`

#### Scenario: Migration Creates link_shares Table

- **WHEN** the up migration runs
- **THEN** the `link_shares` table MUST exist with the specified schema

#### Scenario: Down Migration Reverses Changes

- **WHEN** the down migration runs
- **THEN** the `link_shares` table MUST be dropped and the `visibility` column MUST be removed from `links`
