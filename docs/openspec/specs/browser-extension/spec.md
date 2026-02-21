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
