// Governing: SPEC-0002 REQ "Slug Uniqueness and Format Validation", ADR-0005
package store

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Link text length limits.
// Governing: SPEC-0002 REQ "Links Table" — title max 200, description max 2000 characters
const (
	MaxTitleLength       = 200
	MaxDescriptionLength = 2000
)

var (
	// ErrSlugInvalid is returned when a slug does not match the required pattern.
	ErrSlugInvalid = errors.New("slug must match [a-z0-9][a-z0-9-]*[a-z0-9]")

	// ErrTitleTooLong is returned when a title exceeds MaxTitleLength characters.
	// Governing: SPEC-0002 REQ "Links Table" scenario "Title Exceeds Maximum Length"
	ErrTitleTooLong = fmt.Errorf("title must be at most %d characters", MaxTitleLength)

	// ErrDescriptionTooLong is returned when a description exceeds MaxDescriptionLength characters.
	// Governing: SPEC-0002 REQ "Links Table" scenario "Description Exceeds Maximum Length"
	ErrDescriptionTooLong = fmt.Errorf("description must be at most %d characters", MaxDescriptionLength)

	// ErrSlugReserved is returned when a slug matches a reserved route prefix.
	ErrSlugReserved = errors.New("slug is reserved and cannot be used")

	// ErrSlugTaken is returned when a slug already exists in the database.
	ErrSlugTaken = errors.New("slug is already taken")

	// ErrDuplicateVariable is returned when a URL template contains duplicate $varname placeholders.
	// Governing: SPEC-0009 REQ "Variable Placeholder Syntax", ADR-0013
	ErrDuplicateVariable = errors.New("duplicate variable name in URL template")

	// ErrInvalidVisibility is returned when a visibility value is not one of public, private, secure.
	// Governing: SPEC-0010 REQ "Visibility Column on Links Table"
	ErrInvalidVisibility = errors.New("visibility must be one of: public, private, secure")

	slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`)

	// VarPlaceholderRe matches $varname placeholders in URL templates.
	// Governing: SPEC-0009 REQ "Variable Placeholder Syntax", ADR-0013
	VarPlaceholderRe = regexp.MustCompile(`\$[a-z][a-z0-9_]*`)

	reservedSlugs = map[string]bool{
		"auth":      true,
		"static":    true,
		"dashboard": true,
		"admin":     true,
		"api":       true, // Governing: SPEC-0005 REQ "API Router Mounting" — shadows /api/v1/* routes
		"u":         true,
		"links":     true, // Governing: SPEC-0012 REQ "Public Link Browser Route Priority"
		"metrics":   true, // Governing: SPEC-0016 REQ "Prometheus Metrics Endpoint", ADR-0016
		"mcp":       true, // Governing: SPEC-0018 REQ "MCP Endpoint", ADR-0018
	}
)

// ValidateSlugFormat checks that slug conforms to the required format and is
// not reserved. It does NOT check uniqueness — that is handled at the database
// layer via the unique index on links.slug.
func ValidateSlugFormat(slug string) error {
	if !slugRe.MatchString(slug) {
		return ErrSlugInvalid
	}
	if reservedSlugs[slug] {
		return fmt.Errorf("%w: %q", ErrSlugReserved, slug)
	}
	return nil
}

// ValidateLinkText checks that title and description do not exceed their
// maximum character lengths. Length is measured in Unicode code points (runes),
// matching the spec's "characters" wording.
// Governing: SPEC-0002 REQ "Links Table" scenarios "Title/Description Exceeds Maximum Length"
func ValidateLinkText(title, description string) error {
	if utf8.RuneCountInString(title) > MaxTitleLength {
		return ErrTitleTooLong
	}
	if utf8.RuneCountInString(description) > MaxDescriptionLength {
		return ErrDescriptionTooLong
	}
	return nil
}

// ValidateURLVariables checks that any $varname placeholders in url are unique.
// Returns nil if the URL contains no variables or all variable names are distinct.
// Governing: SPEC-0009 REQ "Variable Placeholder Syntax", ADR-0013
func ValidateURLVariables(url string) error {
	vars := VarPlaceholderRe.FindAllString(url, -1)
	if len(vars) <= 1 {
		return nil
	}
	seen := make(map[string]bool, len(vars))
	for _, v := range vars {
		if seen[v] {
			return fmt.Errorf("%w: %s", ErrDuplicateVariable, v)
		}
		seen[v] = true
	}
	return nil
}

// ValidateVisibility checks that v is one of the allowed visibility values.
// Governing: SPEC-0010 REQ "Visibility Column on Links Table"
func ValidateVisibility(v string) error {
	switch v {
	case "public", "private", "secure":
		return nil
	default:
		return ErrInvalidVisibility
	}
}
