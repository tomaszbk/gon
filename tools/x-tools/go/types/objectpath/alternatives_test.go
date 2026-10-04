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
	var _ = Maybe[int].Some(1)
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
	for _, path := range []objectpath.Path{"E.V99", "E.B99F0", "E.B2F99", "E.B2", "E.V", "E.B"} {
		if _, err := objectpath.Object(pkg, path); err == nil {
			t.Errorf("invalid path %q accepted", path)
		}
	}
}
