# SPEC-0011: Admin Management Screens

## Overview

This specification defines the administrative management screens for joe-links: an enhanced admin links view at `/admin/links` with cross-user link management, an admin keywords screen at `/admin/keywords` with full CRUD, and improvements to the admin users screen at `/admin/users` including safe user deletion with link reassignment.

See ADR-0007 (Application Views and Routing), SPEC-0004 (Application Views and Routing), SPEC-0002 (Link Data Model), SPEC-0001 (Core Web App).

---

## Requirements

### Requirement: Admin Links Screen (`GET /admin/links`)

The admin links screen MUST be served at `GET /admin/links` and MUST require the `admin` role. It MUST display ALL links across all users in the system. Each link row MUST show: slug, URL (truncated with tooltip for long URLs), title, owner(s) display names (comma-separated), tag chips, created date, and action controls. The owner(s) column MUST display the `display_name` of each user in `link_owners` for the link. If a link has multiple owners, all display names MUST be shown. The screen MUST support search/filter by slug, URL, title, or owner display name via an HTMX search input with debounce.

#### Scenario: Admin Sees All Links

- **WHEN** a user with role `admin` visits `/admin/links`
- **THEN** ALL links in the system MUST be listed regardless of ownership

#### Scenario: Owner Display Names Shown

- **WHEN** a link has two owners with display names "Alice" and "Bob"
- **THEN** the owner(s) column MUST display both names (e.g., "Alice, Bob")

#### Scenario: Search Filters Links

- **WHEN** an admin types "jira" in the search field
- **THEN** the link list MUST be replaced with links whose slug, URL, title, or owner display name matches the query

#### Scenario: Non-Admin Blocked

- **WHEN** a user with role `user` visits `/admin/links`
- **THEN** the middleware MUST return `403 Forbidden`

---

### Requirement: Admin Inline Link Editing

An admin MUST be able to edit any link's URL, title, and description directly from the admin links screen. Clicking an "Edit" action on a link row MUST replace the row with an inline edit form (HTMX swap) containing editable fields for URL, title, and description. The slug MUST be displayed read-only. Submitting the inline form MUST issue a `PUT /admin/links/{id}` request. On success, the row MUST be re-rendered with the updated values via HTMX swap. On validation error, the inline form MUST be re-rendered with error messages.

#### Scenario: Inline Edit Activated

- **WHEN** an admin clicks "Edit" on a link row
- **THEN** the row MUST be replaced with an inline edit form showing the current URL, title, and description as editable fields

#### Scenario: Inline Edit Submitted Successfully

- **WHEN** an admin submits valid changes in the inline edit form
- **THEN** `PUT /admin/links/{id}` MUST update the link and the row MUST be re-rendered with updated values

#### Scenario: Inline Edit Cancelled

- **WHEN** an admin clicks "Cancel" on the inline edit form
- **THEN** the original read-only row MUST be restored without any changes

#### Scenario: Slug Not Editable

- **WHEN** the inline edit form is displayed
- **THEN** the slug field MUST be rendered as read-only text and MUST NOT be modifiable

---

### Requirement: Admin Link Deletion

An admin MUST be able to delete any link from the admin links screen. Clicking "Delete" on a link row MUST open a DaisyUI confirmation modal displaying the link's slug and URL. The modal MUST include a warning that deletion is permanent and will remove all ownership and tag associations. Confirming deletion MUST issue `DELETE /admin/links/{id}`. On success, the link row MUST be removed from the DOM via HTMX swap and a success toast MUST appear. The browser `confirm()` dialog MUST NOT be used.

#### Scenario: Delete Confirmation Modal

- **WHEN** an admin clicks "Delete" on a link row
- **THEN** a DaisyUI modal MUST appear showing the link's slug and a deletion warning

#### Scenario: Delete Confirmed

- **WHEN** an admin confirms deletion in the modal
- **THEN** `DELETE /admin/links/{id}` MUST be sent, the link and all `link_owners` and `link_tags` rows MUST be deleted (CASCADE), and the row MUST be removed from the DOM

#### Scenario: Delete Cancelled

- **WHEN** an admin dismisses the confirmation modal
- **THEN** no DELETE request MUST be sent and the link MUST remain in the list

---

### Requirement: Admin Keywords Screen (`GET /admin/keywords`)

The admin keywords screen MUST be served at `GET /admin/keywords` and MUST require the `admin` role. It MUST display all keywords from the `keywords` table in a table format showing: keyword, URL template, description, and created date. The screen MUST include a "New Keyword" form (inline or modal) with fields: keyword (required, unique), URL template (required, MUST contain `{slug}` placeholder), and description (optional). Submitting the form MUST issue `POST /admin/keywords`. Successful creation MUST re-render the keywords table via HTMX swap. Each keyword row MUST have a "Delete" action that opens a DaisyUI confirmation modal. Confirming deletion MUST issue `DELETE /admin/keywords/{id}` and remove the row from the DOM via HTMX swap.

#### Scenario: Keywords Listed

- **WHEN** an admin visits `/admin/keywords`
- **THEN** all keywords MUST be listed with their URL template, description, and creation date

#### Scenario: Create Keyword

- **WHEN** an admin submits a valid keyword and URL template containing `{slug}`
- **THEN** a new keyword MUST be created and the keywords table MUST be re-rendered

