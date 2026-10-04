package types_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	. "go/types"
	"testing"
)

func TestEnumConstructorMetadata(t *testing.T) {
	fset := token.NewFileSet()
	source := `package p
 type Box[T any] enum { Full(T); default Empty; Record { Field T } }
 var record = Box[string].Record{Field: "value"}
 var b = Box[string].Full("payload")
 var o = (string?)((string)("payload"))
 type Alias = string?
 var a Alias
 var r Result[int,error]
 `
	file, err := parser.ParseFile(fset, "metadata.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	info := &Info{Defs: map[*ast.Ident]Object{}, Uses: map[*ast.Ident]Object{}, Types: map[ast.Expr]TypeAndValue{}}
	pkg, err := (&Config{}).Check("p", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b"} {
		typ := pkg.Scope().Lookup(name).Type()
		desc := EnumOf(typ)
		ctor := desc.Lookup("Full", pkg)
		sig, ok := ctor.Object().Type().(*Signature)
		if !ok || sig.Params().Len() != 1 || sig.Params().At(0).Name() != "" || sig.Params().At(0).Type() != Typ[String] || !Identical(sig.Results().At(0).Type(), typ) {
			t.Fatalf("%s constructor metadata = %v", name, ctor.Object())
		}
	}
	for id, obj := range info.Defs {
		if id.Name == "Empty" && obj.Pos() != id.Pos() {
			t.Fatal("default variant object position must point to its name")
		}
	}
	origin := EnumOf(pkg.Scope().Lookup("Box").Type())
	instantiated := EnumOf(pkg.Scope().Lookup("b").Type())
	if instantiated.Lookup("Full", pkg).Object().(*Func).Origin() != origin.Lookup("Full", pkg).Object() || instantiated.Lookup("Record", pkg).Field(0).Origin() != origin.Lookup("Record", pkg).Field(0) {
		t.Fatal("instantiated enum declaration origins were lost")
	}
	if EnumOf(pkg.Scope().Lookup("o").Type()) != nil || OptionalOf(pkg.Scope().Lookup("o").Type()).Elem() != Typ[String] || Universe.Lookup("Option") != nil {
		t.Fatal("native optional identity leaked enum constructors")
	}
	if !IsOptional(pkg.Scope().Lookup("a").Type()) || !IsCanonicalResult(pkg.Scope().Lookup("r").Type()) {
		t.Fatal("canonical alias identity was lost")
	}
	if EnumOf(pkg.Scope().Lookup("b").Type()).Default().Tag() != 0 {
		t.Fatal("reordered default changed its zero tag")
	}
}

func TestEnumCanonicalNamesShadow(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "shadow.go", `package p; type Option[T any] enum { default None; Some(T) }; type Result[T,E any] struct { T T; E E }; var o Option[int]; var r Result[int,error]`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&Config{}).Check("p", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if IsOptional(pkg.Scope().Lookup("o").Type()) || IsCanonicalResult(pkg.Scope().Lookup("r").Type()) {
		t.Fatal("shadowing declaration adopted canonical protocol")
	}
}
