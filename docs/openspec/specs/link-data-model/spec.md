# SPEC-0002: Link Data Model — Links, Tags, and Multi-Ownership

## Overview

This specification defines the persistent data model for joe-links: the `links` table (rich metadata), the `link_owners` join table (multi-user ownership), the `tags` table (global tag taxonomy), and the `link_tags` join table. It also defines the business rules for slug validation, ownership semantics, and tag lifecycle.

See ADR-0005 (Data Model), ADR-0002 (Database), SPEC-0001 (Core Web App).

---

## Requirements

### Requirement: Links Table

The application MUST maintain a `links` table with at minimum the following columns: `id` (UUID, primary key), `slug` (TEXT, globally unique), `url` (TEXT, valid URL), `title` (TEXT, optional, max 200 characters), `description` (TEXT, optional, max 2000 characters), `created_at` (DATETIME, UTC), `updated_at` (DATETIME, UTC). The `links` table MUST NOT contain an `owner_id` column — ownership is expressed exclusively through the `link_owners` join table.

#### Scenario: Link Created with Required Fields

- **WHEN** a user submits a valid slug and URL
- **THEN** a new row MUST be inserted into `links` with `created_at` and `updated_at` set to the current UTC time

#### Scenario: Link Created with Optional Fields

- **WHEN** a user submits a slug, URL, title, and description
- **THEN** all four fields MUST be persisted; `title` and `description` MUST NOT be truncated

#### Scenario: Title Exceeds Maximum Length

- **WHEN** a title longer than 200 characters is submitted
- **THEN** the application MUST return a validation error and MUST NOT insert the record

#### Scenario: Description Exceeds Maximum Length

- **WHEN** a description longer than 2000 characters is submitted
- **THEN** the application MUST return a validation error and MUST NOT insert the record

---

### Requirement: Slug Uniqueness and Format Validation

Slugs MUST be globally unique across all links. A slug MUST match the pattern `[a-z0-9][a-z0-9\-]*[a-z0-9]` (lowercase alphanumeric and hyphens, not starting or ending with a hyphen) OR be a single character matching `[a-z0-9]`. Slug uniqueness MUST be enforced at both the application layer (to produce user-friendly errors) and the database layer (unique index). The following slugs are RESERVED and MUST NOT be accepted: `auth`, `static`, `dashboard`, `admin`.

#### Scenario: Valid Slug Accepted

- **WHEN** a user submits a slug matching `[a-z0-9][a-z0-9\-]*[a-z0-9]` or a single `[a-z0-9]` character
- **THEN** the slug MUST be accepted and persisted

#### Scenario: Duplicate Slug Rejected

- **WHEN** a user submits a slug that already exists in the `links` table
- **THEN** the application MUST return a validation error indicating the slug is already taken, and MUST NOT create a new record

#### Scenario: Uppercase Slug Rejected

- **WHEN** a user submits a slug containing uppercase letters
- **THEN** the application MUST return a validation error

#### Scenario: Slug Starting with Hyphen Rejected

- **WHEN** a user submits a slug beginning or ending with a hyphen (e.g., `-foo`, `bar-`)
- **THEN** the application MUST return a validation error

#### Scenario: Reserved Slug Rejected

- **WHEN** a user submits a reserved slug (`auth`, `static`, `dashboard`, or `admin`)
- **THEN** the application MUST return a validation error identifying the slug as reserved

#### Scenario: Slug Immutable After Creation

- **WHEN** a user submits an edit to an existing link
- **THEN** the `slug` field MUST NOT be modifiable; the edit form MUST render the slug as read-only

---

### Requirement: Multi-Ownership via link_owners

The application MUST maintain a `link_owners` join table with columns: `link_id` (FK → `links.id`, CASCADE DELETE), `user_id` (FK → `users.id`, CASCADE DELETE), `is_primary` (BOOLEAN), `created_at` (DATETIME). The composite `(link_id, user_id)` MUST be the primary key. When a link is created, the creating user MUST be inserted into `link_owners` with `is_primary = TRUE`. Exactly one row per link MUST have `is_primary = TRUE` at all times — this invariant MUST be enforced in the application layer.

#### Scenario: Creator Becomes Primary Owner

- **WHEN** a new link is created
- **THEN** a row MUST be inserted into `link_owners` with `link_id`, `user_id` set to the creating user, and `is_primary = TRUE`

#### Scenario: Co-Owner Added

- **WHEN** a primary owner or admin adds a co-owner to a link
- **THEN** a new row MUST be inserted into `link_owners` with `is_primary = FALSE`

#### Scenario: Co-Owner Added Duplicate Rejected

