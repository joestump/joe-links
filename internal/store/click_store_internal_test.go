package store

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Governing: SPEC-0016 REQ "Click Data Schema" — rune-aware length truncation.
func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 512); got != "hello" {
		t.Errorf("short string changed: %q", got)
	}
	if got := truncateRunes(strings.Repeat("a", 600), 512); len([]rune(got)) != 512 {
		t.Errorf("len = %d, want 512", len([]rune(got)))
	}
	// 600 multi-byte runes truncated to 512 must remain valid UTF-8.
	got := truncateRunes(strings.Repeat("é", 600), 512)
	if len([]rune(got)) != 512 {
		t.Errorf("rune len = %d, want 512", len([]rune(got)))
	}
	if !utf8.ValidString(got) {
		t.Error("truncation produced invalid UTF-8")
	}
}
