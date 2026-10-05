package ir_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
)

type stringEnumImporter struct{ pkg *types.Package }

func (i stringEnumImporter) Import(path string) (*types.Package, error) {
	if path == i.pkg.Path() {
		return i.pkg, nil
	}
	return nil, fmt.Errorf("unexpected import %q", path)
}

func TestGonStringEnumImportedMethods(t *testing.T) {
	fs := token.NewFileSet()
	lib, err := parser.ParseFile(fs, "data.go", `package data
type Role enum string { default Unknown(string); Teacher = "teacher" }
type Alias = Role
type Generic[T any] enum string { default Other(string); Ready = "ready" }
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := new(types.Config).Check("data", fs, []*ast.File{lib}, nil)
	if err != nil {
		t.Fatal(err)
	}
	main, err := parser.ParseFile(fs, "main.go", `package main
import "data"
func main() {
 parse := data.Alias.Parse
 role := parse("teacher")
 var text interface{ String() string } = role
 if text.String() != "teacher" || data.Role.String(role) != "teacher" { panic("imported String") }
 bytes, err := role.MarshalText()
 var decoded data.Role
 if err != nil || (*data.Alias).UnmarshalText(&decoded, bytes) != nil || decoded != role { panic("imported text") }
 generic := data.Generic[int].Parse("future")
 var gtext interface{ String() string } = generic
 if gtext.String() != "future" { panic("imported generic String") }
 bytes, err = generic.MarshalText()
 var other data.Generic[string]
 if err != nil || other.UnmarshalText(bytes) != nil || other.String() != "future" { panic("imported generic text") }
}
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []ir.BuilderMode{0, ir.InstantiateGenerics} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			// Imported package functions have type information and no syntax.
			p, _, err := irutil.BuildPackage(&types.Config{Importer: stringEnumImporter{pkg}}, fs, types.NewPackage("main", "main"), []*ast.File{main}, ir.SanityCheckFunctions|ir.GlobalDebug|mode)
			if err != nil {
				t.Fatal(err)
			}
			if p.Func("main") == nil {
				t.Fatal("missing main IR")
			}
			for _, name := range []string{"Role", "Generic"} {
				named := pkg.Scope().Lookup(name).Type().(*types.Named)
				for i := range named.NumMethods() {
					method := p.Prog.FuncValue(named.Method(i))
					if method == nil || len(method.Blocks) == 0 {
						t.Fatalf("missing imported method body: %s", named.Method(i))
					}
				}
			}
		})
	}
}

func TestGonStringEnums(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Skip("set GON_ROOT for executable pair fixtures")
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, filepath.Join(root, "misc/gon/analysisfixtures/stringenums_"+variant+".go"), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			p, _, err := irutil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ir.SanityCheckFunctions|ir.InstantiateGenerics|ir.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if p.Func("main") == nil {
				t.Fatal("missing main IR")
			}
		})
	}
}

func TestGonStringEnumsLocalConstraints(t *testing.T) {
	const source = `package main
func local[T any]() string {
    type E[U interface{ ~[]T }] enum string { default Unknown(string); Known = "known" }
    value := E[[]T].Parse("future")
    format := E[[]T].String
    data, err := value.MarshalText()
    var decoded E[[]T]
    decode := (*E[[]T]).UnmarshalText
    if err != nil || decode(&decoded, data) != nil || format(decoded) != "future" { panic("local constraints") }
    return value.String()
}
func main() {
    if local[int]() != "future" || local[string]() != "future" { panic("instantiation") }
    println("PASS")
}`
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "constraints.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := irutil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{file}, ir.SanityCheckFunctions|ir.InstantiateGenerics|ir.GlobalDebug)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Func("main") == nil {
		t.Fatal("missing main IR")
	}
}
