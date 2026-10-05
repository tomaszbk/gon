package objectpath_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/types/objectpath"
)

func TestEnumPaths(t *testing.T) {
	const source = `package p
	type E enum { default Empty; Int(int); Record { Count int } }
	type Maybe[T any] enum { default None; Some(T) }
	type Role[T any] enum string { default Unknown(string); Teacher = "teacher" }
	var _ = Maybe[int].Some(1)
	var _ = Role[int].Parse("teacher")
	`
	check := func() (*types.Package, *types.Info) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "enum.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
		pkg, err := new(types.Config).Check("p", fset, []*ast.File{file}, info)
		if err != nil {
			t.Fatal(err)
		}
		return pkg, info
	}
	pkg, info := check()
	other, _ := check()
	enum := types.EnumOf(pkg.Scope().Lookup("E").Type())
	type expectation struct {
		obj  types.Object
		path objectpath.Path
	}
	objects := []expectation{
		{enum.Variant(0).Object(), "E.V0"},
		{enum.Variant(1).Object(), "E.V1"},
		{enum.Variant(2).Object(), "E.V2"},
		{enum.Variant(2).Field(0), "E.B2F0"},
	}
	for id, obj := range info.Uses {
		if id.Name == "Some" {
			objects = append(objects, expectation{obj, "Maybe.V1"})
		}
		if id.Name == "Parse" {
			objects = append(objects, expectation{obj, "Role.Z"})
		}
	}
	var encoder objectpath.Encoder
	for _, item := range objects {
		path, err := encoder.For(item.obj)
		if err != nil || path != item.path {
			t.Fatalf("For(%v) = %q, %v; want %q", item.obj, path, err, item.path)
		}
		decoded, err := objectpath.Object(other, path)
		if err != nil || decoded.Name() != item.obj.Name() {
			t.Fatalf("Object(%q) = %v, %v", path, decoded, err)
		}
		if roundtrip, err := encoder.For(decoded); err != nil || roundtrip != path {
			t.Fatalf("second-package roundtrip = %q, %v; want %q", roundtrip, err, path)
		}
	}
	for _, path := range []objectpath.Path{"E.V99", "E.B99F0", "E.B2F99", "E.B2", "E.V", "E.B", "E.Z", "Role.Z1"} {
		if _, err := objectpath.Object(pkg, path); err == nil {
			t.Errorf("invalid path %q accepted", path)
		}
	}
}

// TestEnumPathsPositionalPayloads covers variants with two or more positional
// payloads. Their fields are all unnamed, so the payload cannot be represented
// as a types.Struct (types.NewStruct rejects duplicate field names); a package
// with such a variant used to panic any search of that package, for example
// the path of an unrelated method of a generic enum.
func TestEnumPathsPositionalPayloads(t *testing.T) {
	const source = `package p
	type G[T any] enum { default Empty; Full(T) }
	func (G[T]) Error() string { return "" }
	type E enum { default A; R(int, string); Q { X int; Y string } }
	type P enum { default Z; Triple(int, string, bool) }
	`
	check := func() *types.Package {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "enum.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := new(types.Config).Check("p", fset, []*ast.File{file}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return pkg
	}
	pkg, other := check(), check()
	enum := func(name string) *types.Enum { return types.EnumOf(pkg.Scope().Lookup(name).Type()) }
	method := pkg.Scope().Lookup("G").Type().(*types.Named).Method(0)
	type expectation struct {
		obj  types.Object
		path objectpath.Path
	}
	objects := []expectation{
		{method, "G.M0"},
		{enum("E").Variant(1).Object(), "E.V1"},
		{enum("E").Variant(1).Field(0), "E.B1F0"},
		{enum("E").Variant(1).Field(1), "E.B1F1"},
		{enum("E").Variant(2).Field(0), "E.B2F0"},
		{enum("E").Variant(2).Field(1), "E.B2F1"},
		{enum("P").Variant(1).Field(0), "P.B1F0"},
		{enum("P").Variant(1).Field(1), "P.B1F1"},
		{enum("P").Variant(1).Field(2), "P.B1F2"},
		{enum("G").Variant(1).Field(0), "G.B1F0"},
	}
	// Reuse one encoder so that later searches exercise the index, and
	// repeat each query to compare the traversal and indexed answers.
	var encoder objectpath.Encoder
	for round := 0; round < 3; round++ {
		for _, item := range objects {
			path, err := encoder.For(item.obj)
			if err != nil || path != item.path {
				t.Fatalf("round %d: For(%v) = %q, %v; want %q", round, item.obj, path, err, item.path)
			}
			decoded, err := objectpath.Object(other, path)
			if err != nil || decoded.Name() != item.obj.Name() || decoded == item.obj {
				t.Fatalf("round %d: Object(%q) = %v, %v", round, path, decoded, err)
			}
			if roundtrip, err := encoder.For(decoded); err != nil || roundtrip != path {
				t.Fatalf("round %d: second-package roundtrip = %q, %v; want %q", round, roundtrip, err, path)
			}
		}
	}
	if obj, err := objectpath.Object(pkg, "E.B1F1"); err != nil || obj != enum("E").Variant(1).Field(1) {
		t.Fatalf("Object(E.B1F1) = %v, %v; want the second payload field", obj, err)
	}
	for _, path := range []objectpath.Path{"E.B1F2", "E.B1", "E.B1.", "E.B1M0", "E.B1E", "E.B1A0", "E.B1B1F0", "P.B1F3", "E.B3F0", "E.B1F"} {
		if _, err := objectpath.Object(pkg, path); err == nil {
			t.Errorf("invalid path %q accepted", path)
		}
	}
}

type stringEnumImporter struct{ pkg *types.Package }

func (i stringEnumImporter) Import(string) (*types.Package, error) { return i.pkg, nil }

func TestStringEnumDerivedParserPath(t *testing.T) {
	check := func(path, source string, importer types.Importer) *types.Package {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path+".go", source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := (&types.Config{Importer: importer}).Check(path, fset, []*ast.File{file}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return pkg
	}
	lib := check("lib", `package lib; type Role enum string { default Unknown(string); Teacher = "teacher" }`, nil)
	client := check("client", `package client; import "lib"; type Derived lib.Role; var _ = Derived.Parse("teacher")`, stringEnumImporter{lib})
	parser := types.EnumOf(client.Scope().Lookup("Derived").Type()).StringParser()
	path, err := objectpath.For(parser)
	if err != nil || path != "Derived.Z" || parser.Pkg() != client {
		t.Fatalf("derived parser path=%q, package=%v, error=%v", path, parser.Pkg(), err)
	}
	if object, err := objectpath.Object(client, path); err != nil || object != parser {
		t.Fatalf("derived parser path did not round trip: %v, %v", object, err)
	}
}
