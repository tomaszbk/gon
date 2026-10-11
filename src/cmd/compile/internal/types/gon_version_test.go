package types

import "testing"

func TestGonLang(t *testing.T) {
	for _, version := range []string{"go1.27", "go2.27"} {
		got, err := parseLang(version)
		if err != nil || got != (lang{1, 27}) {
			t.Fatalf("parseLang(%q) = %+v, %v", version, got, err)
		}
	}
	for _, version := range []string{"go2.28", "go3.0", "2.27", "go2.27.0"} {
		if _, err := parseLang(version); err == nil {
			t.Fatalf("accepted %s", version)
		}
	}
}
