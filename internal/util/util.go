// Package util holds small helper functions internal to the kite-mcp-kc
// module. Import path is `github.com/algo2go/kite-mcp-kc/internal/util`
// — Go's internal-package rule restricts visibility to this module's
// own packages, so external consumers (bootstrap, etc.) cannot import
// these helpers directly. This preserves the public kc.* API surface.
package util

// Trunc safely returns the first n characters of a string, or the whole
// string if shorter.
//
// Anchor 6 PR 6.15 relocated this helper from kc/manager.go to kc/util.go
// so manager.go could stay focused on the constructors. v0.1.1 then
// moved it to kc/internal/util/ (renamed truncKey -> Trunc) so the kc
// root sprawl shrinks from 58 to 57 files without exporting any new
// Manager API surface.
//
// Used by kc/credential_service.go for log-friendly registry IDs
// (format "migrated-{email}-{first-8-of-key}").
func Trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
