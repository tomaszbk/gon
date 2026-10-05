package types2_test

import (
	"cmd/compile/internal/syntax"
	. "cmd/compile/internal/types2"
	"testing"
)

func TestStringEnumMetadata(t *testing.T) {
	pkg := mustTypecheck(`package p; const teacher = "teach" + "er"; type Role[T any] enum string { Student = "student"; default Unknown(string); Teacher = teacher; Empty = "" }; type Alias = Role[int]; var role Role[int]; type Derived Role[int]; var derived Derived = Derived(role)`, nil, nil)
	for _, name := range []string{"Role", "Alias", "role", "Derived", "derived"} {
		e := EnumOf(pkg.Scope().Lookup(name).Type())
		if !e.IsString() || e.Default().Name() != "Unknown" || e.Default().Tag() != 0 {
			t.Fatalf("string enum metadata lost for %s", name)
		}
		if _, ok := e.Default().StringValue(); ok {
			t.Fatal("fallback must not have a fixed spelling")
		}
		for variant, want := range map[string]string{"Teacher": "teacher", "Student": "student", "Empty": ""} {
			if got, ok := e.Lookup(variant, pkg).StringValue(); !ok || got != want {
				t.Fatalf("%s.%s spelling = %q, %v", name, variant, got, ok)
			}
		}
	}
}

func TestStringEnumInvalidDeclarations(t *testing.T) {
	for _, body := range []string{
		`enum string {}`,
		`enum string { default Unknown; A = "a" }`,
		`enum string { default Unknown(int); A = "a" }`,
		`enum string { default Unknown(string, string); A = "a" }`,
		`enum string { default Unknown { Text string }; A = "a" }`,
		`enum string { default Unknown(string) = "unknown"; A = "a" }`,
		`enum string { default Unknown(string); A }`,
		`enum string { default Unknown(string); A(int) = "a" }`,
		`enum string { default Unknown(string); A{} = "a" }`,
		`enum string { default Unknown(string); A = 1 }`,
		`enum string { default Unknown(string); A = text }`,
		`enum string { default Unknown(string); A = "a"; B = "a" }`,
		`enum string { default Unknown(string); Parse = "parse" }`,
		`enum { default Unknown; A = "a" }`,
		`enum string { default Unknown(string); A = "a" }; func (Role) Parse() {}`,
		`enum string { default Unknown(string); A = "a" }; type Ordinary enum { default Unknown(string); A }; var _ = Role(Ordinary.A)`,
		`enum string { default Unknown(string); A = "a" }; type Derived Role; func (Derived) Parse() {}`,
	} {
		t.Run(body, func(t *testing.T) {
			if _, err := typecheck("package p;var text = \"a\";type Role "+body, nil, nil); err == nil {
				t.Fatal("accepted invalid string enum")
			}
		})
	}
}

// Declarations that refer to each other through a type literal are valid, and
// the declaration of one of them can complete while the right-hand side of
// another is still unset. String enum method synthesis must not force the
// underlying type of such an incomplete declaration.
func TestStringEnumDeclarationCycles(t *testing.T) {
	for _, src := range []string{
		`package p; type T6 T7; type T7 *T8; type T8 T6`,
		`package p; type T8 T6; type T7 *T8; type T6 T7`,
		`package p; type A B; type B []A; var _ A`,
		`package p; type B enum { default N; Next(*A) }; type A B; var _ = A.N`,
		`package p; type A B; type B enum { default N; Next(*A) }; var _ = B.Next(nil)`,
		`package p; type A = B; type B *A`,
		`package p; type G[T any] H[T]; type H[T any] *G[T]`,
		`package p; type Role enum string { default Unknown(string); A = "a" }; type D Role; type E D; var _ = E.Parse("a")`,
	} {
		if _, err := typecheck(src, nil, nil); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
	// Invalid cycles are diagnosed, not crashed on or looped over.
	for _, src := range []string{
		`package p; type T1 T1`,
		`package p; type T3 T4; type T4 T5; type T5 T3`,
		`package p; func f() { type T T }`,
		`package p; func f() { type A = A }`,
	} {
		if _, err := typecheck(src, nil, nil); err == nil {
			t.Errorf("%s: invalid cycle accepted", src)
		}
	}
}

func TestStringEnumContextualMarker(t *testing.T) {
	mustTypecheck(`package p; type Text = string; func f() { type string int; type Role enum string { default Unknown(Text); A = "a" }; var role Role; _ = role; var ordinary string; _ = ordinary }`, nil, nil)
}

func TestStringEnumMalformedParse(t *testing.T) {
	var diagnostics []error
	config := &Config{Error: func(err error) { diagnostics = append(diagnostics, err) }}
	if _, err := typecheck(`package p; type E enum string {}; var e = E.Parse("x")`, config, nil); err == nil || len(diagnostics) == 0 {
		t.Fatal("malformed string enum must report errors while continuing to check uses")
	}
}

func TestStringEnumDerivedParserPackage(t *testing.T) {
	lib := mustTypecheck(`package lib; type Role enum string { default Unknown(string); Teacher = "teacher" }`, nil, nil)
	info := &Info{Uses: map[*syntax.Name]Object{}}
	client := mustTypecheck(`package client; import "lib"; type Derived lib.Role; var _ = Derived.Parse("teacher")`, &Config{Importer: testImporter{"lib": lib}}, info)
	owner := client.Scope().Lookup("Derived")
	parser := EnumOf(owner.Type()).StringParser()
	if parser.Pkg() != client || parser.Pos() != owner.Pos() || parser.Signature().Params().At(0).Pkg() != client {
		t.Fatalf("derived parser must belong to its declaring package: %v", parser)
	}
	for id, obj := range info.Uses {
		if id.Value == "Parse" && obj != parser {
			t.Fatalf("derived parser use resolved to %v, want %v", obj, parser)
		}
	}
}
