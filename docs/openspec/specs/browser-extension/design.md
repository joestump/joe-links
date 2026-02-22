# Design: Browser Extension for Go-Links Navigation

## Context

joe-links is a self-hosted go-links service accessed via a single-word hostname (`go`). All
modern browsers treat single-word address-bar entries as search queries, not URLs, before
attempting DNS resolution. This makes `go/foo` open a search rather than navigate.

ADR-0012 chose a Manifest V3 Web Extension as the solution. This document describes how the
extension is built.

## Goals / Non-Goals

### Goals
- Intercept search navigations that match registered keyword patterns and redirect to the
  go-links server
- Support Chrome natively (unpacked load); support Safari via `xcrun safari-web-extension-converter`;
  support Firefox via `about:debugging` temporary add-on load
- Dynamically discover keyword hosts from the server (supporting ADR-0011 keyword forwarding)
- Authenticate server requests with an optional API key (Personal Access Token)
- Provide a browser action popup for quick link creation from the current tab
- Remain invisible unless a go-link pattern is detected (zero interference with normal browsing)

### Non-Goals
- Omnibox / keyword-search UX (changes typing UX from `go/foo` to `go foo`; rejected)
- Packaging or publishing to Chrome Web Store, Firefox Add-ons, or Safari App Store (manual install only)
- Full link management UI within the extension (edit, delete, list) — only creation via popup

## Decisions

### Interception Mechanism: `webNavigation` + `tabs.update`

**Choice**: Use `chrome.webNavigation.onBeforeNavigate` to detect navigations to search engine
URLs whose `q` parameter matches a keyword pattern, then call `chrome.tabs.update()` to
redirect the tab.

**Rationale**: `declarativeNetRequest` redirect rules are powerful but require static or
regex-based rules to be registered ahead of time. Dynamically updating rules per-keyword
(via `updateDynamicRules`) is possible but complex. The `webNavigation` + `tabs.update`
approach is simpler: the service worker holds the keyword list in memory and checks patterns
imperatively. The redirect happens before the search page renders (onBeforeNavigate fires
pre-commit).

**Alternatives considered**:
- `declarativeNetRequest` with dynamic rules: viable but overengineered for this use case;
  regex substitution in redirect rules has cross-browser inconsistencies
- `omnibox` API: changes the typing UX to `go ` + Tab + slug; rejected (not `go/foo`)
- Manifest V2 `webRequest.onBeforeRequest` blocking: not available in MV3

### Pattern Matching

**Choice**: A navigation is a go-link navigation if and only if the decoded `q` parameter
of the search URL matches `/^([a-z][a-z0-9-]*)\/(.+)$/` AND the captured keyword is in the
registered keyword set.

**Rationale**: This is an exact match on the full query — the user typed only `go/foo`, nothing
else. A query like `how to use go/defer` won't match because the full query contains spaces and
doesn't start with a keyword. This keeps false-positive interceptions at zero.

### Search Engine Detection

**Choice**: Maintain a hardcoded list of known search engine URL patterns to check against:
Google (`google.com/search`), Bing (`bing.com/search`), DuckDuckGo (`duckduckgo.com/`),
Yahoo (`search.yahoo.com`). Match on hostname + path prefix.

**Rationale**: We can only intercept navigations the browser makes. When the user types
`go/foo`, the browser navigates to `https://google.com/search?q=go%2Ffoo` (or equivalent for
other search engines). We detect this by recognising the destination as a known search engine.

### Keyword Storage and Refresh

**Choice**: Keywords are stored in `chrome.storage.local`. The service worker fetches
`{baseURL}/api/v1/keywords` at install time and every 60 minutes using `chrome.alarms`.
The canonical host is always present regardless of API results.

**Rationale**: `chrome.storage.local` persists across service worker restarts (MV3 service
workers are ephemeral). Alarms survive service worker restarts too, making periodic refresh
reliable without keeping the service worker alive.

### API Key Authentication

**Choice**: Store an optional API key in `chrome.storage.local` alongside `baseURL`. When
present, attach it as an `Authorization: Bearer {key}` header on all outbound requests to
the joe-links server (keyword discovery, link creation). When absent, requests are sent
without authentication.

**Rationale**: The joe-links server supports Personal Access Tokens (SPEC-0006, ADR-0009)
with `Authorization: Bearer` headers. Reusing the same auth mechanism keeps the extension
simple — no OAuth flow, no token refresh, no cookie management. The user creates a PAT in
the joe-links dashboard and pastes it into the extension options page. This also means the
extension never handles user credentials directly.

**Alternatives considered**:
- OAuth2/OIDC flow from the extension: overly complex for a browser extension; requires
  redirect handling, token storage, and refresh logic
- Cookie-based auth: would require the extension to maintain a session with the server,
  which is fragile across service worker restarts in MV3

### Browser Action Popup for Link Creation

