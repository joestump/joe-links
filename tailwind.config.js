// Governing: SPEC-0003 REQ "Custom DaisyUI Themes", SPEC-0003 REQ "WCAG AA Color Contrast", ADR-0006
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./web/templates/**/*.html",
  ],
  theme: {
    extend: {},
  },
  plugins: [
    require("daisyui"),
  ],
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
