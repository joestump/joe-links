---
status: draft
date: 2026-07-12
implements: [ADR-0018]
requires: [SPEC-0006, SPEC-0010]
---

# SPEC-0018: Agent Access via MCP

## Overview

Exposes joe-links to AI agents over the Model Context Protocol so that agents can create, look up, and share go-links conversationally. Per ADR-0018, the MCP server is an in-process Streamable HTTP endpoint at `/mcp` inside the existing joe-links binary, implemented with the official `modelcontextprotocol/go-sdk`, authenticated with existing personal access tokens (SPEC-0006 / ADR-0009), and delegating all behavior to the same store layer as the REST API (SPEC-0005 / ADR-0008).

The headline workflow: an agent holding a PAT (typically minted for a dedicated agent account) creates a link for content it produced, marks it `secure`, and shares it with a human by email — the human gets a working `go/slug` while the link never appears in public browsing (SPEC-0010).

## Requirements

### Requirement: MCP Endpoint

The server SHALL expose a Model Context Protocol server over the Streamable HTTP transport at `/mcp`, implemented with the official `modelcontextprotocol/go-sdk`. The endpoint MUST operate statelessly: every request MUST be self-contained, the server MUST NOT require session affinity, and server-initiated streams (subscriptions, sampling) are not offered in v1. The MCP server identity (`serverInfo.name`/`version`) MUST report `joe-links` and the build version from `internal/build`.

#### Scenario: Initialize handshake

- **WHEN** an authenticated MCP client POSTs an `initialize` request to `/mcp`
- **THEN** the server responds with protocol version negotiation, `serverInfo` naming `joe-links` and the build version, and a capabilities object advertising tools

#### Scenario: Restart transparency

- **WHEN** the joe-links process restarts between two tool calls from the same client
- **THEN** the second call succeeds without re-initialization errors caused by server-held session state

### Requirement: Bearer Token Authentication

Every `/mcp` request MUST be authenticated with a personal access token presented as `Authorization: Bearer jl_…`, verified by the same middleware/token-store path as `/api/v1` (SPEC-0006). Requests without a valid token MUST receive HTTP 401 with a `WWW-Authenticate: Bearer` header and MUST NOT be dispatched to the MCP handler. Session cookies MUST be ignored by this endpoint — the Authorization header is the only accepted credential. All tool invocations SHALL act as the token's user; the endpoint MUST update the token's last-used timestamp consistently with the REST API.

#### Scenario: Missing or invalid token

- **WHEN** a request to `/mcp` has no Authorization header, a malformed token, or a revoked token
- **THEN** the server responds 401 with `WWW-Authenticate: Bearer`, and no MCP message processing occurs

#### Scenario: Token revocation takes immediate effect

- **WHEN** a PAT used by an agent is revoked in the token management UI and the agent issues its next tool call
- **THEN** the call fails with 401 and no tool executes

#### Scenario: Cookie-bearing request

- **WHEN** a request to `/mcp` carries a valid web session cookie but no Authorization header
- **THEN** the server responds 401 exactly as if no credentials were presented

### Requirement: Tool Inventory

The MCP server SHALL expose the following tools, and in v1 only these tools. Each tool MUST declare a JSON Schema for its inputs, and slug/URL/visibility validation MUST be identical to the REST API's. Links MAY be referenced by `slug` or `id` wherever a link reference is accepted. Admin capabilities (user management, keyword CRUD, admin link listings) MUST NOT be exposed via MCP in v1.

| Tool | Purpose | Key inputs |
|------|---------|------------|
| `create_link` | Create a short link | `slug` (required), `url` (required), `title`, `description`, `tags[]`, `visibility`, `share_with[]` (emails) |
| `get_link` | Fetch one link incl. visibility, tags, owners, and (for owners of secure links) shares | `link` (slug or id) |
| `list_links` | Search links visible to the caller | `q`, `tag`, `filter` (`mine` \| `shared` \| `public`), `cursor`, `limit` |
| `update_link` | Modify an owned link | `link` + any of `url`, `title`, `description`, `tags[]`, `visibility` |
| `delete_link` | Delete an owned link | `link` |
| `share_link` | Grant a user access to a secure link | `link`, `email` |
| `unshare_link` | Revoke a share grant | `link`, `email` |
| `add_co_owner` | Add a co-owner by email | `link`, `email` |
| `get_link_stats` | Click totals and recent clicks | `link`, `limit` |
| `suggest_link_metadata` | LLM-proposed slug/title/description/tags for a URL (SPEC-0017) | `url` |
| `list_keywords` | List keyword templates (SPEC-0008) | — |

