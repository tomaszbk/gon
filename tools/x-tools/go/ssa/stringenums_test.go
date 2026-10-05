package ssa_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/interp"
	"golang.org/x/tools/go/ssa/ssautil"
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
 println("PASS")
}
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []ssa.BuilderMode{0, ssa.InstantiateGenerics} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			// Imported package functions have type information and no syntax.
			p, _, err := ssautil.BuildPackage(&types.Config{Importer: stringEnumImporter{pkg}}, fs, types.NewPackage("main", "main"), []*ast.File{main}, ssa.SanityCheckFunctions|ssa.GlobalDebug|ssa.BareInits|mode)
			if err != nil {
				t.Fatal(err)
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
			if mode&ssa.InstantiateGenerics != 0 {
				if code := interp.Interpret(p, 0, types.SizesFor("gc", runtime.GOARCH), "main", nil); code != 0 {
					t.Fatalf("SSA execution failed: %d", code)
				}
			}
		})
	}
}

func TestGonStringEnums(t *testing.T) {
	root, baseline := os.Getenv("GON_ROOT"), os.Getenv("GON_BASELINE_GO")
	if root == "" || baseline == "" {
		t.Skip("set GON_ROOT and GON_BASELINE_GO for executable pairs")
	}
	gonName := "gon"
	if runtime.GOOS == "windows" {
		gonName += ".exe"
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			path := filepath.Join(root, "misc/gon/analysisfixtures/stringenums_"+variant+".go")
			toolchains := []string{filepath.Join(root, "gon/bin", gonName)}
			if variant == "legacy" {
				toolchains = append(toolchains, baseline)
			}
			for _, tool := range toolchains {
				cmd := exec.Command(tool, "run", path)
				cmd.Dir = t.TempDir()
				for _, entry := range os.Environ() {
					key, _, _ := strings.Cut(entry, "=")
					switch key {
					case "GOROOT", "GOTOOLDIR", "GOTOOLCHAIN", "GOWORK", "GOFLAGS", "GOENV":
						continue
					}
					cmd.Env = append(cmd.Env, entry)
				}
				cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GOENV=off")
				out, err := cmd.CombinedOutput()
				if err != nil || string(out) != "PASS\n" {
					t.Errorf("%s: %v\n%s", tool, err, out)
				}
			}
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, path, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			p, _, err := ssautil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ssa.SanityCheckFunctions|ssa.InstantiateGenerics|ssa.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if code := interp.Interpret(p, 0, types.SizesFor("gc", runtime.GOARCH), "main", nil); code != 0 {
				t.Fatalf("SSA execution failed: %d", code)
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
	pkg, _, err := ssautil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{file}, ssa.SanityCheckFunctions|ssa.InstantiateGenerics|ssa.GlobalDebug)
	if err != nil {
		t.Fatal(err)
	}
	if code := interp.Interpret(pkg, 0, types.SizesFor("gc", runtime.GOARCH), "main", nil); code != 0 {
		t.Fatalf("SSA execution failed: %d", code)
	}
}