#### Scenario: Duplicate Keyword Rejected

- **WHEN** an admin submits a keyword that already exists
- **THEN** the form MUST display an inline error indicating the keyword is already taken and MUST NOT create a duplicate

#### Scenario: URL Template Missing Placeholder

- **WHEN** an admin submits a URL template that does not contain `{slug}`
- **THEN** the form MUST display a validation error and MUST NOT create the keyword

#### Scenario: Delete Keyword with Confirmation

- **WHEN** an admin clicks "Delete" on a keyword row and confirms in the modal
- **THEN** `DELETE /admin/keywords/{id}` MUST be sent and the row MUST be removed from the DOM

---

### Requirement: Admin User Deletion with Link Handling

The admin users screen (`GET /admin/users`) MUST provide a "Delete" action for each non-admin user. Clicking "Delete" MUST open a DaisyUI confirmation modal (not `window.confirm()`). The modal MUST display the user's email and display name, a count of links the user owns, and a radio selection for link disposition: "Reassign links to me" (transfer all `link_owners` rows to the admin performing the deletion) or "Delete all links" (cascade delete all links where the user is the sole owner). If the user is a co-owner (not `is_primary`) on links, those `link_owners` rows MUST always be removed (no reassignment needed for co-ownership). The admin MUST NOT be able to delete themselves. Confirming deletion MUST issue `DELETE /admin/users/{id}` with a `link_action` parameter (`reassign` or `delete`). On success, the user row MUST be removed from the DOM via HTMX swap.

#### Scenario: Delete User Modal Shown

- **WHEN** an admin clicks "Delete" on a user row
- **THEN** a DaisyUI modal MUST appear showing the user's email, link count, and link disposition options

#### Scenario: Delete User — Reassign Links

- **WHEN** an admin confirms deletion with "Reassign links to me"
- **THEN** all links where the deleted user is the primary owner MUST have their `link_owners` rows transferred to the admin, the user's co-ownership rows MUST be removed, the user and their `api_tokens` MUST be deleted, and the user row MUST be removed from the DOM

#### Scenario: Delete User — Delete Links

- **WHEN** an admin confirms deletion with "Delete all links"
- **THEN** all links where the deleted user is the sole primary owner MUST be deleted (CASCADE), the user's co-ownership rows MUST be removed, the user and their `api_tokens` MUST be deleted, and the user row MUST be removed from the DOM

#### Scenario: Admin Cannot Delete Self

- **WHEN** an admin attempts to delete their own user record
- **THEN** the "Delete" action MUST NOT be available (button hidden or disabled) and the endpoint MUST return `400 Bad Request` if called directly

#### Scenario: Delete User — Shared Links Preserved

- **WHEN** a user being deleted is a co-owner (not primary) on a link
- **THEN** the co-ownership row MUST be removed but the link itself MUST NOT be deleted (it still has its primary owner)

---

### Requirement: Admin Link Deletion Endpoint (`PUT /admin/links/{id}`, `DELETE /admin/links/{id}`)

The server MUST provide `PUT /admin/links/{id}` and `DELETE /admin/links/{id}` endpoints. Both MUST require the `admin` role. `PUT` MUST accept `url`, `title`, and `description` fields and MUST NOT allow slug modification. `DELETE` MUST remove the link and all associated `link_owners` and `link_tags` rows (CASCADE). Both endpoints MUST support HTMX responses (check `HX-Request` header) returning appropriate HTML fragments for row re-render or removal.

#### Scenario: Admin Updates Link via PUT

- **WHEN** an admin submits `PUT /admin/links/{id}` with a new URL
- **THEN** the link's URL MUST be updated and `updated_at` MUST be set to the current time

#### Scenario: Admin Deletes Link via DELETE

- **WHEN** an admin submits `DELETE /admin/links/{id}`
- **THEN** the link and all associated ownership and tag rows MUST be permanently deleted

#### Scenario: Non-Admin Blocked from Admin Endpoints

- **WHEN** a user with role `user` calls `PUT /admin/links/{id}` or `DELETE /admin/links/{id}`
- **THEN** the middleware MUST return `403 Forbidden`

---

### Requirement: Admin User Deletion Endpoint (`DELETE /admin/users/{id}`)

The server MUST provide `DELETE /admin/users/{id}` requiring the `admin` role. The request MUST include a `link_action` form field or query parameter with value `reassign` or `delete`. If `reassign`, all links where the target user is primary owner MUST have their primary `link_owners` row updated to point to the requesting admin's user ID. If `delete`, all links where the target user is the sole owner MUST be deleted (CASCADE). In both cases, the target user's non-primary `link_owners` rows MUST be removed, and the user record MUST be deleted (which cascades to `api_tokens` and `sessions`). The endpoint MUST return `400 Bad Request` if the admin attempts to delete themselves. The endpoint MUST support HTMX responses.

#### Scenario: Self-Deletion Rejected

- **WHEN** an admin calls `DELETE /admin/users/{id}` with their own user ID
- **THEN** the server MUST return `400 Bad Request` with an error message

#### Scenario: Missing link_action Parameter

- **WHEN** `DELETE /admin/users/{id}` is called without a `link_action` parameter
- **THEN** the server MUST return `400 Bad Request` indicating the parameter is required
