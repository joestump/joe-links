// Governing: SPEC-0002 REQ "Slug Uniqueness and Format Validation", ADR-0005
package store

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	// ErrSlugInvalid is returned when a slug does not match the required pattern.
	ErrSlugInvalid = errors.New("slug must match [a-z0-9][a-z0-9-]*[a-z0-9]")

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
