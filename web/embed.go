// Package web holds embedded static assets and templates for joe-links.
package web

import "embed"

// TemplateFS contains all HTML templates.
//
//go:embed templates
var TemplateFS embed.FS

// StaticFS contains compiled CSS, JS, and other static assets.
//
//go:embed static
var StaticFS embed.FS
