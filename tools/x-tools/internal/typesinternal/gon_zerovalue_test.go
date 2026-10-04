package typesinternal_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/internal/typesinternal"
)

type zeroEnumImporter struct{ pkg *types.Package }

func (i zeroEnumImporter) Import(path string) (*types.Package, error) { return i.pkg, nil }

func TestGonEnumZeroValue(t *testing.T) {
	fset := token.NewFileSet()
	src := `package lib
 type Unit enum {default hidden; Ready}
 type Pos enum {default hidden(int); Other(string)}
 type Record enum {default hidden { private int }; Ready}
 type Generic[T any] enum {default hidden(T); Other}
 type Alias = Pos
 `
	file, err := parser.ParseFile(fset, "lib.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := (&types.Config{}).Check("lib", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	qual := func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	}
	fixtures := map[string]types.Type{}
	for _, name := range []string{"Unit", "Pos", "Record", "Alias"} {
		fixtures["lib."+name] = lib.Scope().Lookup(name).Type()
	}
	generic, err := types.Instantiate(nil, lib.Scope().Lookup("Generic").Type(), []types.Type{types.Typ[types.Int]}, true)
	if err != nil {
		t.Fatal(err)
	}
	fixtures["lib.Generic[int]"] = generic
	for name, args := range map[string][]types.Type{"Result": {types.Typ[types.Int], types.Universe.Lookup("error").Type()}} {
		typ, err := types.Instantiate(nil, types.Universe.Lookup(name).Type(), args, true)
		if err != nil {
			t.Fatal(err)
		}
		fixtures[types.TypeString(typ, qual)] = typ
	}
	fixtures["int?"] = types.NewOptional(types.Typ[types.Int])
	for name, typ := range fixtures {
		t.Run(name, func(t *testing.T) {
			if _, valid := typesinternal.ZeroString(typ.Underlying(), qual); valid && !types.IsOptional(typ) {
				t.Fatal("ZeroString accepted an unnamed enum descriptor")
			}
			if _, valid := typesinternal.ZeroExpr(typ.Underlying(), qual); valid && !types.IsOptional(typ) {
				t.Fatal("ZeroExpr accepted an unnamed enum descriptor")
			}
			zero, valid := typesinternal.ZeroString(typ, qual)
			if !valid || zero != "*new("+name+")" {
				t.Fatalf("ZeroString=%q,%v", zero, valid)
			}
			expr, valid := typesinternal.ZeroExpr(typ, qual)
			if !valid {
				t.Fatal("invalid ZeroExpr")
			}
			var buf bytes.Buffer
			if err := printer.Fprint(&buf, fset, expr); err != nil {
				t.Fatal(err)
			}
			if buf.String() != zero {
				t.Fatalf("ZeroExpr=%q, want %q", buf.String(), zero)
			}
			for _, value := range []string{zero, buf.String()} {
				source := "package client;import \"lib\";var _ lib.Unit;var _ " + name + " = " + value
				generated, err := parser.ParseFile(fset, "zero.go", source, parser.SkipObjectResolution)
				if err != nil {
					t.Fatal(err)
				}
				conf := types.Config{Importer: zeroEnumImporter{lib}}
				if _, err := conf.Check("client", fset, []*ast.File{generated}, nil); err != nil {
					t.Fatalf("generated zero failed: %s: %v", source, err)
				}
			}
			if strings.Contains(zero, "hidden") {
				t.Fatal("zero construction requires private variant")
			}
		})
	}
}