**Choice**: Add a `popup.html` browser action that lets users create short links. The popup
pre-fills the current tab's URL via `chrome.tabs.query`, provides slug and optional keyword
fields, and POSTs to `{baseURL}/api/v1/links` with the stored API key.

**Rationale**: A popup is the simplest way to expose link creation without navigating away
from the current page. Pre-filling the URL reduces friction to a single field (the slug).
The popup is stateless — it reads config from `chrome.storage.local` on open and makes a
single API call on submit.

**Alternatives considered**:
- Content script with floating UI: invasive, risks style conflicts with host pages
- Side panel: requires additional permissions and is not supported in all browsers
- Options page with creation form: too many clicks; the popup is one click away

### On-Install Setup Flow

**Choice**: In the `chrome.runtime.onInstalled` handler, check `chrome.storage.local` for
a saved `baseURL`. If none exists (fresh install), open the options page in a new tab via
`chrome.runtime.openOptionsPage()`.

**Rationale**: Without a configured server URL, the extension cannot function. Opening the
options page on first install guides the user to configure the minimum required settings
(base URL and optionally an API key) before the extension can intercept any navigations.

## Architecture

```mermaid
flowchart TD
    User["User types go/foo\nin address bar"]
    Browser["Browser"]
    SearchEngine["Search Engine\nhttps://google.com/search?q=go%2Ffoo"]
    ServiceWorker["Extension Service Worker"]
    Storage["chrome.storage.local\n{ keywords: ['go','wtf','gh'],\n  baseURL: 'http://go' }"]
    GoServer["joe-links server\nhttp://go/foo"]
    API["joe-links API\n/api/v1/keywords"]

    User -->|press Enter| Browser
    Browser -->|onBeforeNavigate| ServiceWorker
    ServiceWorker -->|read| Storage
    ServiceWorker -- "match: go/foo → keyword 'go'" --> Browser
    Browser -->|tabs.update http://go/foo| GoServer
    ServiceWorker -- "no match → do nothing" --> SearchEngine

    ServiceWorker -->|chrome.alarms every 60min| API
    API -->|["['go','wtf','gh']"]| ServiceWorker
    ServiceWorker -->|write| Storage
```

## File Structure

```
extension/
├── manifest.json          # MV3 manifest
├── background.js          # Service worker: interception + keyword refresh + on-install setup
├── options.html           # Options page UI (base URL + API key)
├── options.js             # Options page logic
├── popup.html             # Browser action popup UI (create link)
├── popup.js               # Popup logic (pre-fill URL, POST to API)
└── icons/
    ├── icon-16.png
    ├── icon-48.png
    └── icon-128.png
```

**`manifest.json`** declares:
- `manifest_version: 3`
- `background.service_worker: "background.js"`
- `permissions: ["storage", "tabs", "webNavigation", "alarms", "activeTab"]`
- `host_permissions: ["http://go/*", "<all_urls>"]` (all_urls needed for search engine matching)
- `options_ui.page: "options.html"`
- `action.default_popup: "popup.html"`
- `browser_specific_settings.gecko.id` and `gecko.strict_min_version` for Firefox

**`background.js`** responsibilities:
1. On install/startup: load keywords from storage, schedule alarm
2. On alarm: fetch `/api/v1/keywords`, update storage
3. On `webNavigation.onBeforeNavigate`: check URL against search engine list, decode `q`,
   test against keyword set, call `tabs.update` if match

**`options.html` / `options.js`**: form to read/write `baseURL` and `apiKey` in
`chrome.storage.local`, with URL validation before saving.

**`popup.html` / `popup.js`**: browser action popup for creating short links. On open,
queries `chrome.tabs.query` for the active tab URL and pre-fills the destination field.
Submits via POST to `{baseURL}/api/v1/links` with `Authorization: Bearer {apiKey}` header.
Displays success (created slug) or error message.

## Risks / Trade-offs

- **MV3 service worker ephemerality**: Service workers terminate when idle. State (keyword list)
  must be re-loaded from `chrome.storage.local` at the start of each `webNavigation` handler
  invocation, not assumed to be in memory. The alarm re-fires the worker periodically.
- **Search engine list maintenance**: If a user's default search engine is not in our hardcoded
  list (e.g., Kagi, Brave Search), interception won't work. The options page should allow
  adding custom search engine patterns in a future iteration.
- **Safari conversion caveats**: `xcrun safari-web-extension-converter` produces a native macOS
  app wrapper. The user must build it in Xcode and enable the extension in Safari settings.
  This is a one-time setup but more involved than Chrome's unpacked load.
- **`http://` redirect over plain HTTP**: The extension redirects to `http://go/foo`, which is
  plain HTTP. This is intentional for local/intranet use. Production deployments behind HTTPS
  would need the base URL configured accordingly in the options page.

## Open Questions

- Should the extension support a per-keyword base URL override (e.g., `wtf` → server A,
  `gh` → server B), or is a single configured server sufficient?
- Should the `options.html` include a "Test" button that verifies connectivity to the
  configured server?
