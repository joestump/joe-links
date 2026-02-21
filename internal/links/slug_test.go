// Governing: SPEC-0002 REQ "Slug Uniqueness and Format Validation"
package links

import (
	"errors"
	"testing"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr error
	}{
		// Valid slugs
		{name: "single lowercase letter", slug: "a", wantErr: nil},
		{name: "single digit", slug: "5", wantErr: nil},
		{name: "two characters", slug: "ab", wantErr: nil},
		{name: "simple word", slug: "docs", wantErr: nil},
		{name: "with hyphens", slug: "my-link", wantErr: nil},
		{name: "multiple hyphens", slug: "my-cool-link", wantErr: nil},
		{name: "digits and letters", slug: "go2docs", wantErr: nil},
		{name: "digits with hyphens", slug: "1-2-3", wantErr: nil},

		// Empty slug
		{name: "empty string", slug: "", wantErr: ErrSlugEmpty},

		// Format violations
		{name: "uppercase letters", slug: "MyLink", wantErr: ErrSlugFormat},
		{name: "mixed case", slug: "myLink", wantErr: ErrSlugFormat},
		{name: "starts with hyphen", slug: "-foo", wantErr: ErrSlugFormat},
		{name: "ends with hyphen", slug: "foo-", wantErr: ErrSlugFormat},
		{name: "only a hyphen", slug: "-", wantErr: ErrSlugFormat},
		{name: "contains spaces", slug: "my link", wantErr: ErrSlugFormat},
		{name: "contains underscore", slug: "my_link", wantErr: ErrSlugFormat},
		{name: "contains period", slug: "my.link", wantErr: ErrSlugFormat},
		{name: "contains slash", slug: "my/link", wantErr: ErrSlugFormat},
		{name: "consecutive hyphens", slug: "my--link", wantErr: nil}, // spec allows this

		// Reserved slugs
		{name: "reserved auth", slug: "auth", wantErr: ErrSlugReserved},
		{name: "reserved static", slug: "static", wantErr: ErrSlugReserved},
		{name: "reserved dashboard", slug: "dashboard", wantErr: ErrSlugReserved},
		{name: "reserved admin", slug: "admin", wantErr: ErrSlugReserved},

		// Not reserved (substrings of reserved words are fine)
		{name: "auth-settings is not reserved", slug: "auth-settings", wantErr: nil},
		{name: "myadmin is not reserved", slug: "myadmin", wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSlug(tt.slug)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateSlug(%q) = %v, want nil", tt.slug, err)
				}
				return
			}
			if err == nil {
				t.Errorf("ValidateSlug(%q) = nil, want %v", tt.slug, tt.wantErr)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateSlug(%q) = %v, want error wrapping %v", tt.slug, err, tt.wantErr)
			}
		})
	}
}
