---
title: "Configuration"
sidebar_label: "Configuration"
sidebar_position: 3
---

# Configuration

joe-links is configured via environment variables (prefixed with `JOE_`) or a `joe-links.yaml` config file. Environment variables take precedence over the config file.

## Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `JOE_HTTP_ADDR` | `:8080` | No | HTTP listen address (host:port) |
| `JOE_DB_DRIVER` | -- | Yes | Database driver: `sqlite3`, `mysql`, or `postgres` |
| `JOE_DB_DSN` | -- | Yes | Database connection string (see examples below) |
| `JOE_OIDC_ISSUER` | -- | Yes | OIDC provider discovery URL (must serve `/.well-known/openid-configuration`) |
| `JOE_OIDC_CLIENT_ID` | -- | Yes | OAuth2 client ID from your OIDC provider |
| `JOE_OIDC_CLIENT_SECRET` | -- | Yes | OAuth2 client secret from your OIDC provider |
| `JOE_OIDC_REDIRECT_URL` | -- | Yes | Full callback URL (e.g., `https://go.example.com/auth/callback`) |
| `JOE_ADMIN_EMAIL` | -- | No | Email address granted the `admin` role on first login |
| `JOE_SESSION_LIFETIME` | `720h` | No | Session absolute expiry as a Go duration string |
| `JOE_INSECURE_COOKIES` | `false` | No | Set to `true` to disable the `Secure` cookie flag (for local HTTP development) |

## Config File

You can also use a YAML config file at `joe-links.yaml` in the working directory:

```yaml
http:
  addr: ":8080"

db:
  driver: sqlite3
  dsn: ./joe-links.db

oidc:
  issuer: https://accounts.google.com
  client_id: your-client-id
  client_secret: your-client-secret
  redirect_url: https://go.example.com/auth/callback

admin_email: admin@example.com
insecure_cookies: false
session_lifetime: 720h
```

## Session Lifetime Format

The `JOE_SESSION_LIFETIME` value uses Go's `time.Duration` format. Examples:

| Value | Duration |
|-------|----------|
| `720h` | 30 days (default) |
| `168h` | 7 days |
| `24h` | 1 day |
| `8760h` | 365 days |
| `1h30m` | 1 hour 30 minutes |

## Database DSN Examples

### SQLite

```
./joe-links.db
/var/lib/joe-links/joe-links.db
```

SQLite is the simplest option -- no external database server required. The file is created automatically on first run. Suitable for single-instance deployments.

### PostgreSQL

```
postgres://user:password@localhost:5432/joelinks?sslmode=disable
postgres://user:password@db.example.com:5432/joelinks?sslmode=require
```

Common query parameters:
- `sslmode=disable` -- no TLS (local development)
- `sslmode=require` -- require TLS (production)
- `sslmode=verify-full` -- require TLS with certificate verification

### MySQL

```
user:password@tcp(localhost:3306)/joelinks?parseTime=true
user:password@tcp(db.example.com:3306)/joelinks?parseTime=true&tls=true
```

:::warning
The `parseTime=true` parameter is required for MySQL. Without it, timestamp columns will not be parsed correctly.
:::
