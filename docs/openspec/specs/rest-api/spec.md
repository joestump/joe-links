# SPEC-0005: REST API Layer

## Overview

This specification defines the JSON REST API for joe-links, served at `/api/v1`. The API provides programmatic access to all link management, tag browsing, and admin operations for CLI tools, browser extensions, and third-party integrations. It is additive alongside the existing HTMX web UI.

See ADR-0008 (REST API Layer), ADR-0009 (API Token Authentication), ADR-0001 (Technology Stack).

---

## Requirements

### Requirement: API Router Mounting

The application MUST mount a chi sub-router at the path prefix `/api/v1`. All routes under this prefix MUST return `Content-Type: application/json`. The `/api/v1` sub-router MUST be mounted before the catch-all slug resolver. The sub-router MUST apply a `BearerTokenMiddleware` that authenticates every request via `Authorization: Bearer <token>`.

#### Scenario: API Route Takes Precedence Over Slug Resolver

- **WHEN** a request arrives at `/api/v1/links`
- **THEN** the API router MUST handle the request; the slug resolver MUST NOT be invoked

#### Scenario: API Responses Are JSON

- **WHEN** any `/api/v1/*` endpoint is called
- **THEN** the response MUST include `Content-Type: application/json`

#### Scenario: Unauthenticated Request Rejected

- **WHEN** a request to any `/api/v1/*` endpoint lacks a valid `Authorization: Bearer <token>` header
- **THEN** the server MUST return `401 Unauthorized` with `{"error": "unauthorized"}`

---

### Requirement: Standard Error Response Format

All API error responses MUST use a consistent JSON structure. The response body MUST contain an `"error"` field with a human-readable message and a `"code"` field with a machine-readable constant.

```json
{
  "error": "slug already taken",
  "code": "SLUG_CONFLICT"
}
```

#### Scenario: Validation Error Shape

- **WHEN** any API endpoint returns a 4xx or 5xx status
- **THEN** the body MUST be a JSON object with at minimum an `"error"` string field

---

### Requirement: Links Collection (`GET /api/v1/links`, `POST /api/v1/links`)

`GET /api/v1/links` MUST return the list of links the authenticated user owns or co-owns. For users with role `admin`, ALL links in the system MUST be returned. The response MUST be a JSON object with a `"links"` array and pagination fields.

`POST /api/v1/links` MUST create a new link. The request body MUST include `slug` and `url`. `title`, `description`, and `tags` are optional. The slug MUST satisfy the format `[a-z0-9][a-z0-9\-]*[a-z0-9]` and MUST NOT match any reserved prefix.

#### Scenario: List Returns Owned Links

- **WHEN** an authenticated non-admin user calls `GET /api/v1/links`
- **THEN** the response MUST include only links where the user appears in `link_owners`

#### Scenario: Admin Sees All Links

- **WHEN** an authenticated admin user calls `GET /api/v1/links`
- **THEN** the response MUST include all links in the system

#### Scenario: Create Link Success

- **WHEN** `POST /api/v1/links` is called with a valid slug and URL
- **THEN** the server MUST return `201 Created` with the created link as JSON

#### Scenario: Create Link — Slug Conflict

- **WHEN** `POST /api/v1/links` is called with a slug that already exists
- **THEN** the server MUST return `409 Conflict` with `{"error": "slug already taken", "code": "SLUG_CONFLICT"}`

#### Scenario: Create Link — Invalid Slug

- **WHEN** `POST /api/v1/links` is called with a slug that fails format validation or uses a reserved prefix
- **THEN** the server MUST return `400 Bad Request` with a descriptive error

#### Scenario: Pagination

- **WHEN** `GET /api/v1/links?limit=10` is called and more than 10 links exist
- **THEN** the response MUST include `"next_cursor"` for the next page and `"links"` containing at most 10 items

---

### Requirement: Link Resource (`GET`, `PUT`, `DELETE /api/v1/links/{id}`)

`GET /api/v1/links/{id}` MUST return the full link resource for owners or admins.

`PUT /api/v1/links/{id}` MUST update the link's `url`, `title`, `description`, and `tags`. The `slug` field MUST be ignored in the request body (slugs are immutable after creation). Only owners or admins MAY update a link.

`DELETE /api/v1/links/{id}` MUST delete the link. Only owners or admins MAY delete a link.

#### Scenario: Get Link — Owner

- **WHEN** an owner calls `GET /api/v1/links/{id}`
- **THEN** the server MUST return `200 OK` with the full link JSON

