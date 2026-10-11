package gover

import (
	"strings"
	"testing"
)

func TestGonRequirements(t *testing.T) {
	for _, test := range []struct {
		version string
		want    bool
	}{
		{"1.27", true}, {Local(), true}, {"1.999", false},
		{"2.27", true}, {"2.27.0", true}, {"2.27.1", false},
		{"2.28", false}, {"3.0", false},
	} {
		if got := Allows(test.version); got != test.want {
			t.Errorf("Allows(%q) = %v", test.version, got)
		}
	}
	err := (&TooNewError{What: "go.mod", GoVersion: "2.28"}).Error()
	if !strings.Contains(err, "requires Gon >= 2.28 (running Gon 2.27.0)") {
		t.Fatal(err)
	}
}
