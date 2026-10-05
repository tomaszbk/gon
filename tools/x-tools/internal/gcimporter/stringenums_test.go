package gcimporter_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/internal/gcimporter"
)

func TestGonStringEnums(t *testing.T) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "stringenum.go", `package lib
type Role enum string {
	Teacher = "teacher"
	default Unknown(string)
	Student = "student"
	Empty = ""
	Escaped = "name:\"value\"\nñ"
}
type Alias = Role
type Ordinary enum { default Empty; Value(string) }
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
	q, err := gcimporter.IImportShallow(token.NewFileSet(), gcimporter.GetPackagesFromMap(map[string]*types.Package{}), data, p.Path(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Role", "Alias"} {
		original, imported := types.EnumOf(p.Scope().Lookup(name).Type()), types.EnumOf(q.Scope().Lookup(name).Type())
		if imported == nil || !imported.IsString() || imported.NumVariants() != original.NumVariants() || imported.Default().Name() != "Unknown" {
			t.Fatalf("string enum identity lost for %s", name)
		}
		for i := range original.NumVariants() {
			a, b := original.Variant(i), imported.Variant(i)
			atext, aok := a.StringValue()
			btext, bok := b.StringValue()
			if a.Name() != b.Name() || a.Tag() != b.Tag() || a.NumFields() != b.NumFields() || atext != btext || aok != bok {
				t.Fatalf("string enum metadata lost for %s.%s", name, a.Name())
			}
		}
	}
	if types.EnumOf(q.Scope().Lookup("Ordinary").Type()).IsString() {
		t.Fatal("ordinary enum acquired string semantics")
	}
	client, err := parser.ParseFile(fs, "client.go", `package client
import "example/lib"
var _ lib.Alias = lib.Role.Teacher
var _ interface { String() string; MarshalText() ([]byte, error) } = lib.Role.Student
var _ interface { UnmarshalText([]byte) error } = (*lib.Role)(nil)
var parse func(string) lib.Role = lib.Role.Parse
var _ lib.Role = parse("student")
var _ lib.Role = lib.Role.Parse(text: "teacher")
var _ string = lib.Role.String(lib.Role.Teacher)
func Describe(value lib.Role) string {
	return switch value {
	case lib.Role.Teacher => "teacher"
	case lib.Role.Student => "student"
	case lib.Role.Empty => ""
	case lib.Role.Escaped => "escaped"
	case lib.Role.Unknown(text) => text
	}
}
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&types.Config{Importer: enumTestImporter{q}}).Check("client", fs, []*ast.File{client}, nil); err != nil {
		t.Fatalf("cached string enum client: %v", err)
	}
}
