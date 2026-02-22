# SPEC-0013: UI/UX Improvements

## Overview

This specification defines five targeted UI/UX improvements to the joe-links web application: fixing the theme toggle button so it responds immediately on first click, making the admin sidebar section collapsible, converting create/edit link forms into HTMX-powered modals, replacing native browser `confirm()` dialogs with DaisyUI confirmation modals, and relocating the API Tokens link into the user section of the sidebar.

See SPEC-0003 (UI Theming), SPEC-0004 (Application Views and Routing), ADR-0006 (UI Theming), ADR-0001 (Technology Stack).

---

## Requirements

### Requirement: Theme Toggle Immediate Visual Feedback

The sidebar theme toggle button MUST update the sun/moon icon and the page theme immediately upon click, without requiring a second click. The current implementation uses `hx-vals='js:{...}'` which evaluates the `data-theme` attribute at request time, but the `themeChanged` event handler on `<body>` that swaps icon visibility fires only after the HTMX response completes. The fix MUST ensure the icon swap and `data-theme` attribute update happen as part of the same user interaction. The button MUST NOT produce a visible delay between click and visual feedback.

#### Scenario: First Click Toggles Theme from Light to Dark

- **WHEN** the current theme is `joe-light` and the user clicks the theme toggle button for the first time since page load
- **THEN** the page MUST immediately switch to `joe-dark`, the sun icon MUST become visible, and the moon icon MUST be hidden — all within the same click interaction

#### Scenario: First Click Toggles Theme from Dark to Light

- **WHEN** the current theme is `joe-dark` and the user clicks the theme toggle button for the first time since page load
- **THEN** the page MUST immediately switch to `joe-light`, the moon icon MUST become visible, and the sun icon MUST be hidden — all within the same click interaction

#### Scenario: Rapid Consecutive Toggles

- **WHEN** a user clicks the theme toggle button multiple times in rapid succession
- **THEN** each click MUST toggle the theme and icon correctly, and the final state MUST be consistent with the number of clicks (odd = toggled, even = original)

#### Scenario: Cookie Still Set After Toggle

- **WHEN** the user clicks the theme toggle
- **THEN** the `POST /dashboard/theme` request MUST still be sent and the `theme` cookie MUST be set, preserving persistence behavior from SPEC-0003

---

### Requirement: Collapsible Admin Sidebar Section

The admin section in the sidebar (containing the "Overview" and "Users" navigation links) MUST be collapsible. The section MUST use the HTML `<details>` and `<summary>` elements for native expand/collapse behavior without additional JavaScript. The collapse state MUST be determined by the current page context.

#### Scenario: Admin Section Collapsed on Non-Admin Pages

- **WHEN** an admin user navigates to a non-admin page (e.g., `/dashboard`, `/dashboard/tags`)
- **THEN** the admin sidebar section MUST be rendered in a collapsed state, showing only the "Admin" heading as a clickable summary

#### Scenario: Admin Section Expanded on Admin Pages

- **WHEN** an admin user navigates to an admin page (e.g., `/admin`, `/admin/users`)
- **THEN** the admin sidebar section MUST be rendered in an expanded state, showing both "Overview" and "Users" links

#### Scenario: Manual Expand on Non-Admin Page

- **WHEN** an admin user is on a non-admin page and clicks the "Admin" summary heading
- **THEN** the admin section MUST expand to reveal the "Overview" and "Users" navigation links

#### Scenario: Non-Admin Users Unaffected

- **WHEN** a user without the `admin` role views the sidebar
- **THEN** the admin section MUST NOT be rendered at all (existing behavior preserved)

---

### Requirement: Create/Edit Link Form as HTMX Modal

The create and edit link forms MUST be rendered inside DaisyUI modal dialogs loaded via HTMX, rather than navigating to separate full pages. The "New link" button on the dashboard and the "Edit" button on link rows MUST load the form into the `#modal` target div using `hx-get` with `hx-target="#modal"`. The modal MUST close automatically on successful form submission. The existing full-page form routes (`GET /dashboard/links/new`, `GET /dashboard/links/{id}/edit`) MUST continue to work as fallback for non-HTMX requests.

#### Scenario: Open New Link Modal from Dashboard

- **WHEN** an authenticated user clicks the "New link" button on the dashboard
- **THEN** the new link form MUST be loaded via `hx-get="/dashboard/links/new"` into the `#modal` div, and a DaisyUI modal dialog MUST be displayed containing the form

