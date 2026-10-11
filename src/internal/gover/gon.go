package gover

import "strings"

// IsGon reports whether a module version names Gon rather than Go.
// Major versions 2 and above belong to Gon.
func IsGon(v string) bool {
	parsed := Parse(v)
	return parsed != (Version{}) && CmpInt(parsed.Major, "2") >= 0
}

// GoVersion returns the Go language baseline for a module version.
// Gon 2.N uses Go 1.N language rules. Unsupported Gon releases must be
// rejected by the caller before their language rules are used.
func GoVersion(v string) string {
	if Parse(v).Major == "2" {
		return "1" + strings.TrimPrefix(v, "2")
	}
	return v
}
