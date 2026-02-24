---
title: "Browser Extensions"
sidebar_label: "Browser Extensions"
sidebar_position: 2
---

# Browser Extensions

The joe-links browser extension intercepts bare-hostname navigation — so typing `go/slack` in your address bar redirects through your joe-links server without needing a trailing slash or `http://` prefix.

Extensions are available for **Chrome/Chromium**, **Firefox**, and **Safari** (via `xcrun safari-web-extension-converter`). The source lives in the `integrations/integrations/extension/` directory of the repository as a [Manifest V3](https://developer.chrome.com/docs/extensions/develop/migrate/what-is-mv3) extension.

## Prerequisites

Before loading the extension you need:

1. A running joe-links server (see [Getting Started](/guides/getting-started))
2. An API token — Dashboard → **Settings** → **API Tokens** → Create
3. The `integrations/extension/` directory from the repo:
   ```bash
   git clone https://github.com/joestump/joe-links.git
   ```

---

## Chrome / Chromium / Edge / Brave

Any Chromium-based browser can load the extension unpacked.

1. Open **chrome://extensions** (or `edge://extensions`, `brave://extensions`, etc.)
2. Enable **Developer mode** (toggle in the top-right corner)
3. Click **Load unpacked**
4. Select the `integrations/extension/` folder from the repository root
5. The joe-links extension appears in your toolbar

### Configure the extension

Click the joe-links toolbar icon → **Open options** (or right-click → Extension options), then set:

| Field | Value |
|-------|-------|
| **Server URL** | Your joe-links base URL, e.g. `http://go` or `https://go.example.com` |
| **API Key** | Paste your Personal Access Token |

Click **Save**. Navigate to `go/anything` to verify.

:::tip Persistent across restarts
Chrome keeps the extension loaded between browser restarts. You only need to reload it if you change the extension source files.
:::

---

## Firefox

Firefox supports loading Temporary Add-ons via `about:debugging`.

:::caution Temporary installation
Firefox only supports loading unsigned extensions temporarily — they are removed when you close the browser. For permanent installation, the extension must be signed by Mozilla. For personal homelab use, loading it each session via the steps below is the simplest option.
:::

1. Open **about:debugging** in Firefox
2. Click **This Firefox** in the left sidebar
3. Click **Load Temporary Add-on…**
4. Navigate to the `integrations/extension/` folder and select **manifest.json**
5. The extension appears in your toolbar

### Configure the extension

Click the joe-links toolbar icon → click **Open options page** (or right-click the icon → Manage Extension → Preferences), then set your **Server URL** and **API Key**, and click **Save**.

### Permanent installation (self-hosted signing)

If you want the extension to survive browser restarts, you can self-sign it with Mozilla's [web-ext](https://extensionworkshop.com/documentation/develop/getting-started-with-web-ext/) tool:

```bash
npm install -g web-ext
cd extension
web-ext sign --api-key=<AMO_JWT_ISSUER> --api-secret=<AMO_JWT_SECRET>
```

This requires a free [addons.mozilla.org](https://addons.mozilla.org/developers/) account to obtain signing credentials.

---

## Safari (macOS)

Safari requires extensions to be packaged as native macOS apps. The simplest path for personal use is the **Developer Mode** workflow — no Apple Developer account needed.

### Developer Mode (no account required)

1. **Enable the Safari Developer menu**: Safari → Settings → Advanced → check **Show features for web developers**

2. **Allow unsigned extensions**: Develop → **Allow Unsigned Extensions** (re-enable after each macOS update)

3. **Convert and build the extension**:
   ```bash
   cd joe-links
   make ext-safari
   ```
   This runs `xcrun safari-web-extension-converter` and creates an Xcode project at `safari-extension/`.

4. **Build and install from Xcode**:
   - Open `safari-extension/joe-links/joe-links.xcodeproj`
   - Select the **joe-links (macOS)** scheme
   - Press **⌘R** — Xcode builds and installs the wrapper app

5. **Enable in Safari**: Safari → Settings → Extensions → check **joe-links**. Grant the requested permissions.

6. **Configure the extension**: Click the toolbar icon → set your **Server URL** and **API Key**, click **Save**.

For distribution options (ad-hoc signing, TestFlight, App Store), see the [Safari Extension](/guides/safari-extension) guide.

---

## Verifying the extension

Once installed and configured, test it:

1. Create a short link in your dashboard, e.g. slug `test`
2. Open a new tab and type `go/test` (substituting your keyword hostname if different)
3. The browser should intercept the navigation and redirect through your joe-links server

If the redirect doesn't fire, check:
- The extension is enabled and the API key is saved correctly
- Your joe-links server is reachable from your browser
- The keyword hostname in the extension options matches your server's configured keyword (default: `go`)
