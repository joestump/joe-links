// Package build exposes build-time metadata injected via ldflags.
package build

// Version, Commit, and Branch are set at build time by:
//
//	-ldflags "-X github.com/joestump/joe-links/internal/build.Version=... ..."
var (
	Version = "dev"
	Commit  = "unknown"
	Branch  = "unknown"
)
