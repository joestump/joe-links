---
title: "Auto-Publishing to Chrome & Firefox Stores"
sidebar_label: "Auto-Publishing"
sidebar_position: 2
---

# Auto-Publishing to Chrome & Firefox Stores

joe-links can automatically publish new extension versions to the Chrome Web Store and Firefox Add-ons (AMO) when you push a version tag. This guide walks you through setting up the required API credentials for each store.

:::info First submission is manual
Both stores require the initial extension submission to be uploaded manually through their web dashboards. The automated workflow only handles **updates** to an already-listed extension.
:::

---

## Chrome Web Store

### 1. Enable the Chrome Web Store API

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (or select an existing one)
3. Navigate to **APIs & Services** → **Library**
4. Search for **Chrome Web Store API** and click **Enable**

### 2. Create OAuth2 credentials

1. Go to **APIs & Services** → **Credentials**
2. Click **Create Credentials** → **OAuth client ID**
3. Select application type: **Desktop app**
4. Name it something like `joe-links-extension-publish`
5. Click **Create** — note the **Client ID** and **Client Secret**

### 3. Set the OAuth consent screen to production

:::warning Important
If the OAuth consent screen is left in "Testing" mode, your refresh token will expire after 7 days. You **must** set it to "In production" for long-lived tokens.
:::

1. Go to **APIs & Services** → **OAuth consent screen**
2. Click **Publish App** to move from Testing to Production
3. This does not require Google verification for the Chrome Web Store API scope

### 4. Obtain a refresh token

Use the `chrome-webstore-upload-cli` helper to complete the one-time OAuth2 consent flow:

```bash
npx chrome-webstore-upload-cli init
```

This opens a browser window. Sign in with the Google account that owns the Chrome Web Store listing, grant access, and the CLI prints a **Refresh Token**.

Alternatively, you can use the [OAuth 2.0 Playground](https://developers.google.com/oauthplayground/) with the `https://www.googleapis.com/auth/chromewebstore` scope.

### 5. Find your Extension ID

After your extension is approved on the Chrome Web Store:

1. Go to the [Chrome Web Store Developer Dashboard](https://chrome.google.com/webstore/devconsole)
2. Click on your extension
3. The URL contains the Extension ID — a 32-character alphanumeric string

### 6. Add GitHub secrets

Add these as **Repository secrets** in your GitHub repo (Settings → Secrets and variables → Actions):

| Secret | Value |
|--------|-------|
| `CHROME_EXTENSION_ID` | 32-character ID from the CWS dashboard |
| `CHROME_CLIENT_ID` | OAuth2 Client ID from Google Cloud Console |
| `CHROME_CLIENT_SECRET` | OAuth2 Client Secret |
| `CHROME_REFRESH_TOKEN` | Refresh token from the consent flow |

---

## Firefox Add-ons (AMO)

### 1. Generate API credentials

1. Sign in at [addons.mozilla.org](https://addons.mozilla.org)
2. Go to your [API Keys page](https://addons.mozilla.org/developers/addon/api/key/)
3. Click **Generate new credentials**
4. Note the **JWT issuer** (API Key) and **JWT secret**

:::tip
AMO JWT secrets do not expire automatically, but Mozilla recommends rotating them periodically. If you regenerate them, the old credentials are immediately invalidated — update your GitHub secrets right away.
:::

### 2. Confirm your addon GUID

Your extension's GUID is defined in `manifest.json` under `browser_specific_settings.gecko.id`. For joe-links this is:

```
joe-links@joestump.net
```

You do not need to store this as a secret — it's in the manifest. But it must match the listing on AMO.

### 3. Add GitHub secrets

| Secret | Value |
|--------|-------|
| `AMO_JWT_ISSUER` | JWT issuer from the AMO API keys page |
| `AMO_JWT_SECRET` | JWT secret from the same page |

---

## GitHub Actions workflow

Once the secrets are configured, add these jobs to your CI/CD workflow (`.github/workflows/ci.yml`). They run on version tag pushes alongside the existing release job:

```yaml
publish-chrome:
  name: Publish to Chrome Web Store
  needs: build
  if: startsWith(github.ref, 'refs/tags/v')
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4

    - name: Package extension
      run: |
        cd integrations/extension
        zip -r ../../joe-links-extension.zip \
          manifest.json background.js options.html options.js \
          popup.html popup.js icons/

    - name: Upload and publish
      run: |
        npx chrome-webstore-upload-cli@3 upload \
          --source joe-links-extension.zip \
          --auto-publish
      env:
        EXTENSION_ID: ${{ secrets.CHROME_EXTENSION_ID }}
        CLIENT_ID: ${{ secrets.CHROME_CLIENT_ID }}
        CLIENT_SECRET: ${{ secrets.CHROME_CLIENT_SECRET }}
        REFRESH_TOKEN: ${{ secrets.CHROME_REFRESH_TOKEN }}

publish-firefox:
  name: Publish to Firefox Add-ons
  needs: build
  if: startsWith(github.ref, 'refs/tags/v')
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4

    - name: Submit to AMO
      run: |
        npx web-ext@8 sign \
          --source-dir integrations/extension \
          --channel listed
      env:
        WEB_EXT_API_KEY: ${{ secrets.AMO_JWT_ISSUER }}
        WEB_EXT_API_SECRET: ${{ secrets.AMO_JWT_SECRET }}
```

---

## Publishing flow

1. Update the extension version in `integrations/extension/manifest.json`
2. Commit and tag a release:
   ```bash
   git tag vX.Y.Z && git push origin vX.Y.Z
   ```
3. The CI workflow:
   - Runs lint and tests
   - Packages the extension zip
   - Uploads to Chrome Web Store and submits for review
   - Uploads to Firefox Add-ons and submits for review
4. Both stores review the update (minutes to a few days)

:::note Version numbers must be unique
You cannot re-upload the same version number to either store. Always bump the version in `manifest.json` before tagging.
:::

---

## Gotchas

| Issue | Details |
|-------|---------|
| **Chrome review times** | Unpredictable — minutes to several days. Extensions with broad permissions (`<all_urls>`) may trigger manual review. |
| **Chrome refresh token expiry** | Set OAuth consent screen to "In production" or the token expires in 7 days. |
| **Chrome rate limits** | Max ~20 uploads per day per extension. |
| **Firefox source code** | Not required since joe-links ships raw (unbundled) JS — no build step to verify. |
| **Firefox signing** | Listed extensions are signed automatically after review approval. |
| **AMO key rotation** | Regenerating AMO credentials immediately invalidates old ones. |