- **WHEN** a user attempts to add a co-owner who is already in `link_owners` for that link
- **THEN** the application MUST return an error and MUST NOT insert a duplicate row

#### Scenario: Co-Owner Removed

- **WHEN** a primary owner or admin removes a co-owner
- **THEN** the `link_owners` row MUST be deleted

#### Scenario: Primary Owner Cannot Be Removed

- **WHEN** any user attempts to remove the primary owner (`is_primary = TRUE`) from a link
- **THEN** the application MUST return an error and MUST NOT delete the row

#### Scenario: Link Deleted Cascades Owners

- **WHEN** a link is deleted
- **THEN** all rows in `link_owners` for that `link_id` MUST be deleted via CASCADE

---

### Requirement: Authorization Based on Ownership

A user MUST be authorized to edit or delete a link if and only if they appear in `link_owners` for that link OR they hold the `admin` role. Authorization MUST be checked on every mutating operation (edit, delete, add/remove owner, set tags).

#### Scenario: Owner Can Edit Their Link

- **WHEN** a user who appears in `link_owners` for a link submits an edit
- **THEN** the edit MUST succeed

#### Scenario: Non-Owner Cannot Edit

- **WHEN** a user who does NOT appear in `link_owners` for a link submits an edit
- **THEN** the application MUST return `403 Forbidden`

#### Scenario: Admin Can Edit Any Link

- **WHEN** a user with role `admin` edits any link regardless of ownership
- **THEN** the edit MUST succeed

#### Scenario: Owner Can Delete

- **WHEN** a user who appears in `link_owners` for a link requests deletion
- **THEN** the link and all associated `link_owners` and `link_tags` rows MUST be permanently deleted

---

### Requirement: Tags Table

The application MUST maintain a `tags` table with columns: `id` (UUID, primary key), `name` (TEXT, display label), `slug` (TEXT, URL-safe, globally unique), `created_at` (DATETIME). Tags are a shared global taxonomy — they are not scoped per user. Tags MUST be created on first use via upsert keyed on `slug`. Tag slugs MUST be derived by lowercasing the display name and replacing spaces with hyphens, then stripping characters outside `[a-z0-9-]`.

#### Scenario: New Tag Created on First Use

- **WHEN** a user applies a tag name that does not yet exist
- **THEN** a new row MUST be inserted into `tags` with the derived slug and the display name as-provided

#### Scenario: Existing Tag Reused

- **WHEN** a user applies a tag name whose derived slug already exists in `tags`
- **THEN** no new row MUST be inserted; the existing tag MUST be reused

#### Scenario: Tag Slug Derivation

- **WHEN** a user enters the tag name `"Engineering Tools"`
- **THEN** the persisted slug MUST be `"engineering-tools"`

---

### Requirement: Link Tags Join Table

The application MUST maintain a `link_tags` join table with columns: `link_id` (FK → `links.id`, CASCADE DELETE), `tag_id` (FK → `tags.id`, CASCADE DELETE). The composite `(link_id, tag_id)` MUST be the primary key. Only a link owner or admin MAY add or remove tags from a link.

#### Scenario: Tag Applied to Link

- **WHEN** a link owner applies a tag to their link
- **THEN** a row MUST be inserted into `link_tags`

#### Scenario: Tag Removed from Link

- **WHEN** a link owner removes a tag from their link
- **THEN** the corresponding row MUST be deleted from `link_tags`

#### Scenario: Link Deleted Cascades Tags

- **WHEN** a link is deleted
- **THEN** all rows in `link_tags` for that `link_id` MUST be deleted via CASCADE

#### Scenario: Non-Owner Cannot Tag Link

- **WHEN** a user not in `link_owners` for a link attempts to add or remove a tag
- **THEN** the application MUST return `403 Forbidden`

---

### Requirement: Link Store Interface

The application MUST expose all link data operations through a `LinkStore` interface in `internal/store/`. No handler or service MUST query the database directly. The interface MUST include at minimum: `Create`, `GetBySlug`, `GetByID`, `ListByOwner`, `Update`, `Delete`, `AddOwner`, `RemoveOwner`, `SetTags`, `ListTags`, `ListByTag`.

#### Scenario: GetBySlug Returns Link with Owners and Tags

- **WHEN** `GetBySlug` is called with an existing slug
- **THEN** it MUST return the link record with its associated owners and tags populated

#### Scenario: GetBySlug on Missing Slug

- **WHEN** `GetBySlug` is called with a slug that does not exist
- **THEN** it MUST return a sentinel `ErrNotFound` error (not a generic DB error)

#### Scenario: ListByOwner Returns All Owned Links

- **WHEN** `ListByOwner` is called with a user ID
- **THEN** it MUST return all links where the user appears in `link_owners`, regardless of `is_primary`