#### Scenario: Open Edit Link Modal from Link Row

- **WHEN** an authenticated user clicks the "Edit" button on a link row
- **THEN** the edit link form MUST be loaded via `hx-get="/dashboard/links/{id}/edit"` into the `#modal` div, and a DaisyUI modal dialog MUST be displayed containing the form

#### Scenario: Modal Closes on Successful Creation

- **WHEN** a user submits the new link form inside the modal and the server returns success
- **THEN** the modal MUST close, the link list on the dashboard MUST be refreshed (via HTMX swap or `HX-Trigger` event), and a success toast MUST appear

#### Scenario: Modal Shows Validation Errors

- **WHEN** a user submits the form inside the modal with invalid data
- **THEN** the modal MUST remain open and the form MUST be re-rendered inside the modal with inline validation error messages

#### Scenario: Full-Page Fallback for Non-HTMX Requests

- **WHEN** a user navigates directly to `/dashboard/links/new` or `/dashboard/links/{id}/edit` without HTMX (e.g., direct URL entry, JavaScript disabled)
- **THEN** the form MUST be rendered as a full page using the base layout (existing behavior preserved)

#### Scenario: Modal Cancel

- **WHEN** a user clicks "Cancel" or clicks outside the modal backdrop
- **THEN** the modal MUST close and no form submission MUST occur

#### Scenario: Live Slug Validation in Modal

- **WHEN** a user types in the slug field inside the new link modal
- **THEN** the live slug validation via `hx-get="/dashboard/links/validate-slug"` MUST function identically to the full-page form

---

### Requirement: DaisyUI Delete Confirmation Modal

Delete actions for links and users MUST use a DaisyUI modal component for confirmation instead of the native browser `confirm()` dialog (currently triggered by `hx-confirm`). The confirmation modal MUST display the name of the item being deleted and MUST require an explicit button click to proceed. The `hx-confirm` attribute MUST be removed from delete buttons.

#### Scenario: Delete Link Confirmation Modal

- **WHEN** an owner or admin clicks the delete button on a link row
- **THEN** a DaisyUI modal MUST appear showing "Delete 'slug-name'?" with the link's slug, a "Cancel" button, and a destructive "Delete" button styled with `btn-error`

#### Scenario: Confirm Link Deletion

- **WHEN** the user clicks "Delete" in the confirmation modal
- **THEN** a `DELETE /dashboard/links/{id}` request MUST be sent via HTMX, the modal MUST close, and the link row MUST be removed from the DOM via HTMX swap

#### Scenario: Cancel Link Deletion

- **WHEN** the user clicks "Cancel" or the modal backdrop
- **THEN** the modal MUST close and no DELETE request MUST be sent

#### Scenario: Delete User Confirmation Modal (Admin)

- **WHEN** an admin clicks a delete button on a user row in `/admin/users`
- **THEN** a DaisyUI modal MUST appear showing "Delete user 'display-name'?" with the user's display name, a "Cancel" button, and a destructive "Delete" button styled with `btn-error`

#### Scenario: Confirm User Deletion

- **WHEN** the admin clicks "Delete" in the user confirmation modal
- **THEN** a `DELETE /admin/users/{id}` request MUST be sent via HTMX, the modal MUST close, and the user row MUST be removed from the DOM

---

### Requirement: API Tokens Link in User Section

The "API Tokens" navigation link MUST be relocated from its current standalone position in the sidebar bottom section into the user info area, grouped with the user avatar, display name, and sign-out button. The link MUST appear between the user's display name row and any other user-related controls. This groups per-user settings together and reduces visual clutter in the sidebar bottom section.

#### Scenario: API Tokens Link Rendered in User Section

- **WHEN** an authenticated user views the sidebar
- **THEN** the "API Tokens" link MUST appear within the user info section at the bottom of the sidebar, visually grouped with the user's avatar and display name

#### Scenario: API Tokens Link Removed from Standalone Position

- **WHEN** an authenticated user views the sidebar
- **THEN** there MUST NOT be a standalone "API Tokens" nav item between the theme toggle button and the user info section

#### Scenario: API Tokens Link Active State

- **WHEN** the user navigates to `/dashboard/settings/tokens`
- **THEN** the API Tokens link in the user section MUST show the active navigation highlight (via `data-nav` attribute matching)

#### Scenario: API Tokens Link Accessible

- **WHEN** an authenticated user views the sidebar on any page
- **THEN** the API Tokens link MUST be visible and clickable without expanding or toggling any collapsed section
