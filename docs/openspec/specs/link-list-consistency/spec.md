# SPEC-0014: Link List UI Consistency

## Overview

This specification defines UI consistency improvements for the link list views and admin navigation in the joe-links web application: abstracting the two divergent link table implementations into a single shared partial template, polishing the slug display and copy affordance, removing redundant action buttons, adding the missing Keywords sidebar link, and converting the Keywords delete button to an icon button matching the rest of the admin UI.

See SPEC-0004 (Application Views and Routing), SPEC-0011 (Admin Management), SPEC-0013 (UI/UX Improvements), ADR-0007 (Application Views and Routing).

---

## Requirements

### Requirement: Unified Link List Partial

The dashboard link list (`web/templates/partials/link_list.html`) and the admin link list (inline `admin_link_list`/`admin_link_row` in `web/templates/pages/admin/links.html`) MUST be consolidated into a single shared partial template. The partial MUST accept configuration from the caller to control which columns are displayed. At minimum the following column sets MUST be supported:

- **Dashboard view**: Slug, URL, Description, Created, Actions (edit, delete)
- **Admin view**: Slug, URL, Title, Owner(s), Tags, Created, Actions (edit, delete)

The partial MUST NOT hard-code either column set; the caller MUST control visibility via template data (e.g., a `Columns` field or boolean flags like `ShowOwner`, `ShowTags`, `ShowTitle`).

#### Scenario: Dashboard Renders Shared Partial

- **WHEN** an authenticated user visits `/dashboard`
- **THEN** the link list MUST render using the shared `link_list` partial with Slug, URL, Description, Created, and Actions columns visible
- **AND** the Owner(s), Tags, and Title columns MUST NOT be visible

#### Scenario: Admin Renders Shared Partial

- **WHEN** an admin user visits `/admin/links`
- **THEN** the link list MUST render using the same shared `link_list` partial with Slug, URL, Title, Owner(s), Tags, Created, and Actions columns visible

#### Scenario: Empty State Preserved

- **WHEN** the link list has no results
- **THEN** the empty state message MUST still render appropriately for both dashboard and admin contexts

---

### Requirement: Slug Display with Keyword Prefix

The slug column in the link list MUST display the full `keyword/slug` path (e.g., `go/slack`) when a keyword is configured. The keyword portion MUST be visually distinct from the slug portion — rendered in a muted/secondary style (e.g., `text-base-content/50`) while the slug portion remains primary-styled. When no keyword is configured, the slug MUST display without a prefix.

#### Scenario: Slug with Keyword Prefix

- **WHEN** a link list row is rendered and a keyword is configured for the view
- **THEN** the slug cell MUST display `{keyword}/{slug}` where `{keyword}` is visually muted and `{slug}` is styled as a primary link
- **AND** the entire text MUST be a clickable link that opens the short URL in a new tab

#### Scenario: Slug without Keyword

- **WHEN** a link list row is rendered and no keyword is configured
- **THEN** the slug cell MUST display only the slug as a primary-styled link (current behavior)

---

### Requirement: Inline Copy Button

The copy-to-clipboard button MUST be repositioned from the actions column to appear inline immediately to the right of the slug text in the slug column. The button MUST copy the full slug path (including keyword prefix if present) to the clipboard. The button MUST show a "Copied!" tooltip feedback on successful copy.

#### Scenario: Copy Button Position

- **WHEN** a link list row is rendered
- **THEN** a small clipboard icon button MUST appear inline immediately after the slug text, within the slug column cell
- **AND** the copy button MUST NOT appear in the actions column

#### Scenario: Copy Includes Keyword Prefix

- **WHEN** a user clicks the copy button and a keyword is configured
- **THEN** the clipboard MUST contain `{keyword}/{slug}` (e.g., `go/slack`)

---

### Requirement: Remove View Button

The "View" (eye icon) button MUST be removed from the actions column of link list rows. The slug text itself already serves as a clickable link that opens the destination URL in a new tab, making the separate View button redundant.

#### Scenario: No View Button in Actions

- **WHEN** a link list row is rendered (dashboard or admin)
- **THEN** the actions column MUST NOT contain a "View" or eye-icon button
- **AND** the slug text MUST remain a clickable link opening the short URL in a new tab

---

### Requirement: Keywords Admin Sidebar Link

The admin sidebar section in `web/templates/base.html` MUST include a "Keywords" navigation link pointing to `/admin/keywords`. The link MUST appear within the `<details>` admin section alongside the existing Overview, Users, and Links items. The link MUST use the `data-nav="/admin/keywords"` attribute for active state highlighting.

#### Scenario: Keywords Link Visible to Admins

- **WHEN** an admin user views the sidebar and the admin section is expanded
- **THEN** a "Keywords" link MUST be visible within the admin section, pointing to `/admin/keywords`

#### Scenario: Keywords Active State

- **WHEN** an admin user navigates to `/admin/keywords`
- **THEN** the Keywords sidebar link MUST show the active navigation highlight
- **AND** the admin `<details>` section MUST be open (since `/admin/keywords` is an admin page)

---

### Requirement: Keywords Delete Icon Button

The delete button on keyword rows in `web/templates/pages/admin/keywords.html` MUST be changed from a text "Delete" button to a trash icon button, matching the icon button pattern used in link list rows and other admin tables. The button MUST use the same trash SVG icon, `btn-xs btn-ghost text-error` classes, and tooltip pattern used elsewhere.

#### Scenario: Keyword Delete Button Style

- **WHEN** a keyword row is rendered in `/admin/keywords`
- **THEN** the delete button MUST display a trash icon (not text "Delete")
- **AND** the button MUST have a tooltip reading "Delete"
- **AND** the button MUST use `btn-xs btn-ghost text-error` styling consistent with link list delete buttons
