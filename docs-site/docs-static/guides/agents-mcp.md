---
title: "Agents (MCP)"
sidebar_label: "Agents (MCP)"
sidebar_position: 5
---

# Agents (MCP)

joe-links exposes a [Model Context Protocol](https://modelcontextprotocol.io) server at `/mcp`, so AI agents can create, look up, and share go-links conversationally. It runs inside the joe-links binary (Streamable HTTP transport, stateless), authenticates with the same [personal access tokens](../guides/api-guide.md) as the REST API, and enforces the same authorization rules — tools delegate to the same code paths as `/api/v1`.

Governing design artifacts: ADR-0018 and SPEC-0018 (Agent Access via MCP).

## Connect

**Claude Code** (one command):

```bash
claude mcp add --transport http joe-links https://go.example.com/mcp \
  --header "Authorization: Bearer jl_your_token_here"
```

**Claude Desktop / other clients**: add a remote MCP server pointing at `https://go.example.com/mcp` with an `Authorization: Bearer jl_…` header. Clients that only speak stdio can bridge with `mcp-remote`.

Tokens are minted in the web UI under **API Tokens** (`/dashboard/settings/tokens`) and can be revoked there at any time — revocation takes effect on the agent's next call.

### Recommended: a dedicated agent account

Agents work best with their own identity: sign an agent account in once via your OIDC provider, mint it a PAT, and give that token to your agents. Links the agent creates are honestly owned by the agent, and handing one to a human is an explicit `share_with` — which is exactly the flow the tools are optimized for. Running agents on your own PAT also works; sharing to yourself is then unnecessary.

## The share-with-me workflow

The headline use case is one tool call:

```json
{
  "name": "create_link",
  "arguments": {
    "slug": "retro-notes",
    "url": "https://docs.example.com/retro-2026-07",
    "title": "July retro notes",
    "share_with": ["joe@example.com"]
  }
}
```

This creates the link with **secure** visibility, grants `joe@example.com` access, and returns the working short URL (`https://go.example.com/retro-notes`) — all atomically. If any `share_with` email has no joe-links account yet, nothing is created and the error names the emails.

## Defaults that differ from the REST API

Agent-created links default to **`private`** (REST defaults to `public`): agent output should never land in the public Browse page unless asked. Providing `share_with` without an explicit `visibility` upgrades the default to **`secure`**, because share grants only gate secure links. An explicit `visibility` always wins.

## Tools

| Tool | Purpose | Key inputs |
|------|---------|------------|
| `create_link` | Create a link (returns `short_url`) | `slug`*, `url`*, `title`, `description`, `tags[]`, `visibility`, `share_with[]` |
| `get_link` | One link incl. owners and (for owners) shares | `link`* (slug or id) |
| `list_links` | Links visible to you | `q`, `tag`, `filter` (`mine`\|`shared`\|`public`), `limit` |
| `update_link` | Modify an owned link (omitted fields unchanged) | `link`* + `url`/`title`/`description`/`tags[]`/`visibility` |
| `delete_link` | Delete an owned link | `link`* |
| `share_link` | Grant a user access to a secure link | `link`*, `email`* |
| `unshare_link` | Revoke a grant | `link`*, `email`* |
| `add_co_owner` | Add a co-owner (can edit/delete) | `link`*, `email`* |
| `get_link_stats` | Click totals + recent clicks | `link`*, `limit` |
| `suggest_link_metadata` | LLM-suggested slug/title/description/tags | `url`* |
| `list_keywords` | Keyword templates on this server | — |

`suggest_link_metadata` only appears in `tools/list` when the server has an [LLM provider configured](../guides/configuration.md). Admin operations are deliberately not exposed over MCP.

## Error codes

Tool failures come back as MCP error results whose text content is machine-readable JSON: `{"code": "...", "message": "..."}`. Codes are stable:

| Code | Meaning |
|------|---------|
| `validation_failed` | Bad slug format, reserved slug, invalid URL/visibility, over-length text |
| `duplicate_slug` | Slug already exists — pick another |
| `not_found` | No link with that slug or id |
| `forbidden` | You lack access (not an owner/recipient/admin for this operation) |
| `unknown_user` | A `share_with`/`email` has no joe-links account (users must sign in once first) |
| `duplicate_share` / `duplicate_owner` | Grant or ownership already exists |
| `llm_error` | The configured LLM provider failed |
| `internal_error` | Server-side failure — check server logs |

Authentication failures are HTTP-level: `401` with a `WWW-Authenticate: Bearer` challenge (missing, invalid, expired, or revoked token).

## Authorization model

Identical to the REST API, verified by a cross-surface test matrix:

| Operation | Owner / co-owner | Share recipient | Anyone else | Admin |
|-----------|------------------|-----------------|-------------|-------|
| `get_link` | ✓ | ✓ | ✗ | ✓ |
| `update_link` / `delete_link` | ✓ | ✗ | ✗ | ✓ |
| `get_link_stats` | ✓ | ✗ | ✗ | ✓ |
| `share_link` / `unshare_link` / `add_co_owner` | ✓ | ✗ | ✗ | ✓ |
| `list_links filter:public` | public links only — never other users' `private`/`secure` | | | |

## Operational notes

- The endpoint is stateless: no session affinity, safe behind any proxy, survives server restarts mid-conversation.
- Request bodies are capped at 1 MB; responses carry strict security headers.
- Every tool call increments `joelinks_mcp_tool_calls_total{tool,outcome}` in the Prometheus registry (`/metrics`).
- `mcp` is a reserved slug — no go-link can shadow the endpoint.

## Troubleshooting

- **401 on every call** — token missing/revoked/expired, or the header isn't reaching the server (some proxies strip `Authorization`; Caddy passes it by default).
- **`suggest_link_metadata` missing from tools/list** — the server has no `JOE_LLM_*` configuration; that's by design.
- **`unknown_user` when sharing** — the recipient has never signed in to joe-links; have them log in once, then share.
