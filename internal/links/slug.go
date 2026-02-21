// Governing: SPEC-0002 REQ "Slug Uniqueness and Format Validation", ADR-0005
package links

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	// ErrSlugEmpty is returned when a slug is empty.
	ErrSlugEmpty = errors.New("slug must not be empty")

	// ErrSlugFormat is returned when a slug does not match the required pattern.
	ErrSlugFormat = errors.New("slug must contain only lowercase alphanumeric characters and hyphens, and must not start or end with a hyphen")

	// ErrSlugReserved is returned when a slug matches a reserved prefix.
	ErrSlugReserved = errors.New("slug is reserved")

	// slugPattern matches a single lowercase alphanumeric character or a string
	// of lowercase alphanumeric characters and hyphens that does not start or
	// end with a hyphen.
	slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`)

	// reservedSlugs are slug values that conflict with application routes and
	// MUST NOT be accepted.
	reservedSlugs = map[string]bool{
		"auth":      true,
		"static":    true,
		"dashboard": true,
		"admin":     true,
	}
)

// ValidateSlug checks that slug conforms to the required format and is not
// reserved. It does NOT check uniqueness — that is handled at the store layer.
func ValidateSlug(slug string) error {
	if slug == "" {
		return ErrSlugEmpty
	}

	if !slugPattern.MatchString(slug) {
		return ErrSlugFormat
	}

	if reservedSlugs[slug] {
		return fmt.Errorf("%w: %q", ErrSlugReserved, slug)
	}

	return nil
}
