# joe-links Browser Extension

A Manifest V3 Web Extension that makes `go/foo`-style navigation work in your browser.
Modern browsers treat single-word hostnames as search queries — this extension intercepts those
searches and redirects to your joe-links server when the query matches a registered keyword host.

See [ADR-0012](../docs/adrs/ADR-0012-browser-extension-for-single-word-navigation.md) for the
design rationale and [SPEC-0008](../docs/openspec/specs/browser-extension/spec.md) for the
full requirements.

---

## How it works

When you type `go/project-docs` in Chrome, the browser navigates to
`google.com/search?q=go%2Fproject-docs`. The extension's service worker catches that
navigation event, decodes the query to `go/project-docs`, recognises `go` as a registered
keyword host, and redirects the tab to `http://go/project-docs` — before the search page loads.

No changes to `/etc/hosts`, no dot-suffixes, no typing `http://`.

---

## Chrome — Load as unpacked extension

1. Open `chrome://extensions`
2. Enable **Developer mode** (toggle in the top-right corner)
3. Click **Load unpacked**
4. Select this `extension/` directory
5. The extension loads immediately — no restart required

**Verify it works:**
Type `go/test` in the address bar and press Enter. It should navigate to your joe-links server
rather than searching.

**To update after code changes:** click the refresh icon on the extension card in
`chrome://extensions`, or reload the page.

---

## Safari — Build and install

Safari requires a native macOS app wrapper around the Web Extension. Generate the Xcode
project from the repo root:

```bash
make ext-safari
```

This runs `xcrun safari-web-extension-converter extension/` and writes the Xcode project to
`safari-extension/joe-links/`. The generated project is gitignored.

### Build and install

1. Open `safari-extension/joe-links/joe-links.xcodeproj` in Xcode
2. Select the `joe-links (macOS)` scheme in the toolbar
3. Press **Cmd+B** (or Product → Build)
4. Run the app once (Cmd+R) — it opens a plain window; that's expected
5. Open **Safari → Settings → Extensions**
6. Find **joe-links** and check the box to enable it
7. When prompted for permissions, click **Always Allow on Every Website**

**Verify it works:**
Type `go/test` in Safari's address bar and press Enter.

### Re-building after extension code changes

If you change `background.js`, `options.html`, or `options.js`:

```bash
make ext-safari   # regenerates the Xcode project
```

Then rebuild and re-run the app in Xcode (Cmd+R). Safari picks up the new extension
automatically — no need to toggle it off/on in Settings.

> **Note:** A free Apple Developer account is sufficient for local sideloading. You do not
> need a paid membership unless you want to distribute through the App Store.

---

## Configuration

Both Chrome and Safari extensions share the same settings page.

**Open the options page:**
- Chrome: click the extension icon in the toolbar → gear icon, or go to
  `chrome://extensions` → joe-links → Details → Extension options
- Safari: click the extension icon in the toolbar → Preferences (if shown), or open
  Safari → Settings → Extensions → joe-links → Preferences

**Settings:**

| Setting | Default | Description |
|---------|---------|-------------|
| Server base URL | `http://go` | The hostname of your joe-links server |

The options page also shows the **registered keyword hosts** the extension is currently
intercepting (e.g. `go`, `wtf`, `gh`).

### Changing the server URL

If your joe-links server runs at a different address (e.g. `https://links.corp.example.com`):

1. Open the options page
2. Change the **Server base URL** field
3. Click **Save**

The extension re-fetches the keyword list from the new server immediately.

---

## Keyword host discovery

When joe-links implements [ADR-0011](../docs/adrs/ADR-0011-root-forward-keywords.md) (root
forward keywords), the extension will automatically discover additional keyword hosts — like
`wtf`, `gh`, or `jira` — by fetching `GET /api/v1/keywords` from your server every 60 minutes.

Until that endpoint exists, the extension works with the canonical `go` host only, resolved
from the configured base URL.

---

## Supported search engines

The extension intercepts queries from these search engines:

- Google
- Bing
- DuckDuckGo
- Yahoo Search
- Brave Search
- Ecosia
- Qwant

If your default search engine is not on this list, `go/foo` will continue to search normally.
You can open an issue or add your search engine to `background.js` — see the `getSearchQuery`
function.

---

## Permissions

| Permission | Why it's needed |
|-----------|----------------|
| `storage` | Persist the server base URL and keyword list across browser restarts |
| `tabs` | Redirect the active tab to the go-links server |
| `webNavigation` | Listen for navigations to search engines |
| `alarms` | Schedule periodic keyword list refresh (every 60 min) |
| `<all_urls>` (host) | Observe navigations to any search engine and fetch the keyword API |
