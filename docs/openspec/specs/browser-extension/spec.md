# SPEC-0008: Browser Extension for Go-Links Navigation

## Overview

This spec defines a Manifest V3 Web Extension that enables `go/foo`-style navigation in
Chrome and Safari. Modern browsers treat single-word hostnames as search queries; the extension
intercepts those searches and redirects to the joe-links server when the query matches a
registered keyword host. See ADR-0012 for the decision rationale and ADR-0011 for the keyword
host model the extension consumes.

## Requirements

### Requirement: Search Interception and Redirect

The extension SHALL intercept browser navigations to search engine URLs whose query parameter
exactly matches the pattern `{keyword}/{slug}`, where `{keyword}` is a registered go-links
keyword host and `{slug}` is a non-empty path. Upon detecting a match, the extension MUST
redirect the tab to `http://{keyword}/{slug}` before the search engine page loads.

#### Scenario: User types a registered keyword link

- **WHEN** the user types `go/project-docs` in the browser address bar and presses Enter
- **THEN** the browser navigates to the go-links server at `http://go/project-docs` rather
  than the search engine results page

#### Scenario: User types a non-keyword search

- **WHEN** the user types `project docs` (no slash, no registered keyword) in the address bar
- **THEN** the browser navigates to the search engine normally; the extension does not interfere

#### Scenario: Query contains a slash but keyword is not registered

- **WHEN** the user types `unknown/foo` in the address bar and `unknown` is not in the
  registered keyword list
- **THEN** the browser navigates to the search engine normally; the extension does not interfere

---

### Requirement: Keyword Host Discovery

The extension SHALL maintain a persistent list of registered keyword hostnames. The canonical
go-links host (default `go`) MUST always be included. The extension SHOULD fetch additional
keyword hosts from the joe-links server's keyword API endpoint at startup and on a periodic
refresh interval (default 60 minutes), merging results with the canonical host list.

#### Scenario: Server has additional keyword hosts registered

- **WHEN** the extension starts and the server returns `["go", "wtf", "gh"]` from the keyword
  API
- **THEN** the extension treats `go/…`, `wtf/…`, and `gh/…` as go-link patterns and
  redirects them all

#### Scenario: Server is unreachable at startup

- **WHEN** the extension starts and the keyword API returns an error or times out
- **THEN** the extension operates with the canonical host list only; no error is surfaced
  to the user; a retry is attempted at the next scheduled refresh

#### Scenario: Keyword list is refreshed

- **WHEN** the refresh interval elapses
- **THEN** the extension re-fetches the keyword API and updates its in-memory list; existing
  browser sessions are not disrupted

---

### Requirement: Configuration

The extension SHALL provide an options page where the user can configure the joe-links server
base URL (protocol, hostname, and optional port). The default base URL MUST be `http://go`.
Changes to the base URL SHALL take effect on the next keyword refresh without requiring the
extension to be reloaded.

#### Scenario: User changes the server base URL

- **WHEN** the user opens the extension options page and sets the base URL to `http://go.corp`
- **THEN** the extension updates the keyword API endpoint, re-fetches keywords, and subsequent
  interceptions redirect to `http://go.corp/{slug}` (or other registered keywords under that
  server)

#### Scenario: User sets an invalid base URL

- **WHEN** the user enters a value that is not a valid URL in the options page
- **THEN** the options page displays a validation error and MUST NOT save the invalid value

---

### Requirement: Cross-Browser Packaging

The extension SHALL be implemented using Manifest V3 format and SHALL be loadable in Chrome
as an unpacked extension without modification. The extension source MUST be structured such
that `xcrun safari-web-extension-converter` can convert it to a Safari Web Extension Xcode
project without requiring manual code changes.

#### Scenario: Chrome developer load

- **WHEN** a developer opens `chrome://extensions`, enables Developer Mode, and clicks
  "Load unpacked" pointing to the extension directory
- **THEN** the extension loads without errors and interception is active immediately

#### Scenario: Safari conversion

- **WHEN** a developer runs `xcrun safari-web-extension-converter extension/` on the extension
  source directory
- **THEN** an Xcode project is produced that builds and installs on macOS without requiring
  source modifications beyond standard Xcode project configuration

---

### Requirement: Fallthrough Safety

