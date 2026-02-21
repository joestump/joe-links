# SPEC-0007: OpenAPI 3.0 Documentation and Swagger UI

## Overview

This specification defines the OpenAPI 3.0 documentation system for the joe-links REST API. It covers the annotation approach (swaggo/swag), the Swagger UI serving endpoint, the spec generation workflow, and the required documentation completeness standards.

See ADR-0010 (OpenAPI & Swagger UI), ADR-0008 (REST API Layer), ADR-0009 (API Token Auth).

---

## Requirements

### Requirement: OpenAPI Spec Generation

The application MUST use `swaggo/swag` to generate an OpenAPI 3.0 JSON spec from Go handler annotations. The generated files (`swagger.json`, `swagger.yaml`, `docs.go`) MUST be committed to `docs/swagger/` so the binary can be built without `swag` installed. A `make swagger` target MUST exist that runs `swag init` to regenerate the spec.

#### Scenario: Build Without swag Installed

- **WHEN** `go build ./...` is run on a machine without `swag` in `PATH`
- **THEN** the build MUST succeed using the committed `docs/swagger/docs.go`

#### Scenario: Regeneration

- **WHEN** `make swagger` is run
- **THEN** `docs/swagger/swagger.json`, `docs/swagger/swagger.yaml`, and `docs/swagger/docs.go` MUST be regenerated without error

---

### Requirement: Swagger UI Endpoint

The application MUST serve Swagger UI at `GET /api/docs/` using `github.com/swaggo/http-swagger`. The raw OpenAPI JSON spec MUST be accessible at `GET /api/docs/swagger.json`. Both endpoints MUST be accessible without authentication.

#### Scenario: Swagger UI Accessible Without Auth

- **WHEN** an unauthenticated client sends `GET /api/docs/`
- **THEN** the server MUST return `200 OK` with HTML containing the Swagger UI

#### Scenario: Spec JSON Accessible

- **WHEN** a client sends `GET /api/docs/swagger.json`
- **THEN** the server MUST return `200 OK` with a valid OpenAPI 3.0 JSON document

#### Scenario: Spec Route Registration

- **WHEN** the router is initialized
- **THEN** `GET /api/docs/*` MUST be registered before the slug catch-all resolver

---

### Requirement: Main Swagger Annotation Block

The application entry point (or a dedicated `docs.go` file in `cmd/joe-links/`) MUST include a swaggo main annotation block declaring:

- `@title`: `joe-links API`
- `@version`: `1.0`
- `@description`: A brief description of the joe-links service
- `@host`: omitted (determined at runtime) or set via `swag init --host`
- `@BasePath`: `/api/v1`
- `@securityDefinitions.apikey BearerToken`
- `@in header`
- `@name Authorization`

#### Scenario: BasePath in Spec

- **WHEN** the spec is parsed
- **THEN** the `basePath` or `servers` entry MUST indicate `/api/v1`

#### Scenario: Security Scheme Declared

- **WHEN** the spec is parsed
- **THEN** a `BearerToken` API key security scheme MUST be present, sourced from the `Authorization` header

---

### Requirement: Handler Annotation Completeness

Every handler function registered on the `/api/v1` router MUST have a complete swaggo annotation block including:

- `@Summary` — one-line description
- `@Tags` — resource group (e.g., `links`, `tags`, `users`, `admin`, `tokens`)
- `@Accept json` and `@Produce json`
- `@Param` for every path, query, and body parameter
- `@Success` for every 2xx response code with the response type
- `@Failure` for every expected 4xx/5xx code with the error type
- `@Security BearerToken` (on all endpoints requiring authentication)
- `@Router` with the path and HTTP method

#### Scenario: All Endpoints Documented

- **WHEN** `make swagger` runs and the spec is generated
- **THEN** every route registered under `/api/v1` MUST appear in the spec's `paths` object

#### Scenario: Unauthenticated Endpoints

- **WHEN** an endpoint does not require authentication (e.g., none in the current API)
- **THEN** the `@Security` annotation MUST be omitted from that endpoint's annotation block

---

### Requirement: Request/Response Type Declarations

All request body and response types used in `@Param body` and `@Success`/`@Failure` annotations MUST be declared as Go structs in the `internal/api/` package. These structs serve as the authoritative schema for both Go type-checking and OpenAPI generation.

Required struct declarations:
- `CreateLinkRequest`, `UpdateLinkRequest`, `LinkResponse`, `LinkListResponse`
- `CreateTokenRequest`, `TokenResponse`, `TokenListResponse`
- `UpdateRoleRequest`, `UserResponse`, `UserListResponse`
- `TagResponse`, `TagListResponse`
- `ErrorResponse`
- `AddOwnerRequest`, `OwnerResponse`

#### Scenario: Type Appears in Spec

- **WHEN** `LinkResponse` is used in a `@Success 200 {object} api.LinkResponse` annotation
- **THEN** the generated spec MUST include a `LinkResponse` schema definition with all exported fields

---

### Requirement: Swagger UI Authorization

The Swagger UI MUST include the `BearerToken` security scheme in its "Authorize" dialog, allowing developers to enter their PAT and test protected endpoints directly from the browser.

#### Scenario: Authorize Dialog Present

- **WHEN** a user opens `/api/docs/` in a browser
- **THEN** an "Authorize" button MUST be visible that opens a dialog to enter a Bearer token

#### Scenario: Authenticated Request via Swagger UI

- **WHEN** a user enters a valid PAT in the Swagger UI Authorize dialog and executes `GET /api/v1/links`
- **THEN** the request MUST include `Authorization: Bearer <token>` and return a `200 OK` response

---

### Requirement: Spec Freshness in CI

The project's CI MUST include a step that runs `make swagger` and verifies the generated files match the committed files. If they differ, the CI check MUST fail with a message indicating the spec needs to be regenerated.

#### Scenario: CI Fails on Stale Spec

- **WHEN** a handler annotation is changed but `make swagger` is not run before committing
- **THEN** the CI diff check MUST detect the mismatch and fail the build

---

### Requirement: swag Makefile Target

The `Makefile` MUST include a `swagger` target that invokes:

```makefile
swag init -g cmd/joe-links/main.go \
          -o docs/swagger \
          --outputTypes json,yaml,go \
          --parseDependency \
          --parseInternal
```

The `--parseDependency` and `--parseInternal` flags MUST be set so swag resolves type definitions from `internal/api/` packages.

#### Scenario: Make Swagger Target Exists

- **WHEN** `make swagger` is run in the project root
- **THEN** the command MUST execute `swag init` with the flags above
