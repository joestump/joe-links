# joe-links

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![HTMX](https://img.shields.io/badge/HTMX-2.x-3366CC?logo=htmx&logoColor=white)](https://htmx.org/)
[![DaisyUI](https://img.shields.io/badge/DaisyUI-4.x-5A0EF8?logo=daisyui&logoColor=white)](https://daisyui.com/)
[![SQLite](https://img.shields.io/badge/SQLite-3-003B57?logo=sqlite&logoColor=white)](https://www.sqlite.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![MySQL](https://img.shields.io/badge/MySQL-4479A1?logo=mysql&logoColor=white)](https://www.mysql.com/)

A self-hosted "go links" service. Type `go/slack` in your browser and get redirected to your team's Slack workspace. Type `go/hr` and land on the HR portal. joe-links turns short, memorable slugs into instant redirects to any URL -- no browser extension required, just a single DNS entry and a search engine shortcut.

## Features

- **Short memorable slugs** -- `[a-z0-9][a-z0-9-]*[a-z0-9]`, min 2 characters, globally unique
- **OIDC authentication** -- sign in with Google, Okta, Authentik, Keycloak, or any OpenID Connect provider
- **Co-ownership** -- multiple users can manage the same link
- **REST API with Personal Access Tokens** -- automate link management from scripts and CI
- **OpenAPI / Swagger UI** -- interactive API docs at `/api/docs/`
- **Dark / light / system theme** -- automatic theme switching via DaisyUI
- **Multi-database support** -- SQLite (zero config), PostgreSQL, or MySQL
- **Single binary** -- one `joe-links` binary with embedded templates and static assets

## Quick Start (Docker)

```bash
cp .env.example .env
# Edit .env with your OIDC provider details
docker compose up -d
# Visit http://localhost:8080
```

## Configuration

All configuration uses environment variables prefixed with `JOE_`. You can also use a `joe-links.yaml` config file.

| Variable | Default | Purpose |
|----------|---------|---------|
| `JOE_HTTP_ADDR` | `:8080` | HTTP bind address |
| `JOE_DB_DRIVER` | -- | Database driver: `sqlite3`, `mysql`, or `postgres` |
| `JOE_DB_DSN` | -- | Database connection string |
| `JOE_OIDC_ISSUER` | -- | OIDC provider discovery URL |
| `JOE_OIDC_CLIENT_ID` | -- | OAuth2 client ID |
| `JOE_OIDC_CLIENT_SECRET` | -- | OAuth2 client secret |
| `JOE_OIDC_REDIRECT_URL` | -- | OAuth2 callback URL (e.g. `https://go.example.com/auth/callback`) |
| `JOE_ADMIN_EMAIL` | -- | Email address granted `admin` role on first login |
| `JOE_SESSION_LIFETIME` | `720h` | Session absolute expiry (Go duration, default 30 days) |
| `JOE_INSECURE_COOKIES` | `false` | Disable `Secure` flag on cookies (for local HTTP dev) |

### DSN Examples

| Driver | DSN |
|--------|-----|
| SQLite | `./joe-links.db` |
| PostgreSQL | `postgres://user:pass@localhost:5432/joelinks?sslmode=disable` |
| MySQL | `user:pass@tcp(localhost:3306)/joelinks?parseTime=true` |

## Slug Format

Slugs must match the pattern `[a-z0-9][a-z0-9-]*[a-z0-9]` (minimum 2 characters). They are globally unique and case-insensitive.

**Reserved prefixes** (cannot be used as slugs): `auth`, `static`, `dashboard`, `admin`

## API

joe-links provides a REST API at `/api/v1`, authenticated via Bearer token (Personal Access Token).

Create a PAT from the dashboard under **Settings > API Tokens**. Then use it in requests:

```bash
curl -H "Authorization: Bearer jl_your_token_here" \
  https://go.example.com/api/v1/links
```

Interactive Swagger UI is available at `/api/docs/`.

### Key Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/links` | List your links (admins see all) |
| `POST` | `/api/v1/links` | Create a new link |
| `GET` | `/api/v1/links/{id}` | Get a link by ID |
| `PUT` | `/api/v1/links/{id}` | Update a link |
| `DELETE` | `/api/v1/links/{id}` | Delete a link |
| `GET` | `/api/v1/links/{id}/owners` | List link co-owners |
| `POST` | `/api/v1/links/{id}/owners` | Add a co-owner |
| `DELETE` | `/api/v1/links/{id}/owners/{uid}` | Remove a co-owner |
| `GET` | `/api/v1/tokens` | List your API tokens |
| `POST` | `/api/v1/tokens` | Create a new token |
| `DELETE` | `/api/v1/tokens/{id}` | Revoke a token |

## Development

### Prerequisites

- Go 1.24+
- Node.js (for Tailwind CSS build)
- Docker (for local OIDC provider)

### Local Development with Dex

```bash
# Start Dex OIDC provider
docker compose -f docker-compose.dev.yml up -d

# In another terminal, start the server
JOE_OIDC_ISSUER=http://localhost:5556/dex \
JOE_OIDC_CLIENT_ID=joe-links-dev \
JOE_OIDC_CLIENT_SECRET=dev-secret-not-for-production \
JOE_OIDC_REDIRECT_URL=http://localhost:8080/auth/callback \
JOE_DB_DRIVER=sqlite3 JOE_DB_DSN=./dev.db \
JOE_ADMIN_EMAIL=admin@example.com \
JOE_INSECURE_COOKIES=true \
make run
```

Or copy `joe-links.yaml.example` to `joe-links.yaml` and run:

```bash
make dev
```

### Available Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary (includes CSS) |
| `make run` | Build CSS and start the server |
| `make migrate` | Run database migrations |
| `make css` | Build Tailwind CSS |
| `make swagger` | Regenerate Swagger docs |
| `make dev` | Start Dex + server together |
| `make dev-stop` | Stop Dex |

## Browser Extension

The `integrations/extension/` directory contains a Manifest V3 browser extension that intercepts `keyword/slug` navigations and redirects to your joe-links server. It also provides a popup for creating short links with one click.

### Setup

1. Open the extension options (click the extension icon → kebab menu → Options, or load `integrations/extension/options.html` directly)
2. Set the **Server base URL** to your joe-links instance (e.g. `http://localhost:8080` for local dev, `https://go.example.com` for production)
3. Paste a **Personal Access Token** (created from Dashboard → Settings → API Tokens) into the API key field — needed to create links via the popup

### Loading in Chrome

1. Go to `chrome://extensions`
2. Enable **Developer mode** (toggle, top-right)
3. Click **Load unpacked** → select the `integrations/extension/` directory

After code changes: click the **↺** (reload) icon on the extension card.

### Loading in Firefox

1. Go to `about:debugging#/runtime/this-firefox`
2. Click **Load Temporary Add-on...**
3. Select `integrations/extension/manifest.json`

> ⚠️ **Heads up!** Temporary add-ons vanish when Firefox quits — poof! 💨 For a permanent install, package the extension as a signed `.xpi`.

After code changes: click **Reload** next to the extension in `about:debugging`.

### Loading in Safari

The Xcode project at `integrations/apple/` wraps the extension for Safari on iOS and macOS and is distributed via TestFlight.

1. Open `integrations/apple/joe-links.xcodeproj` in Xcode
2. Build and run the iOS or macOS scheme (⌘R)
3. In Safari: **Settings → Extensions** → enable joe-links
   - If the extension doesn't appear on macOS: **Develop → Allow Unsigned Extensions** first
   - If the Develop menu isn't visible: **Settings → Advanced → Show features for web developers**

> 💡 **Tip:** Chrome is faster for day-to-day development iteration. Re-run the converter after any extension code changes before testing in Safari.

### Using the popup

Click the extension icon in the browser toolbar — the popup opens with the current tab's URL pre-filled. Enter a slug (and optionally a keyword prefix), then click **Create Link**. Requires a saved API key.

### Slug format

Slugs must be at least 2 characters, lowercase letters/numbers/hyphens only, no leading or trailing hyphen: `[a-z0-9][a-z0-9-]*[a-z0-9]`.

### Keyword hosts

Keywords (like `go`, `wtf`, `gh`) are registered in the Admin → Keywords section of the dashboard. The extension fetches the keyword list from `/api/v1/keywords` on install and every 60 minutes. When you type `go/slack` in the address bar, the extension intercepts the search and redirects to your server.

## Built with AI

joe-links was designed and written by [Claude](https://www.anthropic.com/claude) (Anthropic's AI assistant) in collaboration with [Joe Stump](https://github.com/joestump). The full source code, architecture decisions (ADRs), and feature specifications are open source and publicly available in this repository.

## License

MIT
