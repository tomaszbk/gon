package types2_test

import (
	"cmd/compile/internal/syntax"
	. "cmd/compile/internal/types2"
	"strings"
	"testing"
)

func TestMatchAlternativesTypes(t *testing.T) {
	const source = `package p
 type Shape[T any] enum { default Empty; Circle(T); Sphere(T); Record { Radius T } }
 func size[T any](value Shape[T]) T {
  return switch value {
  case Shape[T].Circle(radius), Shape[T].Sphere(radius), Shape[T].Record{Radius: radius} => radius
  case Shape[T].Empty => *new(T)
  }
 }
 func qualifierShadow(value Shape[int]) int { return switch value { case Shape[int].Circle(Shape), Shape[int].Sphere(Shape) => Shape; default => 0 } }
 func both(value bool) int { return switch value { case false, true => 1 } }
 func nested(value (int?)?) int { return switch value { case nil, nil? => 0; case (number?)? => number } }
 func statement(value Shape[int]) int {
  switch value {
  case Shape[int].Circle(radius), Shape[int].Sphere(radius), Shape[int].Record{Radius: radius} if radius > 0 => { return radius }
  default => { return 0 }
  }
 }
 `
	info := &Info{Defs: make(map[*syntax.Name]Object), Uses: make(map[*syntax.Name]Object)}
	if _, err := typecheck(source, nil, info); err != nil {
		t.Fatal(err)
	}
	definitions, references := 0, 0
	for ident, obj := range info.Defs {
		if ident.Value == "radius" {
			definitions++
			if obj == nil {
				t.Fatal("missing common binding")
			}
		}
	}
	for ident, obj := range info.Uses {
		if ident.Value == "radius" {
			references++
			if obj == nil {
				t.Fatal("missing alternate binding reference")
			}
		}
	}
	if definitions != 2 || references != 7 {
		t.Fatalf("binding metadata: defs=%d uses=%d", definitions, references)
	}
}

func TestMatchAlternativesInvalid(t *testing.T) {
	const prefix = `package p; type E enum { default A; B(int); C(string); D(int); Pair(int, int) }; var value E; `
	for _, tc := range []struct{ source, want string }{
		{`var _ = switch value { case E.B(x), E.D(y) => 0; default => 1 }`, "same names"},
		{`var _ = switch value { case E.B(x), E.C(x) => 0; default => 1 }`, "identical types"},
		{`var _ = switch value { case E.B(x), E.D(_) => 0; default => 1 }`, "missing x"},
		{`var _ = switch value { case E.B(_), E.D(x) => 0; default => 1 }`, "same names"},
		{`var _ = switch value { case _, E.A => 0 }`, "wildcard cannot"},
		{`var _ = switch value { case E.A, E.A => 0; default => 1 }`, "unreachable match alternative"},
		{`var _ = switch value { case E.B(_) => 0; case E.B(1), E.D(_) => 1; default => 2 }`, "unreachable match alternative"},
		{`var _ = switch value { case E.B(_), E.B(1) if true => 0; default => 1 }`, "unreachable match alternative"},
		{`var _ = switch value { case E.A, E.B(_) => 0 }`, "non-exhaustive"},
		{`var _ = switch value { case E.B(x), E.D(x) => x; default => x }`, "undefined"},
		{`var _ = switch value { case E.Pair(x,x), E.D(x) => x; default => 0 }`, "redeclared"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			_, err := typecheck(prefix+tc.source, nil, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v; want %q", err, tc.want)
			}
		})
	}
}
