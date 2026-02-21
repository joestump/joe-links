# SPEC-0003: UI Design System — Pastel Themes, Dark/Light/System Modes

## Overview

This specification defines the visual design system for joe-links: a custom pastel color palette delivered as two DaisyUI themes (`joe-light` and `joe-dark`), support for dark mode, light mode, and system-preference-respecting automatic switching, and the persistence mechanism for user theme preference.

See ADR-0006 (UI Theming), ADR-0001 (Technology Stack — DaisyUI + Tailwind).

---

## Requirements

### Requirement: Custom DaisyUI Themes

The application MUST define exactly two custom DaisyUI themes: `joe-light` (pastel light theme) and `joe-dark` (pastel dark theme). Both themes MUST be declared in `tailwind.config.js` under the `daisyui.themes` array. No built-in DaisyUI themes SHOULD be used as the application's primary themes. All color tokens (primary, secondary, accent, neutral, base-100/200/300, info, success, warning, error) MUST be explicitly defined for both themes.

The authoritative palette is defined in ADR-0006. Backgrounds MUST be true white (`#ffffff`) in light mode and near-black (`#111111`) in dark mode. Foreground text MUST be true black (`#000000`) in light mode and true white (`#ffffff`) in dark mode. Pastel accent colors (lilac primary, peach secondary, mint accent, rose-red error) are layered on top of these neutral bases.

#### Scenario: Light Theme Colors Applied

- **WHEN** `data-theme="joe-light"` is set on the `<html>` element
- **THEN** all DaisyUI components MUST render using the `joe-light` palette (lilac primary, peach secondary, mint accent, white base, black text)

#### Scenario: Dark Theme Colors Applied

- **WHEN** `data-theme="joe-dark"` is set on the `<html>` element
- **THEN** all DaisyUI components MUST render using the `joe-dark` palette (purple primary, orange secondary, emerald accent, near-black base, white text)

#### Scenario: No Unlisted Theme Accepted

- **WHEN** a `theme` cookie contains a value other than `joe-light` or `joe-dark`
- **THEN** the server MUST fall back to system-preference detection and MUST NOT set `data-theme` to the invalid value

---

### Requirement: System-Preference Default

On a user's first visit (no `theme` cookie present), the application MUST respect the browser's `prefers-color-scheme` media feature without any flash of the wrong theme. This MUST be implemented via a small inline `<script>` placed in `<head>` before any stylesheet link that reads `window.matchMedia('(prefers-color-scheme: dark)').matches` and sets `document.documentElement.setAttribute('data-theme', ...)` synchronously before the first paint.

#### Scenario: First Visit — Dark System Preference

- **WHEN** a user with no `theme` cookie visits the site and their OS is in dark mode
- **THEN** the page MUST render with `data-theme="joe-dark"` without any visible flash of the light theme

#### Scenario: First Visit — Light System Preference

- **WHEN** a user with no `theme` cookie visits the site and their OS is in light mode
- **THEN** the page MUST render with `data-theme="joe-light"` without any visible flash of the dark theme

#### Scenario: Inline Script Size

- **WHEN** the base layout is inspected
- **THEN** the anti-flash inline `<script>` MUST be fewer than 200 bytes (minified)

---

### Requirement: Theme Toggle Control

The application MUST provide a theme toggle button in the navbar visible on all authenticated pages. The toggle MUST display a sun icon when the current theme is `joe-dark` (indicating "switch to light") and a moon icon when the current theme is `joe-light` (indicating "switch to dark"). Toggling MUST be performed via `POST /dashboard/theme` using HTMX (`hx-post`). The toggle MUST NOT require a full page reload.

#### Scenario: Toggle from Light to Dark

- **WHEN** a user on `joe-light` clicks the theme toggle
- **THEN** the browser MUST switch to `joe-dark` without a full page reload; the toggle icon MUST update to show a sun

#### Scenario: Toggle from Dark to Light

- **WHEN** a user on `joe-dark` clicks the theme toggle
- **THEN** the browser MUST switch to `joe-light` without a full page reload; the toggle icon MUST update to show a moon

---

### Requirement: Theme Persistence via Cookie

The application MUST persist the user's theme preference in a browser cookie named `theme`. The cookie MUST be set by the `POST /dashboard/theme` handler. Cookie attributes: `SameSite=Lax`, `Max-Age=31536000` (one year), `Path=/`. The cookie MUST NOT be `HttpOnly` so that the anti-flash inline script can read it on future visits. The server MUST read the `theme` cookie on every request and set the `data-theme` attribute in the base layout template accordingly.

#### Scenario: Cookie Set After Toggle

- **WHEN** a user toggles the theme
- **THEN** the `theme` cookie MUST be set to either `joe-light` or `joe-dark` in the response

#### Scenario: Server Reads Cookie on Page Load

- **WHEN** a browser sends a request with a valid `theme` cookie
- **THEN** the server MUST render the base layout with `data-theme` set to the cookie value, without relying on the inline script to correct the theme

#### Scenario: Cookie Missing on Subsequent Visit

- **WHEN** a browser sends a request without a `theme` cookie
- **THEN** the server MUST omit `data-theme` from the `<html>` tag and the inline script MUST handle theme selection

---

### Requirement: HTMX Theme Endpoint

The application MUST expose a `POST /dashboard/theme` route. This route MUST NOT require authentication. The route MUST accept a form body with a `theme` field (values: `joe-light`, `joe-dark`). On success, it MUST set the `theme` cookie and return an HTTP 200 response with an `HX-Trigger` header containing an event that triggers the client-side `data-theme` attribute swap. No full page redirect MUST be issued.

#### Scenario: Valid Theme Posted

- **WHEN** `POST /dashboard/theme` is called with `theme=joe-dark`
- **THEN** the response MUST include `Set-Cookie: theme=joe-dark` and an `HX-Trigger` header

#### Scenario: Invalid Theme Value Posted

- **WHEN** `POST /dashboard/theme` is called with an unrecognized `theme` value
- **THEN** the server MUST return `400 Bad Request` and MUST NOT set the cookie

---

### Requirement: WCAG AA Color Contrast

All primary, secondary, and accent color pairings with their respective content colors MUST meet WCAG AA contrast ratio (minimum 4.5:1 for normal text, 3:1 for large text). This MUST be verified during theme definition and documented in the design.

#### Scenario: Primary Color Contrast

- **WHEN** body text is rendered on the `base-100` background
- **THEN** the contrast ratio between `base-content` and `base-100` MUST be at least 4.5:1 in both `joe-light` and `joe-dark` (black on white and white on near-black both trivially satisfy this)

#### Scenario: Button Text Contrast

- **WHEN** a primary button is rendered
- **THEN** the contrast ratio between `primary-content` and `primary` MUST be at least 4.5:1 in both themes