#### Scenario: Get Link — Forbidden

- **WHEN** a non-owner non-admin calls `GET /api/v1/links/{id}`
- **THEN** the server MUST return `403 Forbidden`

#### Scenario: Update Link — Slug Is Immutable

- **WHEN** `PUT /api/v1/links/{id}` is called with a `slug` field in the body
- **THEN** the server MUST ignore the `slug` field and MUST NOT update it

#### Scenario: Delete Link

- **WHEN** an owner calls `DELETE /api/v1/links/{id}`
- **THEN** the server MUST return `204 No Content` and the link MUST no longer be resolvable

---

### Requirement: Co-Owner Management (`/api/v1/links/{id}/owners`)

`GET /api/v1/links/{id}/owners` MUST list all `link_owners` rows for the link.

`POST /api/v1/links/{id}/owners` MUST add a user as co-owner. The request body MUST include `email`. Only owners or admins MAY add co-owners.

`DELETE /api/v1/links/{id}/owners/{uid}` MUST remove the specified co-owner. The primary owner (`is_primary = TRUE`) MUST NOT be removable.

#### Scenario: Remove Primary Owner Blocked

- **WHEN** `DELETE /api/v1/links/{id}/owners/{uid}` targets the primary owner
- **THEN** the server MUST return `400 Bad Request` with `{"error": "cannot remove primary owner", "code": "PRIMARY_OWNER_PROTECTED"}`

---

### Requirement: Tags (`GET /api/v1/tags`, `GET /api/v1/tags/{slug}/links`)

`GET /api/v1/tags` MUST return all tags that have at least one link, including each tag's link count.

`GET /api/v1/tags/{slug}/links` MUST return all links tagged with the given slug that the authenticated user owns or co-owns (all for admins).

#### Scenario: Tags Without Links Hidden

- **WHEN** `GET /api/v1/tags` is called and a tag has no associated links
- **THEN** that tag MUST NOT appear in the response

---

### Requirement: User Profile (`GET /api/v1/users/me`)

`GET /api/v1/users/me` MUST return the authenticated user's profile: `id`, `email`, `display_name`, `role`, and `created_at`.

#### Scenario: Me Returns Caller Identity

- **WHEN** an authenticated user calls `GET /api/v1/users/me`
- **THEN** the server MUST return `200 OK` with the user's own profile

---

### Requirement: Admin Endpoints (`/api/v1/admin/*`)

All `/api/v1/admin/*` routes MUST require `role = admin`. A separate chi middleware group MUST enforce this.

`GET /api/v1/admin/users` MUST return all users.

`PUT /api/v1/admin/users/{id}/role` MUST update the specified user's role. The request body MUST include `role` (valid values: `user`, `admin`).

`GET /api/v1/admin/links` MUST return all links in the system.

#### Scenario: Non-Admin Blocked

- **WHEN** a user with role `user` calls any `/api/v1/admin/*` endpoint
- **THEN** the server MUST return `403 Forbidden`

#### Scenario: Role Update

- **WHEN** an admin calls `PUT /api/v1/admin/users/{id}/role` with `{"role": "admin"}`
- **THEN** the user's role MUST be updated and the response MUST return the updated user

---

### Requirement: Pagination

All list endpoints (`/api/v1/links`, `/api/v1/tags`, `/api/v1/admin/users`, `/api/v1/admin/links`) MUST support cursor-based pagination. The `?limit=N` parameter MUST be accepted (default 50, max 200). Responses MUST include a `"next_cursor"` field (opaque string) when more results exist, and `null` when on the last page.

#### Scenario: Default Limit Applied

- **WHEN** a list endpoint is called without a `?limit=` parameter
- **THEN** the response MUST return at most 50 items

#### Scenario: Limit Capped

- **WHEN** a list endpoint is called with `?limit=999`
- **THEN** the server MUST return at most 200 items (silently cap)

---

### Requirement: API Response Structures

All link resources in API responses MUST follow a consistent JSON shape:

```json
{
  "id": "uuid",
  "slug": "jira",
  "url": "https://company.atlassian.net/jira",
  "title": "Jira",
  "description": "Company Jira board",
  "tags": ["engineering", "tools"],
  "owners": [{"id": "uuid", "email": "user@example.com", "is_primary": true}],
  "created_at": "2026-02-21T12:00:00Z",
  "updated_at": "2026-02-21T12:00:00Z"
}
```

Tag resources:
```json
{
  "slug": "engineering",
  "link_count": 42
}
```
