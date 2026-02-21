# SPEC-0006: API Token Authentication — Personal Access Tokens

## Overview

This specification defines Personal Access Tokens (PATs) for authenticating programmatic clients to the REST API. PATs are user-generated opaque tokens stored hashed in the database, passed as `Authorization: Bearer <token>` on API requests, and managed via the web UI settings page and API.

See ADR-0009 (API Token Authentication), ADR-0008 (REST API Layer), ADR-0003 (OIDC Auth).

---

## Requirements

### Requirement: Token Format

All PATs MUST use the format `jl_<32-random-bytes-base62>`. The `jl_` prefix identifies the token type. The random component MUST provide at least 192 bits of entropy (32 bytes). Tokens MUST be generated using a cryptographically secure random source.

#### Scenario: Token Prefix

- **WHEN** a PAT is generated
- **THEN** the returned plaintext token MUST begin with `jl_`

#### Scenario: Token Entropy

- **WHEN** two PATs are generated
- **THEN** the random components MUST differ (trivially satisfied by 192-bit random)

---

### Requirement: Token Storage

Tokens MUST NOT be stored in plaintext. The server MUST store only the SHA-256 hash of the plaintext token in the `api_tokens` table. The plaintext token MUST be returned to the user exactly once (at creation time) and MUST NOT be recoverable afterward.

#### Scenario: Plaintext Not Recoverable

- **WHEN** a user visits the token management page after creating a token
- **THEN** the full plaintext token MUST NOT be displayed — only the token's name, prefix, creation date, and last-used date

#### Scenario: Token Hash Stored

- **WHEN** a token is created
- **THEN** the `api_tokens` table MUST contain `token_hash = sha256(plaintext_token)` and MUST NOT contain the plaintext

---

### Requirement: `api_tokens` Table

The application MUST create and maintain an `api_tokens` table with the following columns: `id` (UUID primary key), `user_id` (FK to `users.id`, CASCADE DELETE), `name` (human-readable label, required), `token_hash` (SHA-256 hex, UNIQUE), `last_used_at` (nullable timestamp), `expires_at` (nullable timestamp — NULL means no expiry), `created_at` (timestamp, NOT NULL DEFAULT), `revoked_at` (nullable timestamp — NULL means active).

#### Scenario: User Deletion Cascades

- **WHEN** a user record is deleted
- **THEN** all `api_tokens` rows for that user MUST be deleted automatically (CASCADE)

---

### Requirement: Bearer Token Middleware

The API sub-router MUST apply a `BearerTokenMiddleware` to all `/api/v1/*` routes. The middleware MUST:

1. Read the `Authorization` header; if absent or not `Bearer <token>`, return `401 Unauthorized`
2. Hash the presented token with SHA-256
3. Query `api_tokens` WHERE `token_hash = <hash>` AND `revoked_at IS NULL` AND (`expires_at IS NULL` OR `expires_at > NOW()`)
4. If no matching row, return `401 Unauthorized`
5. Load the associated `users` row; if the user is not found or has been deleted, return `401 Unauthorized`
6. Inject the user into the request context using the same context key as the OIDC middleware
7. Update `last_used_at` asynchronously (MUST NOT block the response)

#### Scenario: Valid Token Authenticates

- **WHEN** a request includes `Authorization: Bearer jl_<valid-token>`
- **THEN** the middleware MUST inject the token owner's user record into the context

#### Scenario: Expired Token Rejected

- **WHEN** a request includes a Bearer token whose `expires_at` is in the past
- **THEN** the middleware MUST return `401 Unauthorized`

#### Scenario: Revoked Token Rejected

- **WHEN** a request includes a Bearer token whose `revoked_at` is non-null
- **THEN** the middleware MUST return `401 Unauthorized`

#### Scenario: last_used_at Updated

- **WHEN** a valid token is used
- **THEN** `last_used_at` MUST be updated to the current time (asynchronously — MUST NOT delay the response)

---

### Requirement: Token Management API (`/api/v1/tokens`)

The application MUST expose token management endpoints under the authenticated `/api/v1` router (auth via Bearer token):

- `GET /api/v1/tokens` — list the caller's own tokens (name, id, last_used_at, expires_at, created_at; MUST NOT include token_hash or plaintext)
- `POST /api/v1/tokens` — create a new token; returns the plaintext token in the response **exactly once**
- `DELETE /api/v1/tokens/{id}` — revoke a token (sets `revoked_at`; MUST only revoke tokens belonging to the caller)

#### Scenario: Token Creation Returns Plaintext Once

- **WHEN** `POST /api/v1/tokens` is called with `{"name": "my-cli", "expires_at": "2027-01-01T00:00:00Z"}`
- **THEN** the response MUST include `{"token": "jl_xxxx", "id": "uuid", "name": "my-cli", ...}` and the plaintext `token` field MUST NOT appear in any subsequent API call

#### Scenario: Token List Hides Plaintext

- **WHEN** `GET /api/v1/tokens` is called
- **THEN** the response MUST NOT include the `token_hash` or any plaintext token value

#### Scenario: Revoke Own Token Only

- **WHEN** a user calls `DELETE /api/v1/tokens/{id}` for a token belonging to another user
- **THEN** the server MUST return `404 Not Found` (MUST NOT reveal existence of another user's tokens)

---

### Requirement: Token Management Web UI (`/dashboard/settings/tokens`)

The application MUST provide a web UI page at `GET /dashboard/settings/tokens` (requires authentication via OIDC session) that:

- Lists the user's active tokens (name, created_at, last_used_at, expires_at)
- Provides a "New Token" form with fields: name (required), expiry date (optional)
- Shows the plaintext token in a one-time reveal dialog after creation (with a prominent "copy and save — you won't see this again" warning)
- Allows revoking any active token with a confirmation dialog

#### Scenario: New Token Displayed Once

- **WHEN** a user creates a token via the web UI
- **THEN** a modal or alert MUST display the full plaintext token with instructions to copy it, and the token MUST NOT be displayed again upon page refresh

#### Scenario: Token Revocation Confirmation

- **WHEN** a user clicks "Revoke" on a token
- **THEN** a confirmation dialog MUST appear before the revocation is submitted

---

### Requirement: No Web UI Session on API Routes

The `BearerTokenMiddleware` MUST NOT fall back to SCS session authentication. API routes at `/api/v1/*` MUST exclusively use Bearer token auth. If a request to an API route includes a valid session cookie but no Bearer token, the server MUST return `401 Unauthorized`.

#### Scenario: Session Cookie Not Accepted on API

- **WHEN** a browser with a valid OIDC session calls `GET /api/v1/links` without an `Authorization` header
- **THEN** the server MUST return `401 Unauthorized`
