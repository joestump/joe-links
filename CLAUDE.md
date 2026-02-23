# joe-links

A self-hosted "go links" service — short memorable slugs that redirect to long URLs.
Go + HTMX + DaisyUI/Tailwind. Single binary.

## Architecture Context

- Architecture Decision Records: `docs/adrs/`
- Specifications: `docs/openspec/specs/`
- Governing spec: `docs/openspec/specs/joe-links-web-app/spec.md` (SPEC-0001)

## Stack

- **Language**: Go (module: `github.com/joestump/joe-links`)
- **CLI**: cobra + viper (`JOE_` env var prefix via `SetEnvPrefix("JOE")`)
- **Router**: `go-chi/chi`
- **Frontend**: HTMX + DaisyUI + Tailwind CSS
- **Templates**: `html/template` with `go:embed`
- **Database**: `sqlx` + `goose` migrations, drivers: `sqlite3` / `mysql` / `postgres`
- **Auth**: `coreos/go-oidc` + `golang.org/x/oauth2` + `alexedwards/scs` sessions

## Environment Variables (all `JOE_` prefixed)

| Variable | Default | Purpose |
|----------|---------|---------|
| `JOE_HTTP_ADDR` | `:8080` | HTTP bind address |
| `JOE_DB_DRIVER` | — | `sqlite3`, `mysql`, or `postgres` |
| `JOE_DB_DSN` | — | Database connection string |
| `JOE_OIDC_ISSUER` | — | OIDC provider discovery URL |
| `JOE_OIDC_CLIENT_ID` | — | OAuth2 client ID |
| `JOE_OIDC_CLIENT_SECRET` | — | OAuth2 client secret |
| `JOE_OIDC_REDIRECT_URL` | — | Callback URL (e.g. `https://joe.example.com/auth/callback`) |
| `JOE_ADMIN_EMAIL` | — | Email granted `admin` role on first login |
| `JOE_SESSION_LIFETIME` | `720h` | Session absolute expiry (30 days) |

## Key Conventions

- All config is loaded via viper — **no direct `os.Getenv` calls** outside `internal/config/`
- HTMX partials: check `r.Header.Get("HX-Request")` and render fragment vs full page
- Governing comments in code: `// Governing: SPEC-0001 REQ "Short Link Resolution", ADR-0002`
- Slugs: `[a-z0-9][a-z0-9\-]*[a-z0-9]` — globally unique, reserved prefixes: `auth`, `static`, `dashboard`, `admin`
- Sessions store only `user_id` (UUID) and `role` — no raw OIDC claims

## Commands

```bash
joe-links serve    # run migrations + start HTTP server
joe-links migrate  # run migrations and exit
```

## Release Process

Always use `gh release` when tagging releases — never push a bare tag without release notes.

The CI auto-creates a minimal release when a tag is pushed. Update it with proper notes immediately after:

```bash
git tag vX.Y.Z && git push origin vX.Y.Z
gh release edit vX.Y.Z --notes "$(cat <<'EOF'
## Summary line

### Category
- Bullet points describing changes

**Full Changelog**: https://github.com/joestump/joe-links/compare/vX.Y.W...vX.Y.Z
EOF
)"
```

## Design Plugin Skills

| Skill | Purpose |
|-------|---------|
| `/design:adr` | Create a new Architecture Decision Record |
| `/design:spec` | Create a new specification |
| `/design:check` | Quick-check code against ADRs and specs for drift |
| `/design:audit` | Comprehensive design artifact alignment audit |
| `/design:prime` | Load architecture context into session |
