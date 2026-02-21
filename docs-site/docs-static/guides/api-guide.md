---
title: "API Guide"
sidebar_label: "API Guide"
sidebar_position: 4
---

# API Guide

joe-links provides a REST API at `/api/v1` for programmatic link management. All API requests require authentication via a Personal Access Token (PAT).

## Creating a Personal Access Token

1. Sign in to joe-links and go to the **Dashboard**.
2. Navigate to **Settings > API Tokens**.
3. Click **Create Token**, give it a name, and optionally set an expiration date.
4. Copy the token immediately -- it is only shown once and cannot be retrieved later.

## Authentication

Include your token in the `Authorization` header as a Bearer token:

```bash
curl -H "Authorization: Bearer jl_your_token_here" \
  https://go.example.com/api/v1/links
```

### Python (requests)

```python
import requests

headers = {"Authorization": "Bearer jl_your_token_here"}
resp = requests.get("https://go.example.com/api/v1/links", headers=headers)
links = resp.json()
```

## Pagination

List endpoints use cursor-based pagination. The response includes a `next_cursor` field when more results are available.

### Parameters

| Parameter | Default | Max | Description |
|-----------|---------|-----|-------------|
| `limit` | `50` | `200` | Number of items to return per page |
| `cursor` | -- | -- | Opaque cursor from a previous response's `next_cursor` |

### Example

Fetch the first page:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://go.example.com/api/v1/links?limit=10"
```

Response:

```json
{
  "links": [ ... ],
  "next_cursor": "eyJzbHVnIjoibXktbGluayJ9"
}
```

Fetch the next page using the cursor:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://go.example.com/api/v1/links?limit=10&cursor=eyJzbHVnIjoibXktbGluayJ9"
```

When `next_cursor` is `null`, you have reached the last page.

## Error Responses

All errors follow a consistent JSON shape:

```json
{
  "error": "human-readable error message",
  "code": "MACHINE_READABLE_CODE"
}
```

### Common Error Codes

| HTTP Status | Code | Description |
|-------------|------|-------------|
| 400 | `BAD_REQUEST` | Invalid request body or missing required fields |
| 400 | `INVALID_SLUG` | Slug format is invalid or uses a reserved prefix |
| 401 | `UNAUTHORIZED` | Missing or invalid Bearer token |
| 403 | `FORBIDDEN` | Authenticated but not authorized for this resource |
| 404 | `NOT_FOUND` | Resource does not exist |
| 409 | `SLUG_CONFLICT` | Slug is already taken |
| 409 | `DUPLICATE_OWNER` | User is already an owner of this link |

## API Reference

### Links

#### List Links

```
GET /api/v1/links?limit=50&cursor=...
```

Returns links owned by the authenticated user. Admins see all links.

#### Create a Link

```
POST /api/v1/links
```

```json
{
  "slug": "my-link",
  "url": "https://example.com",
  "title": "Example",
  "description": "An example link",
  "tags": ["example", "docs"]
}
```

The authenticated user becomes the primary owner. `slug` and `url` are required. `title`, `description`, and `tags` are optional.

#### Get a Link

```
GET /api/v1/links/{id}
```

Returns a single link. Only owners and admins may access.

#### Update a Link

```
PUT /api/v1/links/{id}
```

```json
{
  "url": "https://new-example.com",
  "title": "Updated Title",
  "description": "Updated description",
  "tags": ["updated"]
}
```

Updates the link's URL, title, description, and tags. The slug is immutable and cannot be changed.

#### Delete a Link

```
DELETE /api/v1/links/{id}
```

Returns `204 No Content` on success. Only owners and admins may delete.

### Co-Owners

#### List Owners

```
GET /api/v1/links/{id}/owners
```

Returns all owners of a link with their `id`, `email`, and `is_primary` flag.

#### Add a Co-Owner

```
POST /api/v1/links/{id}/owners
```

```json
{
  "email": "colleague@example.com"
}
```

The user must already have an account in joe-links. Returns `201` with the new owner entry.

#### Remove a Co-Owner

```
DELETE /api/v1/links/{id}/owners/{uid}
```

Returns `204 No Content`. The primary owner cannot be removed.

### Tokens

#### List Tokens

```
GET /api/v1/tokens
```

Returns all tokens for the authenticated user. Token hashes are never included.

#### Create a Token

```
POST /api/v1/tokens
```

```json
{
  "name": "CI Pipeline",
  "expires_at": "2026-12-31T23:59:59Z"
}
```

Returns the plaintext token in the response. This is the only time the token value is returned -- store it securely.

#### Revoke a Token

```
DELETE /api/v1/tokens/{id}
```

Soft-deletes the token. Returns `204 No Content`.

## Swagger UI

For interactive API exploration, visit `/api/docs/` on your joe-links instance. The Swagger UI provides a complete reference with request/response schemas and the ability to try requests directly.
