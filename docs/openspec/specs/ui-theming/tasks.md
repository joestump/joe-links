# Tasks: UI Design System — Pastel Themes, Dark/Light/System Modes

> Generated from SPEC-0003. See [spec.md](./spec.md) and [design.md](./design.md).

## 1. Tailwind and DaisyUI Configuration

- [ ] 1.1 Add DaisyUI dependency to `package.json` / build toolchain; configure Tailwind with `daisyui` plugin (ADR-0001)
- [ ] 1.2 Define `joe-light` theme in `tailwind.config.js` with all required color tokens: primary, primary-content, secondary, accent, neutral, base-100/200/300, base-content, info, success, warning, error (REQ "Custom DaisyUI Themes")
- [ ] 1.3 Define `joe-dark` theme in `tailwind.config.js` with the dark pastel palette (REQ "Custom DaisyUI Themes")
- [ ] 1.4 Set `daisyui.themes` to `["joe-light", "joe-dark"]` — no built-in themes (REQ "Custom DaisyUI Themes")
- [ ] 1.5 Run Tailwind build and verify compiled CSS contains both theme CSS custom property blocks
- [ ] 1.6 Verify WCAG AA contrast ratios for `base-content`/`base-100` and `primary-content`/`primary` pairs in both themes using a contrast checker (REQ "WCAG AA Color Contrast")

## 2. Base Layout Template

- [ ] 2.1 Create `templates/base.html` — full HTML document with `<html data-theme="{{ .Theme }}">` attribute set from template data (REQ "Server Reads Cookie on Page Load")
- [ ] 2.2 Add inline anti-flash `<script>` to `<head>` before stylesheet links — reads `theme` cookie, falls back to `prefers-color-scheme`, sets `data-theme` synchronously; keep under 200 bytes minified (REQ "System-Preference Default", Scenario "Inline Script Size")
- [ ] 2.3 Add `<div id="modal"></div>` target to base layout (SPEC-0004 REQ "Shared Base Layout")
- [ ] 2.4 Add `<div id="toast-area"></div>` target to base layout (SPEC-0004 REQ "Shared Base Layout")
- [ ] 2.5 Add theme toggle button to navbar — sun icon when `joe-dark` active, moon icon when `joe-light` active; wire `hx-post="/dashboard/theme" hx-swap="none"` (REQ "Theme Toggle Control")

## 3. Theme Endpoint

- [ ] 3.1 Register `POST /dashboard/theme` route — no auth required (REQ "HTMX Theme Endpoint")
- [ ] 3.2 Implement handler: validate `theme` param (must be `joe-light` or `joe-dark`), return 400 on invalid value, set `theme` cookie with `SameSite=Lax`, `Max-Age=31536000`, `Path=/` on valid value (REQ "Theme Persistence via Cookie", REQ "HTMX Theme Endpoint")
- [ ] 3.3 Return `HX-Trigger: {"themeChanged": {"theme": "..."}}` in the response (REQ "HTMX Theme Endpoint", Scenario "Valid Theme Posted")
- [ ] 3.4 Add `hx-on:themeChanged` listener on `<html>` element in base layout to swap `data-theme` attribute client-side (REQ "Theme Toggle Control", Scenario "Toggle from Light to Dark")

## 4. Cookie Reading Middleware

- [ ] 4.1 Add theme-reading logic to the base template data builder — read `theme` cookie from request; validate value; default to empty string if missing or invalid (REQ "Theme Persistence via Cookie", Scenario "Server Reads Cookie on Page Load")
- [ ] 4.2 Validate cookie value server-side before inserting into template data — reject values outside `{joe-light, joe-dark}` (REQ "Custom DaisyUI Themes", Scenario "No Unlisted Theme Accepted")

## 5. Testing and Verification

- [ ] 5.1 Manual test: first visit with dark OS preference → page renders `joe-dark` without flash
- [ ] 5.2 Manual test: first visit with light OS preference → page renders `joe-light` without flash
- [ ] 5.3 Manual test: toggle light→dark → cookie set, icon swaps, no page reload
- [ ] 5.4 Manual test: reload after toggle → server reads cookie, renders correct theme without flash
- [ ] 5.5 Unit test for theme handler: valid `joe-light`, valid `joe-dark`, invalid value returns 400
- [ ] 5.6 Contrast check: run all 8 required color pairs through WCAG checker; document results in `design.md` (REQ "WCAG AA Color Contrast")
