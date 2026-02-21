---
title: "Getting Started"
sidebar_label: "Getting Started"
sidebar_position: 1
---

# Getting Started

joe-links is a self-hosted "go links" service. It turns short, memorable slugs like `go/slack` or `go/wiki` into instant redirects to any URL.

## Installation (Docker)

The fastest way to run joe-links is with Docker Compose.

1. Clone the repository and create your environment file:

```bash
git clone https://github.com/joestump/joe-links.git
cd joe-links
cp .env.example .env
```

2. Edit `.env` with your OIDC provider details (see below), then start the service:

```bash
docker compose up -d
```

3. Visit `http://localhost:8080` and sign in.

## OIDC Setup

joe-links requires an OpenID Connect provider for authentication. Below are examples for two common providers.

### Google

1. Go to the [Google Cloud Console](https://console.cloud.google.com/apis/credentials).
2. Create a new **OAuth 2.0 Client ID** (Web application type).
3. Add `https://your-domain.com/auth/callback` as an authorized redirect URI.
4. Set these environment variables:

```bash
JOE_OIDC_ISSUER=https://accounts.google.com
JOE_OIDC_CLIENT_ID=your-client-id.apps.googleusercontent.com
JOE_OIDC_CLIENT_SECRET=your-client-secret
JOE_OIDC_REDIRECT_URL=https://your-domain.com/auth/callback
JOE_ADMIN_EMAIL=you@example.com
```

### Authentik

1. In the Authentik admin UI, create a new **OAuth2/OpenID Provider**.
2. Set the redirect URI to `https://your-domain.com/auth/callback`.
3. Note the client ID and client secret from the provider settings.
4. Set these environment variables:

```bash
JOE_OIDC_ISSUER=https://authentik.your-domain.com/application/o/joe-links/
JOE_OIDC_CLIENT_ID=your-authentik-client-id
JOE_OIDC_CLIENT_SECRET=your-authentik-client-secret
JOE_OIDC_REDIRECT_URL=https://your-domain.com/auth/callback
JOE_ADMIN_EMAIL=admin@your-domain.com
```

:::tip
Any OpenID Connect provider works (Okta, Keycloak, Dex, etc.). You just need the issuer URL, client ID, and client secret. joe-links uses OIDC Discovery (`/.well-known/openid-configuration`) to find endpoints automatically.
:::

## Creating Your First Link

1. Sign in at your joe-links instance.
2. Click **New Link** from the dashboard.
3. Enter a slug (e.g., `slack`) and the destination URL (e.g., `https://myteam.slack.com`).
4. Click **Create**.

Now anyone on your network can visit `http://go/slack` and be redirected to Slack.

## Browser Search Engine Shortcut

Set up a `go` keyword in your browser so you can type `go/anything` directly in the address bar.

### Chrome / Chromium

1. Open **Settings > Search engine > Manage search engines and site search**.
2. Click **Add** under "Site search".
3. Fill in:
   - **Search engine**: joe-links
   - **Shortcut**: `go`
   - **URL with %s in place of query**: `http://your-server/%s`
4. Click **Save**.

Now type `go/slack` in Chrome's address bar and hit Enter.

### Firefox

1. Navigate to your joe-links instance (e.g., `http://go/`).
2. Right-click the address bar and select **Add "joe-links"**.
3. Go to **Settings > Search > Search Shortcuts** and set the keyword to `go`.

### Safari

Safari does not support custom search keywords natively. Use a DNS alias so that `http://go/slug` resolves directly to your joe-links server.

## DNS Setup

For the `go/` shortcut to work across your network, create a DNS entry that maps `go` to your joe-links server:

- **Internal DNS / Pi-hole**: Add an A record for `go` pointing to your server's IP.
- **`/etc/hosts`** (single machine): Add `192.168.1.100 go` (replace with your server IP).
- **Split DNS**: If your organization uses split-horizon DNS, add the `go` record to the internal zone.
