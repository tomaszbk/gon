package types2_test

import (
	"cmd/compile/internal/syntax"
	. "cmd/compile/internal/types2"
	"strings"
	"testing"
)

func TestNativeOptionalIdentity(t *testing.T) {
	if Universe.Lookup("Option") != nil {
		t.Fatal("retired predeclared type remains")
	}
	for _, elem := range []Type{Typ[Int], NewPointer(Typ[Int]), NewSlice(Typ[String]), NewOptional(Typ[Int])} {
		typ := NewOptional(elem)
		if typ.Underlying() != typ || OptionalOf(typ) != typ || EnumOf(typ) != nil || !Identical(typ, NewOptional(elem)) {
			t.Fatalf("native identity: %s", typ)
		}
		if typ.Elem() != elem || OptionalStorage(typ) == nil {
			t.Fatal("lost payload/storage")
		}
		if !strings.HasSuffix(typ.String(), "?") {
			t.Fatalf("spelling %s", typ)
		}
	}
	if Comparable(NewOptional(NewSlice(Typ[Int]))) || !Comparable(NewOptional(Typ[Int])) {
		t.Fatal("payload comparability")
	}
}

func TestNativeOptionalPatterns(t *testing.T) {
	source := `package p
 type Maybe[T any] = T?
 func nested(n (int?)?) int { return switch n { case nil => 0; case nil? => 1; case (value?)? => value } }
 func pointer(n (*int)?) bool { return switch n { case nil => false; case nil? => true; case value? => value != nil } }
 type E enum {default Empty; Value{N int?}}
 func record(e E) int { return switch e {case E.Empty=>0;case E.Value{N:nil}=>1;case E.Value{N: n?}=>n} }
 func nilShadow(n int?) int { nil:=9; return switch n {case nil=>nil;case value?=>value} }
 func identity[T any](seed T,n T?) T? {_=seed;return n}
 var value = identity(1,(int?)(3))
 `
	if _, err := new(Config).Check("p", []*syntax.File{mustParse(source)}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestNativeOptionalInvalid(t *testing.T) {
	for _, source := range []string{
		`var _ Option[int]`, `var _ int? = .Some(1)`, `var _ int? = .None`,
		`var _ = (int?).Some(1)`, `var _ = (int?).None`,
		`func f(n int) int {return switch n {case value?=>value}}`,
		`func f(n int?) int {return switch n {case nil?=>0;case nil=>0;default=>1}}`,
		`func f(n int?) int {return switch n {case nil=>0}}`,
		`func f(n (int?)?) int {return switch n {case nil=>0;case (value?)?=>value}}`,
		`func f(n int?) int {return switch n {case nil=>0;case value? if value>0=>value}}`,
		`var _ (int?)? = 3`, `var _ = ((int?)?)(3)`,
	} {
		f := mustParse("package p;" + source)
		if _, err := new(Config).Check("p", []*syntax.File{f}, nil); err == nil {
			t.Fatalf("accepted invalid native optional: %s", source)
		}
	}
}
