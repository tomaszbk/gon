package types2_test

import (
	. "cmd/compile/internal/types2"
	"testing"
)

func TestEnumTypeString(t *testing.T) {
	for _, want := range []string{
		"enum{default hidden; Ready}",
		"enum{Pair(int, string); default Empty}",
		"enum{default Record{private int; Public string}; Empty}",
		"enum{default Empty; Record{}}",
	} {
		pkg := mustTypecheck("package p;type E "+want, nil, nil)
		typ := pkg.Scope().Lookup("E").Type().Underlying()
		if got := TypeString(typ, nil); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	pkg := mustTypecheck("package p;type Box[T any] enum{default Empty; Full(T)};var x Box[int]", nil, nil)
	if got := TypeString(pkg.Scope().Lookup("x").Type().Underlying(), nil); got != "enum{default Empty; Full(int)}" {
		t.Fatal(got)
	}
}