Tool results SHALL include the canonical short URL (e.g. `https://go.stump.rocks/slug`) in `create_link`, `get_link`, and `update_link` responses so agents can hand humans a working link without extra calls.

#### Scenario: Tool discovery

- **WHEN** an authenticated client sends `tools/list`
- **THEN** the response contains exactly the tools above (minus `suggest_link_metadata` when no LLM is configured), each with an input schema

#### Scenario: Create and hand back a working URL

- **WHEN** an agent calls `create_link` with slug `retro-notes` and a destination URL
- **THEN** the result reports success and contains the absolute short URL for `retro-notes`

### Requirement: Authorization Parity with the REST API

Every tool MUST enforce the same authorization rules as its corresponding `/api/v1` operation by delegating to the shared store/service layer. Tool handlers MUST NOT reimplement visibility, ownership, or share logic. Where UI and REST currently disagree, the REST behavior is normative for MCP.

#### Scenario: Non-owner mutation denied

- **WHEN** a caller invokes `update_link` or `delete_link` on a link they neither own nor co-own (and they are not admin)
- **THEN** the tool returns a permission-denied error and no change occurs

#### Scenario: Visibility respected in listing

- **WHEN** a caller invokes `list_links` with `filter: "public"`
- **THEN** results contain no `private` or `secure` links belonging to other users

### Requirement: Agent-Oriented Creation Defaults

`create_link` SHALL default `visibility` to `private` when the field is omitted. This deliberately diverges from the REST API's `public` default: agent-created links reference agent-produced content and MUST NOT appear in public browsing unless explicitly requested. When `share_with` is provided, the server MUST resolve every email to an existing user before creating the link; if any email is unknown, the tool MUST fail without creating the link and MUST name the unresolvable emails. When `share_with` is provided and `visibility` is omitted, visibility SHALL default to `secure` (a share grant is only enforced for secure links, per SPEC-0010).

#### Scenario: Default is private

- **WHEN** `create_link` is called without `visibility`
- **THEN** the created link has visibility `private`

#### Scenario: Share-with-me in one call

- **WHEN** an agent calls `create_link` with `share_with: ["joe@stump.rocks"]` and no `visibility`
- **THEN** the link is created with visibility `secure` and a share grant for that user, and the result contains the short URL

#### Scenario: Unknown share recipient

- **WHEN** `create_link` includes `share_with: ["nobody@example.com"]` and no such user exists
- **THEN** no link is created and the error names `nobody@example.com`

### Requirement: Structured Tool Errors

Tool failures MUST be returned as MCP tool results flagged as errors (not protocol-level errors) carrying a stable machine-readable code plus a human-readable message. Codes MUST reuse the REST API error vocabulary (e.g. `duplicate_slug`, `not_found`, `forbidden`, `validation_failed`) with one consistent casing. Protocol-level errors are reserved for malformed MCP messages and authentication failures.

#### Scenario: Duplicate slug

- **WHEN** `create_link` is called with a slug that already exists
- **THEN** the tool result is an error with code `duplicate_slug` and a message naming the slug, and the client session remains usable

### Requirement: Conditional Suggestion Tool

`suggest_link_metadata` MUST be registered only when an LLM provider is configured (SPEC-0017). When unregistered, it MUST NOT appear in `tools/list`.

#### Scenario: No LLM configured

- **WHEN** the server runs without LLM configuration and a client sends `tools/list`
- **THEN** `suggest_link_metadata` is absent

### Requirement: Observability

MCP usage SHALL be observable via the existing Prometheus registry (ADR-0016): a counter labelled by tool name and outcome (`success`/`error`) MUST be incremented per tool invocation, and request logging MUST include the acting user id (never the token).

