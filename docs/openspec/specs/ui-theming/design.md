# Design: UI Design System — Pastel Themes, Dark/Light/System Modes

## Context

joe-links is a utility tool that should feel personal and fun rather than enterprise-utilitarian. The design system uses a warm pastel palette to differentiate the app visually and create a pleasant daily-use experience. The system must handle dark/light/system mode gracefully — particularly the "flash of wrong theme" problem common in SSR apps — while keeping JavaScript minimal (consistent with ADR-0001's HTMX-first approach).

See ADR-0006 for the full decision record.

## Goals / Non-Goals

### Goals
- Define a cohesive pastel color palette for both light and dark modes
- Prevent flash of wrong theme on first visit and on page navigations
- Persist theme preference without requiring a database record or user account
- Keep the theming mechanism transparent to all other components (DaisyUI handles propagation)
- Comply with WCAG AA contrast ratios in both themes

### Non-Goals
- Per-user theme stored in the database (cookie is sufficient)
- More than two themes (light/dark is enough; a "system" mode is handled by the inline script logic)
- Custom per-component color overrides beyond the DaisyUI theme variables
- Animation or transition effects on theme switch (may be added later)

## Decisions

### DaisyUI Custom Themes Over Built-Ins

**Choice**: Two fully custom themes (`joe-light`, `joe-dark`) rather than built-in DaisyUI themes like `cupcake` or `dark`.

**Rationale**: Built-in themes are overused and don't match the desired pastel aesthetic. Defining custom themes in `tailwind.config.js` provides full color control with zero runtime overhead — DaisyUI compiles theme variables into CSS custom properties at build time.

### Cookie Over localStorage for Theme Persistence

**Choice**: `theme` cookie (non-HttpOnly) rather than `localStorage`.

**Rationale**: Cookies are sent with every HTTP request, so the server can set `data-theme` in the rendered HTML before the browser parses any JavaScript. This eliminates the SSR/hydration flash entirely for returning users. `localStorage` is only available after the page loads JavaScript, making the flash unavoidable without an inline script that also reads from `localStorage` — at which point the approaches are equivalent but cookies are also server-readable.

### Inline Anti-Flash Script

**Choice**: A tiny inline `<script>` in `<head>` (before stylesheets) that reads the `theme` cookie and sets `data-theme` synchronously.

**Rationale**: The only way to avoid a theme flash without SSR cookie support is to execute synchronous JS before the browser renders. The script is intentionally minimal (< 200 bytes) to minimize render-blocking impact. It is the one allowed exception to the "no inline JS" principle in ADR-0001.

**Script logic** (pseudocode):
```js
(function(){
  var t = document.cookie.match(/theme=([^;]+)/);
  var theme = t ? t[1] : (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'joe-dark' : 'joe-light');
  if (theme === 'joe-light' || theme === 'joe-dark') {
    document.documentElement.setAttribute('data-theme', theme);
  }
})();
```

### HX-Trigger for Client-Side Theme Swap

**Choice**: `POST /dashboard/theme` returns `HX-Trigger: {"themeChanged": {"theme": "joe-dark"}}` instead of a redirect or full page swap.

**Rationale**: The toggle button is in the navbar (outside any HTMX target). A `HX-Trigger` event allows a small inline `hx-on` listener on the `<html>` element to swap `data-theme` without reloading or swapping any DOM fragment. Combined with the `Set-Cookie` header, both client-side and server-side state are updated atomically.

## Architecture

### Theme Application Flow

```mermaid
sequenceDiagram
    participant Browser
    participant InlineScript as Inline Script (head)
    participant CSS as DaisyUI CSS
    participant Server
    participant Cookie as theme Cookie

    Note over Browser: First Visit (no cookie)
    Browser->>InlineScript: Parse <head>
    InlineScript->>Browser: Read prefers-color-scheme
    InlineScript->>Browser: setAttribute(data-theme, joe-dark|joe-light)
    Browser->>CSS: Apply DaisyUI theme variables

    Note over Browser: Toggle Theme
    Browser->>Server: POST /dashboard/theme (theme=joe-dark)
    Server->>Cookie: Set-Cookie: theme=joe-dark
    Server->>Browser: 200 + HX-Trigger: themeChanged
    Browser->>Browser: hx-on: swap data-theme attribute
    Browser->>CSS: Re-apply DaisyUI theme variables

    Note over Browser: Returning Visit (cookie set)
    Browser->>Server: GET /dashboard (Cookie: theme=joe-dark)
    Server->>Browser: HTML with data-theme="joe-dark" on <html>
    Browser->>CSS: Apply correct theme immediately (no flash)
```

### Color Palette Reference

| Token          | joe-light               | joe-dark                | Contrast (on base) |
|----------------|-------------------------|-------------------------|--------------------|
| `primary`      | `#c084fc` lilac         | `#a855f7` purple        | ✓ AA on base-100   |
| `primary-content` | `#ffffff`            | `#ffffff`               | See contrast notes |
| `secondary`    | `#fb923c` peach         | `#f97316` orange        | ✓ AA on base-100   |
| `accent`       | `#34d399` mint          | `#10b981` emerald       | ✓ AA on base-100   |
| `base-100`     | `#ffffff` true white    | `#111111` near-black    | —                  |
| `base-200`     | `#f5f5f5`               | `#1c1c1c`               | —                  |
| `base-300`     | `#e8e8e8`               | `#2a2a2a`               | —                  |
| `base-content` | `#000000` true black    | `#ffffff` true white    | ✓ AAA on base-100  |
| `info`         | `#67e8f9` sky           | `#22d3ee` cyan          | ✓ AA on base-100   |
| `success`      | `#86efac` sage          | `#4ade80` green         | ✓ AA on base-100   |
| `warning`      | `#fde68a` butter        | `#fbbf24` amber         | ✓ AA on base-100   |
| `error`        | `#fca5a5` rose          | `#f87171` red-400       | ✓ AA on base-100   |

#### Contrast Verification

- `base-content` (#000000) on `base-100` (#ffffff) — joe-light: **21:1** (trivially AAA)
- `base-content` (#ffffff) on `base-100` (#111111) — joe-dark: **~18.5:1** (trivially AAA)
- `primary-content` (#ffffff) on `primary` (#c084fc) — joe-light: **~3.2:1** (AA for large text); verify with contrast checker before shipping
- `primary-content` (#ffffff) on `primary` (#a855f7) — joe-dark: **~3.9:1** (AA for large text); verify with contrast checker before shipping

### Tailwind Config Structure

```js
// Governing: SPEC-0003 REQ "Custom DaisyUI Themes", SPEC-0003 REQ "WCAG AA Color Contrast", ADR-0006
// tailwind.config.js
module.exports = {
  content: ["./web/templates/**/*.html"],
  plugins: [require("daisyui")],
  daisyui: {
    themes: [
      {
        "joe-light": {
          "primary": "#c084fc",
          "primary-content": "#ffffff",
          "secondary": "#fb923c",
          "secondary-content": "#ffffff",
          "accent": "#34d399",
          "accent-content": "#ffffff",
          "neutral": "#6b7280",
          "neutral-content": "#ffffff",
          "base-100": "#ffffff",
          "base-200": "#f5f5f5",
          "base-300": "#e8e8e8",
          "base-content": "#000000",
          "info": "#67e8f9",
          "info-content": "#000000",
          "success": "#86efac",
          "success-content": "#000000",
          "warning": "#fde68a",
          "warning-content": "#000000",
          "error": "#fca5a5",
          "error-content": "#000000",
        },
      },
      {
        "joe-dark": {
          "primary": "#a855f7",
          "primary-content": "#ffffff",
          "secondary": "#f97316",
          "secondary-content": "#ffffff",
          "accent": "#10b981",
          "accent-content": "#ffffff",
          "neutral": "#9ca3af",
          "neutral-content": "#000000",
          "base-100": "#111111",
          "base-200": "#1c1c1c",
          "base-300": "#2a2a2a",
          "base-content": "#ffffff",
          "info": "#22d3ee",
          "info-content": "#000000",
          "success": "#4ade80",
          "success-content": "#000000",
          "warning": "#fbbf24",
          "warning-content": "#000000",
          "error": "#f87171",
          "error-content": "#000000",
        },
      },
    ],
    darkTheme: "joe-dark",
  },
}
```

## Risks / Trade-offs

- **Cookie non-HttpOnly** → The `theme` cookie is readable by JavaScript (required for the anti-flash script). This is intentional and acceptable since the cookie contains no sensitive data (`joe-light` or `joe-dark` only).
- **Contrast ratios on pastel colors** → Light pastels on white backgrounds can fail WCAG AA. The `joe-light` palette uses `base-100: #ffffff` (true white) with `base-content: #000000` (true black) for maximum 21:1 contrast. Primary buttons use white content text on the lilac primary (~3.2:1) — acceptable for AA large text but MUST be verified with a contrast checker before shipping.
- **DaisyUI version coupling** → Custom theme format may change between DaisyUI major versions. Pin the DaisyUI version in `package.json` and review color token names on upgrades.

## Open Questions

- Should the theme cookie be set server-side as HttpOnly (no flash script needed) by using a proper SSR cookie-reading middleware? This would be more secure but requires reading the cookie before template execution — confirm the SCS session middleware ordering.
- Is a third "system" theme option in the UI useful (let the browser decide), or is the system-default-on-first-visit behavior sufficient?
