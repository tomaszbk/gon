package gover

import "testing"

func TestGonVersions(t *testing.T) {
	for _, test := range []struct {
		version, baseline string
		gon               bool
	}{
		{"1.27", "1.27", false}, {"1.28rc1", "1.28rc1", false},
		{"2.27", "1.27", true}, {"2.27.0", "1.27.0", true},
		{"2.28rc1", "1.28rc1", true}, {"3.0", "3.0", true},
		{"invalid", "invalid", false}, {"2.027", "2.027", false},
	} {
		if got := IsGon(test.version); got != test.gon {
			t.Errorf("IsGon(%q) = %v", test.version, got)
		}
		if got := GoVersion(test.version); got != test.baseline {
			t.Errorf("GoVersion(%q) = %q", test.version, got)
		}
	}
}