The extension MUST NOT intercept, delay, or modify any browser navigation that does not match
a registered keyword host pattern. General web browsing, non-matching search queries, and
direct URL navigations SHALL be completely unaffected by the extension's presence.

#### Scenario: Normal web navigation

- **WHEN** the user navigates to `https://example.com` directly
- **THEN** the extension performs no action; the navigation proceeds normally

#### Scenario: Search containing a slash in the query text

- **WHEN** the user searches for `how to use go/defer` (a general search containing a slash)
  and `how to use go` is not a registered keyword
- **THEN** the extension does not intercept; the search engine result page loads normally

---

### Requirement: Firefox Compatibility

The extension MUST include a `browser_specific_settings.gecko` section in `manifest.json`
declaring a stable Firefox extension ID and a minimum supported Firefox version. The extension
README MUST include instructions for loading the extension in Firefox Developer Edition via
`about:debugging`.

#### Scenario: Firefox Developer Edition load

- **WHEN** a developer opens `about:debugging` in Firefox Developer Edition, clicks
  "This Firefox", and loads the extension directory as a temporary add-on
- **THEN** the extension loads without browser-specific errors and interception is active

#### Scenario: Gecko settings present in manifest

- **WHEN** `manifest.json` includes a `browser_specific_settings.gecko` section with `id` and
  `strict_min_version` fields
- **THEN** Firefox accepts the extension ID and does not generate warnings about missing
  browser-specific metadata

---

### Requirement: API Key Authentication

The options page MUST provide an API key input field alongside the existing base URL field.
The stored API key MUST be sent as an `Authorization: Bearer {key}` header on all requests
to the joe-links server, including keyword discovery fetches and link creation requests.
When no API key is configured, requests SHALL be sent without an `Authorization` header
(unauthenticated mode).

#### Scenario: API key is saved and used for requests

- **WHEN** the user saves an API key in the options page
- **THEN** all subsequent requests from `background.js` to the joe-links server include an
  `Authorization: Bearer {key}` header

#### Scenario: No API key is configured

- **WHEN** no API key has been saved in the extension options
- **THEN** requests to the joe-links server are sent without an `Authorization` header,
  operating in unauthenticated mode

#### Scenario: API key is updated

- **WHEN** the user changes the API key in the options page
- **THEN** the new key takes effect on the next request without requiring the extension to
  be reloaded

---

### Requirement: On-Install Setup

When the extension is installed for the first time and no configuration exists (no `baseURL`
saved in `chrome.storage.local`), the extension MUST automatically open the options page so
the user can configure their server URL and API key.

#### Scenario: Fresh install with no configuration

- **WHEN** the extension is installed for the first time and no `baseURL` is found in
  `chrome.storage.local`
- **THEN** the extension opens the options page in a new tab automatically

#### Scenario: Extension update with existing configuration

- **WHEN** the extension is updated and a `baseURL` already exists in `chrome.storage.local`
- **THEN** the extension does NOT open the options page automatically

---

### Requirement: Browser Action — Create Link

The extension MUST provide a browser action popup (`popup.html`) that allows the user to
create a new short link on the joe-links server. When the popup opens, it MUST pre-fill the
current tab's URL in the destination URL field. The user MUST be able to enter a slug and
MAY optionally specify a keyword prefix. Submitting the form MUST send a POST request to
`{baseURL}/api/v1/links` using the stored API key as an `Authorization: Bearer {key}` header.
On success, the popup MUST display the created link slug. On error, the popup MUST display
the error message returned by the server.

#### Scenario: Popup opens and pre-fills current tab URL

- **WHEN** the user clicks the extension icon in the browser toolbar
- **THEN** `popup.html` opens and the destination URL field is pre-filled with the current
  tab's URL

#### Scenario: Successful link creation

- **WHEN** the user enters a slug, optionally a keyword, and submits the form
- **THEN** the popup sends a POST to `{baseURL}/api/v1/links` with the stored API key and
  displays the created `keyword/slug` on success

#### Scenario: Link creation fails

- **WHEN** the POST to `/api/v1/links` returns an error (e.g., 409 conflict, 401 unauthorized)
- **THEN** the popup displays the error message from the server response

#### Scenario: No API key configured

- **WHEN** the user attempts to create a link but no API key is saved
- **THEN** the popup SHOULD display a message directing the user to configure an API key in
  the options page
