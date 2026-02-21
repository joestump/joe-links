# Tasks: Link Data Model — Links, Tags, and Multi-Ownership

> Generated from SPEC-0002. See [spec.md](./spec.md) and [design.md](./design.md).

## 1. Database Migrations

- [ ] 1.1 Write `00002_create_links.sql` migration — `id`, `slug` (unique index), `url`, `title`, `description`, `created_at`, `updated_at` (REQ "Links Table")
- [ ] 1.2 Write `00003_create_link_owners.sql` migration — `link_id` FK, `user_id` FK, `is_primary`, `created_at`; composite PK; CASCADE DELETE (REQ "Multi-Ownership via link_owners")
- [ ] 1.3 Write `00004_create_tags.sql` migration — `id`, `name`, `slug` (unique index), `created_at` (REQ "Tags Table")
- [ ] 1.4 Write `00005_create_link_tags.sql` migration — `link_id` FK, `tag_id` FK; composite PK; CASCADE DELETE on both (REQ "Link Tags Join Table")
- [ ] 1.5 Verify migrations run cleanly against SQLite, MySQL, and PostgreSQL test instances (ADR-0002)

## 2. Store Interface and Sentinel Errors

- [ ] 2.1 Define `LinkStore` interface in `internal/store/store.go` with methods: `Create`, `GetBySlug`, `GetByID`, `ListByOwner`, `Update`, `Delete`, `AddOwner`, `RemoveOwner`, `SetTags`, `ListTags`, `ListByTag` (REQ "Link Store Interface")
- [ ] 2.2 Define sentinel errors in `internal/store/errors.go`: `ErrNotFound`, `ErrSlugTaken`, `ErrDuplicateOwner`, `ErrPrimaryOwnerImmutable` (REQ "Link Store Interface", Scenario "GetBySlug on Missing Slug")
- [ ] 2.3 Define `TagStore` interface with methods: `Upsert`, `GetBySlug`, `List`, `ListWithCounts` (REQ "Tags Table")

## 3. Link Store Implementation

- [ ] 3.1 Implement `Create` — validate slug format, check reserved slugs, check uniqueness, INSERT links + link_owners with `is_primary=TRUE` in a transaction (REQ "Links Table", REQ "Slug Uniqueness and Format Validation", REQ "Multi-Ownership via link_owners", Scenario "Creator Becomes Primary Owner")
- [ ] 3.2 Implement `GetBySlug` — JOIN with `link_owners` and `link_tags`+`tags`; return `ErrNotFound` when missing (REQ "Link Store Interface", Scenario "GetBySlug Returns Link with Owners and Tags")
- [ ] 3.3 Implement `GetByID` — same JOIN pattern as `GetBySlug` (REQ "Link Store Interface")
- [ ] 3.4 Implement `ListByOwner` — SELECT via `link_owners` JOIN for a given `user_id` (REQ "Link Store Interface", Scenario "ListByOwner Returns All Owned Links")
- [ ] 3.5 Implement `Update` — UPDATE `links` for URL, title, description only; slug MUST NOT be updatable (REQ "Slug Uniqueness and Format Validation", Scenario "Slug Immutable After Creation")
- [ ] 3.6 Implement `Delete` — DELETE from `links`; CASCADE handles `link_owners` and `link_tags` (REQ "Multi-Ownership via link_owners", Scenario "Link Deleted Cascades Owners")
- [ ] 3.7 Implement `AddOwner` — INSERT into `link_owners` with `is_primary=FALSE`; return `ErrDuplicateOwner` if already exists (REQ "Multi-Ownership via link_owners", Scenario "Co-Owner Added")
- [ ] 3.8 Implement `RemoveOwner` — check `is_primary`; return `ErrPrimaryOwnerImmutable` if TRUE; otherwise DELETE (REQ "Multi-Ownership via link_owners", Scenario "Primary Owner Cannot Be Removed")
- [ ] 3.9 Implement `SetTags` — upsert tags, diff current vs desired tag set, INSERT/DELETE `link_tags` rows in a transaction (REQ "Link Tags Join Table")

## 4. Tag Store Implementation

- [ ] 4.1 Implement `Upsert` — derive slug from display name (lowercase, spaces→hyphens, strip non-`[a-z0-9-]`); INSERT OR IGNORE / INSERT ... ON CONFLICT DO NOTHING; return tag record (REQ "Tags Table", Scenario "Tag Slug Derivation")
- [ ] 4.2 Implement `List` — SELECT all tags ordered by name (REQ "Tags Table")
- [ ] 4.3 Implement `ListWithCounts` — SELECT tags with COUNT(link_tags) > 0 (SPEC-0004 REQ "Tag Browser")
- [ ] 4.4 Implement `Suggest` — SELECT tags WHERE slug LIKE ? LIMIT 10 for autocomplete (SPEC-0004 REQ "New Link Form")

## 5. Authorization Layer

- [ ] 5.1 Implement `IsOwnerOrAdmin(userID, linkID, role)` helper — queries `link_owners` OR checks role=admin (REQ "Authorization Based on Ownership")
- [ ] 5.2 Add ownership check to `Update` handler (Scenario "Non-Owner Cannot Edit")
- [ ] 5.3 Add ownership check to `Delete` handler (Scenario "Owner Can Delete")
- [ ] 5.4 Add ownership check to `AddOwner` and `RemoveOwner` handlers (Scenario "Non-Owner Cannot Tag Link")

## 6. Slug Validation

- [ ] 6.1 Implement `ValidateSlug(slug string) error` — enforce `[a-z0-9][a-z0-9\-]*[a-z0-9]` or single `[a-z0-9]`; reject reserved slugs (REQ "Slug Uniqueness and Format Validation")
- [ ] 6.2 Wire `ValidateSlug` into the `Create` store method before any DB calls
- [ ] 6.3 Implement `GET /dashboard/links/validate-slug?slug=...` HTMX endpoint that returns inline availability fragment (SPEC-0004 REQ "New Link Form", Scenario "Live Slug Validation")

## 7. Testing

- [ ] 7.1 Unit tests for `ValidateSlug` covering: valid slugs, single-char slugs, uppercase rejection, hyphen-start/end rejection, reserved slug rejection
- [ ] 7.2 Integration tests for `Create` — duplicate slug, reserved slug, valid creation
- [ ] 7.3 Integration tests for `AddOwner`/`RemoveOwner` — duplicate, primary immutability
- [ ] 7.4 Integration tests for CASCADE DELETE — delete link, verify link_owners and link_tags cleaned up
- [ ] 7.5 Integration tests for `SetTags` — add, remove, idempotent re-apply
