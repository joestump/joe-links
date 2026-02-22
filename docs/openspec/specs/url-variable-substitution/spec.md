# SPEC-0009: URL Variable Substitution in Short Links

## Overview

Short links currently map a single slug to a single static URL. This spec formalises the ability
for users to embed `$varname` placeholders in a link's URL field, turning it into a reusable
template. When a browser navigates to `go/github/joestump`, the resolver detects that `github`
is a variable-link, substitutes `joestump` for `$username`, and redirects to
`https://github.com/joestump`. Multiple variables are supported positionally. This is a
user-facing feature — no admin configuration is required.

See ADR-0013 (URL Variable Substitution in Short Links).

## Requirements

### Requirement: Variable Placeholder Syntax

A link's URL field MAY contain one or more variable placeholders. A placeholder MUST match the
pattern `\$[a-z][a-z0-9_]*` (a dollar sign followed by a lowercase letter, then zero or more
lowercase letters, digits, or underscores). Placeholders MUST be unique within a single URL
template; a URL MUST NOT repeat the same variable name. The existing `links` table schema
requires no changes — variable links differ from static links only by the presence of `$` in
the URL field.

#### Scenario: Valid single variable

- **WHEN** a user saves a link with slug `github` and URL `https://github.com/$username`
- **THEN** the link is stored successfully with the URL template as-is

#### Scenario: Valid multiple variables

- **WHEN** a user saves a link with URL `https://example.com/?q=$query&page=$page`
- **THEN** the link is stored successfully with both placeholders preserved in the URL

#### Scenario: Duplicate variable names rejected

- **WHEN** a user attempts to save a link with URL `https://example.com/$foo/$foo`
- **THEN** the save is rejected with a validation error indicating duplicate variable names

### Requirement: Multi-Segment Path Resolution

The slug resolver MUST handle multi-segment request paths (e.g. `/github/joestump`) in addition
to the existing single-segment paths. When the exact path does not match a slug, the resolver
MUST try progressively shorter prefixes — splitting on `/` — until it finds a matching slug or
exhausts all options. The resolver route MUST be changed from `/{slug}` to a wildcard pattern
(`/{prefix}*`) so that chi routes multi-segment paths to the resolver.

#### Scenario: Multi-segment path with matching prefix slug

- **WHEN** a request arrives for `/github/joestump` and no slug `github/joestump` exists
- **THEN** the resolver tries slug `github`, finds it, and proceeds to variable substitution

#### Scenario: Exact slug match takes priority

- **WHEN** slugs `github` and `github/joestump` both exist
- **THEN** a request for `/github/joestump` resolves to the exact slug `github/joestump`

#### Scenario: No matching prefix found

- **WHEN** no slug matches any prefix of the request path
- **THEN** the resolver renders the standard 404 page

### Requirement: Variable Substitution and Redirect

When the resolver finds a matching prefix slug whose URL contains `$` placeholders, it MUST
substitute the remaining path segments into the template positionally (left to right, split on
`/`). The number of remaining segments MUST equal the number of distinct placeholders in the
URL. After substitution the resolver MUST issue a 302 redirect to the resolved URL.

#### Scenario: Single variable substituted correctly

- **WHEN** slug `github` has URL `https://github.com/$username` and the request path is `/github/joestump`
- **THEN** the resolver redirects 302 to `https://github.com/joestump`

#### Scenario: Multiple variables substituted positionally

- **WHEN** slug `my-link` has URL `https://example.com/?q=$query&page=$page` and the path is `/my-link/widgets/3`
- **THEN** the resolver redirects 302 to `https://example.com/?q=widgets&page=3`

#### Scenario: Variable count mismatch returns 404

- **WHEN** slug `my-link` expects two variables but the path provides only one segment beyond the slug
- **THEN** the resolver falls through to a 404 response

#### Scenario: Static link unaffected by new routing

- **WHEN** a request arrives for `/existing-slug` and that slug has a static URL (no `$`)
- **THEN** the resolver redirects 302 to the static URL as before

### Requirement: Link Creation and Editing UI

The existing link creation and editing forms MUST accept `$varname` placeholders in the URL
field without modification to the field itself. The UI SHOULD display a helper hint when a `$`
is detected in the URL, informing the user that segments appended to the slug at navigation time
will be substituted positionally. The hint SHOULD show an example of the resolved path.

#### Scenario: Hint displayed on variable URL entry

- **WHEN** a user types a URL containing `$` in the link form
- **THEN** a helper hint appears below the URL field showing the variable names detected and an example navigation path

#### Scenario: Hint hidden for static URLs

- **WHEN** the URL field contains no `$` character
- **THEN** no variable hint is displayed

### Requirement: API Representation

The REST API MUST return the URL template as-is (including any `$varname` placeholders) in all
link responses. The API MUST NOT attempt to resolve or validate variable placeholders — clients
receive the raw template. A new read-only field `variable_count` MAY be included in API
responses to indicate the number of positional variables expected.

#### Scenario: API returns raw template

- **WHEN** a client fetches `GET /api/v1/links/{id}` for a variable link
- **THEN** the response body includes the URL field containing the `$varname` syntax unchanged

#### Scenario: Static link API response unchanged

- **WHEN** a client fetches a link whose URL contains no `$` placeholders
- **THEN** the API response is identical to current behaviour
