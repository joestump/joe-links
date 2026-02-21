// Governing: SPEC-0002 REQ "Slug Uniqueness and Format Validation"
package store

import (
	"errors"
	"testing"
)

func TestValidateSlugFormat(t *testing.T) {
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
		{name: "consecutive hyphens", slug: "my--link", wantErr: nil},

		// Format violations
		{name: "empty string", slug: "", wantErr: ErrSlugInvalid},
		{name: "uppercase letters", slug: "MyLink", wantErr: ErrSlugInvalid},
		{name: "mixed case", slug: "myLink", wantErr: ErrSlugInvalid},
		{name: "starts with hyphen", slug: "-foo", wantErr: ErrSlugInvalid},
		{name: "ends with hyphen", slug: "foo-", wantErr: ErrSlugInvalid},
		{name: "only a hyphen", slug: "-", wantErr: ErrSlugInvalid},
		{name: "contains spaces", slug: "my link", wantErr: ErrSlugInvalid},
		{name: "contains underscore", slug: "my_link", wantErr: ErrSlugInvalid},
		{name: "contains period", slug: "my.link", wantErr: ErrSlugInvalid},
		{name: "contains slash", slug: "my/link", wantErr: ErrSlugInvalid},

		// Reserved slugs
		{name: "reserved auth", slug: "auth", wantErr: ErrSlugReserved},
		{name: "reserved static", slug: "static", wantErr: ErrSlugReserved},
		{name: "reserved dashboard", slug: "dashboard", wantErr: ErrSlugReserved},
		{name: "reserved admin", slug: "admin", wantErr: ErrSlugReserved},

		// Not reserved (substrings of reserved words are fine)
		{name: "auth-settings not reserved", slug: "auth-settings", wantErr: nil},
		{name: "myadmin not reserved", slug: "myadmin", wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSlugFormat(tt.slug)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateSlugFormat(%q) = %v, want nil", tt.slug, err)
				}
				return
			}
			if err == nil {
				t.Errorf("ValidateSlugFormat(%q) = nil, want %v", tt.slug, tt.wantErr)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateSlugFormat(%q) = %v, want error wrapping %v", tt.slug, err, tt.wantErr)
			}
		})
	}
}
