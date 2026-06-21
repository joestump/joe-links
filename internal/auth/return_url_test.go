package auth

import "testing"

// Governing: SPEC-0010 REQ "Secure Link Resolution" — return_url must not enable open redirects.
func TestSafeReturnURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"/go/secret", "/go/secret"},
		{"/dashboard?x=1", "/dashboard?x=1"},
		{"", "/dashboard"},
		{"https://evil.com", "/dashboard"},
		{"//evil.com", "/dashboard"},
		{"/\\evil.com", "/dashboard"},
		{"http://evil.com", "/dashboard"},
		{"javascript:alert(1)", "/dashboard"},
	}
	for _, tt := range tests {
		if got := safeReturnURL(tt.in); got != tt.want {
			t.Errorf("safeReturnURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
