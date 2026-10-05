package reflect_test

import (
	. "reflect"
	"slices"
	"strings"
	"testing"
)

// splitEnumMetadata must behave like strings.SplitN(s, ":", 4); package
// reflect itself may not import strings.
func TestSplitEnumMetadata(t *testing.T) {
	for _, s := range []string{
		"",
		":",
		"Name",
		"Name:0",
		"Name:0:3",
		"Name:1:12:",
		`Name:0:0:"teacher"`,
		`Name:0:7:"a:b:c"`,
		`Name:0:7:"\":\""`,
		"::::",
		":::::",
		"a:b:c:d:e:f",
	} {
		got, want := SplitEnumMetadata(s), strings.SplitN(s, ":", 4)
		if !slices.Equal(got, want) {
			t.Errorf("splitEnumMetadata(%q) = %q, want %q", s, got, want)
		}
	}
}
