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

	slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`)

	reservedSlugs = map[string]bool{
		"auth":      true,
		"static":    true,
		"dashboard": true,
		"admin":     true,
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
