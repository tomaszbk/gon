package gcimporter_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/internal/gcimporter"
)

func TestGonAlternatives(t *testing.T) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "lib.go", `package lib
type E[T any] enum { Value(T); default Empty; Record { Item T; Ready bool } }
type Alias = E[int]
type Maybe = int?
func Combine(first int, second string) {}
var Constructor = E[int].Value
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	p, err := new(types.Config).Check("example/lib", fs, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := gcimporter.IExportShallow(fs, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]*types.Package{}
	importedFS := token.NewFileSet()
	q, err := gcimporter.IImportShallow(importedFS, gcimporter.GetPackagesFromMap(imports), data, p.Path(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Alias"} {
		original, imported := types.EnumOf(p.Scope().Lookup(name).Type()), types.EnumOf(q.Scope().Lookup(name).Type())
		if imported == nil || imported.NumVariants() != original.NumVariants() {
			t.Fatalf("descriptor lost for %s", name)
		}
		for i := range original.NumVariants() {
			a, b := original.Variant(i), imported.Variant(i)
			if a.Name() != b.Name() || a.Tag() != b.Tag() || a.IsRecord() != b.IsRecord() || a.NumFields() != b.NumFields() || b.Object() == nil {
				t.Fatalf("variant metadata lost for %s: %v", name, b)
			}
			if fs.Position(a.Object().Pos()).Line != importedFS.Position(b.Object().Pos()).Line {
				t.Fatalf("variant declaration position lost for %s.%s", name, a.Name())
			}
			for j := range a.NumFields() {
				if a.Field(j).Name() != b.Field(j).Name() || types.TypeString(a.Field(j).Type(), nil) != types.TypeString(b.Field(j).Type(), nil) {
					t.Fatalf("payload metadata lost %s.%s", name, a.Name())
				}
			}
		}
	}
	if !types.IsOptional(q.Scope().Lookup("Maybe").Type()) {
		t.Fatal("optional identity changed on cache import")
	}
	if types.EnumOf(q.Scope().Lookup("Maybe").Type()) != nil || types.OptionalOf(q.Scope().Lookup("Maybe").Type()).Elem() != types.Typ[types.Int] {
		t.Fatal("native optional export identity")
	}
	sig := q.Scope().Lookup("Combine").Type().(*types.Signature)
	if sig.Params().At(0).Name() != "first" || sig.Params().At(1).Name() != "second" {
		t.Fatal("static parameter names lost")
	}
	client, err := parser.ParseFile(fs, "client.go", `package client
import "example/lib"
var _ lib.Alias = lib.Alias.Record{Item:3}
var _ = lib.Alias.Value(4)
var _ lib.Maybe = 5
func F(v lib.Alias) int { lib.Combine(second:"s",first:1); return switch v { case lib.Alias.Empty => 0; case lib.Alias.Value(n) => n; case lib.Alias.Record{Item:n,...} => n } }
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (&types.Config{Importer: enumTestImporter{q}}).Check("client", fs, []*ast.File{client}, nil)
	if err != nil {
		t.Fatalf("cached client check: %v", err)
	}
}

type enumTestImporter struct{ p *types.Package }

func (i enumTestImporter) Import(string) (*types.Package, error) { return i.p, nil }