#### Scenario: Tool call metrics

- **WHEN** `create_link` succeeds
- **THEN** the MCP tool-call counter for `create_link` with outcome `success` increments

### Requirement: Error Handling Standards

All error-producing operations MUST follow structured error handling:

- Errors MUST be wrapped with contextual information at each layer boundary (e.g., "mcp create_link: resolve share recipient: user lookup failed: …")
- Sentinel errors MUST be defined for domain-specific failure modes that callers need to distinguish programmatically
- Silent error swallowing MUST NOT occur — every error MUST be either returned to the caller, logged with sufficient context, or explicitly handled with a documented reason for suppression
- Structured logging MUST be used for error reporting (key-value pairs, not string interpolation)

#### Scenario: Store failure surfaces cleanly

- **WHEN** the database is unavailable during a `list_links` call
- **THEN** the tool returns an error result with code `internal_error`, and the underlying cause is logged with tool name and user id context

### Requirement: Database Operation Standards

All database operations MUST follow structured data access patterns:

- Transactions MUST be used for multi-step mutations that require atomicity — specifically, `create_link` with `share_with` MUST create the link, ownership, tags, and share grants atomically
- Connection lifecycle MUST be explicitly managed — connections MUST be returned to the pool after use, with timeouts configured
- Query parameters MUST use parameterized queries — string interpolation in queries MUST NOT occur

#### Scenario: Atomic create-and-share

- **WHEN** `create_link` with `share_with` fails while writing share grants
- **THEN** the link, ownership, and tag writes are rolled back and no partial link exists

## Security Requirements

<!-- Governing: ADR-0018 (Security-by-Default), SPEC-0016 REQ "Mandatory Security Section in Web Specs" -->

### Authentication

All endpoints MUST require authentication by default. Public (unauthenticated) endpoints MUST be explicitly listed with justification.

| Endpoint | Auth | Justification |
|----------|------|---------------|
| POST /mcp | Required | — |
| GET /mcp | Required | Stateless mode; MAY respond 405 when SSE streams are not offered, but never without auth evaluation first |
| DELETE /mcp | Required | Stateless mode; session teardown is a no-op but MUST NOT leak endpoint existence to unauthenticated callers |

### Rate Limiting

No application-level rate limiting in v1: the endpoint is PAT-gated, single-tenant, and fronted by Caddy on a private deployment; abuse equals a compromised token, whose remedy is revocation (SPEC-0006). This matches the existing `/api/v1` posture. If the REST API gains rate limiting, `/mcp` MUST adopt the same limits.

### Security Headers

All `/mcp` HTTP responses MUST include:

- `Content-Security-Policy`: `default-src 'none'` (JSON/SSE responses only; nothing is rendered)
- `X-Frame-Options`: DENY
- `X-Content-Type-Options`: nosniff
- `Referrer-Policy`: strict-origin-when-cross-origin

### Request Body Size Limits

All `/mcp` requests MUST be bounded with `http.MaxBytesReader`. Default limit: 1 MB. No tool accepts payloads that justify more.

### CSRF Protection

The endpoint accepts only `Authorization: Bearer` credentials and MUST ignore cookies entirely (see REQ "Bearer Token Authentication"); with no ambient credential, cross-site request forgery is not exploitable. No CSRF token is used, matching `/api/v1`.

### Redirect Validation

`/mcp` performs no HTTP redirects. Tool inputs containing destination URLs are stored, not followed; they are validated by the same URL validation as the REST API. Open redirects MUST NOT be introduced.

## References

- ADR-0018 (MCP server placement, transport, auth — implemented by this spec)
- ADR-0008 / SPEC-0005 (REST API layer — behavioral source of truth for parity)
- ADR-0009 / SPEC-0006 (PAT lifecycle and bearer middleware)
- SPEC-0010 (visibility modes and share grants)
- SPEC-0017 (LLM metadata suggestions — backing for `suggest_link_metadata`)
- SPEC-0008 (keyword templates — backing for `list_keywords`)
