package gcimporter_test

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/gcexportdata"
	"golang.org/x/tools/internal/gcimporter"
)

const simplificationLib = `package lib
type Maybe = int?
type Outcome[T any] = Result[T?,string]
func Some[T any](value T) (out T?) { return (T)(value) }
func Lift[T any](value T) (out T?) { return value }
func Absent[T any]() (out T?) { return nil }
func Wrap[T any](value T, fail bool) (out Outcome[T]) {
 if fail { return .Err("bad") }
 return .Ok(value)
}
func Zero[T any]() (out Outcome[T]) { return .Ok(nil) }
func NilPresent() (out (*int)?) { return (*int)(nil) }
`

const simplificationClient = `package main
import "example/lib"
func main() {
 if (lib.Some(7) ?? 0) != 7 { panic("Some") }
 if (lib.Lift(8) ?? 0) != 8 { panic("implicit wrapping") }
 switch lib.Absent[int]() { case nil => {}; default => { panic("None") } }
 var pointer *int
 switch lib.Lift(pointer) { case p? if p == nil => {}; default => { panic("typed nil presence") } }
 switch lib.NilPresent() { case p? if p == nil => {}; default => { panic("explicit nil presence") } }
 switch lib.Wrap(fail: false, value: 9) { case Result[int?,string].Ok(n?) if n == 9 => {}; default => { panic("success") } }
 switch lib.Zero[int]() { case Result[int?,string].Ok(nil) => {}; default => { panic("nil success") } }
 switch lib.Wrap(value: 10, fail: true) { case Result[int?,string].Err(problem) if problem == "bad" => {}; default => { panic("failure") } }
 var alias lib.Maybe = 11
 if (alias ?? 0) != 11 { panic("exported alias") }
 println("PASS")
}
`

const simplificationLegacyLib = `package lib
type Option[T any] struct { Present bool; Value T }
type Maybe = Option[int]
type Outcome[T any] struct { Failed bool; Value Option[T]; Problem string }
func Some[T any](value T) (out Option[T]) { return Option[T]{true,value} }
func Lift[T any](value T) (out Option[T]) { return Option[T]{true,value} }
func Absent[T any]() (out Option[T]) { return Option[T]{} }
func Wrap[T any](value T, fail bool) (out Outcome[T]) {
 if fail { return Outcome[T]{Failed:true,Problem:"bad"} }
 return Outcome[T]{Value:Option[T]{true,value}}
}
func Zero[T any]() (out Outcome[T]) { return Outcome[T]{} }
func NilPresent() (out Option[*int]) { return Option[*int]{true,nil} }
`

const simplificationLegacyClient = `package main
import "example/lib"
func main() {
 if p:=lib.Some(7); !p.Present || p.Value!=7 { panic("Some") }
 if p:=lib.Lift(8); !p.Present || p.Value!=8 { panic("implicit wrapping") }
 if p:=lib.Absent[int](); p.Present { panic("None") }
 var pointer *int
 if p:=lib.Lift(pointer); !p.Present || p.Value!=nil { panic("typed nil presence") }
 if p:=lib.NilPresent(); !p.Present || p.Value!=nil { panic("explicit nil presence") }
 if r:=lib.Wrap(9,false); r.Failed || !r.Value.Present || r.Value.Value!=9 { panic("success") }
 if r:=lib.Zero[int](); r.Failed || r.Value.Present { panic("nil success") }
 if r:=lib.Wrap(10,true); !r.Failed || r.Problem!="bad" { panic("failure") }
 var alias lib.Maybe = lib.Option[int]{true,11}
 if !alias.Present || alias.Value!=11 { panic("exported alias") }
 println("PASS")
}
`

func simplificationModule(t *testing.T, library, client string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "lib"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"go.mod": "module example\n\ngo 1.27\n", "lib/lib.go": library, "main.go": client} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func simplificationCommand(t *testing.T, tool, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(tool, args...)
	cmd.Dir = dir
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GOROOT", "GOTOOLDIR", "GOTOOLCHAIN", "GOWORK", "GOFLAGS", "GOENV", "GOEXPERIMENT":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GOENV=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", tool, args, err, out)
	}
	return out
}

func assertSimplificationSignatures(t *testing.T, pkg *types.Package) {
	t.Helper()
	if !types.IsOptional(pkg.Scope().Lookup("Maybe").Type()) {
		t.Fatal("exported optional alias lost canonical identity")
	}
	for _, name := range []string{"Some", "Lift", "Absent", "Wrap", "Zero", "NilPresent"} {
		sig := pkg.Scope().Lookup(name).Type().(*types.Signature)
		if sig.Results().Len() != 1 || sig.Results().At(0).Name() != "out" {
			t.Fatalf("%s named single result lost: %v", name, sig)
		}
		result := sig.Results().At(0).Type()
		if name == "Wrap" || name == "Zero" {
			if !types.IsCanonicalResult(result) {
				t.Fatalf("%s result identity lost", name)
			}
			desc := types.EnumOf(result)
			var payload types.Type
			for i := range desc.NumVariants() {
				if v := desc.Variant(i); v.Name() == "Ok" {
					payload = v.Field(0).Type()
				}
			}
			if !types.IsOptional(payload) {
				t.Fatalf("%s success payload optional identity lost", name)
			}
		} else if !types.IsOptional(result) {
			t.Fatalf("%s optional result identity lost", name)
		}
		if sig.Params().Len() > 0 && sig.Params().At(0).Name() != "value" {
			t.Fatalf("%s parameter name lost", name)
		}
		if name == "Wrap" && sig.Params().At(1).Name() != "fail" {
			t.Fatal("reordered named argument signature lost")
		}
	}
}

// TestGonAlternativesSimplification validates compiler-produced unified data,
// both maintained cache formats, and importing generic bodies in a second package.
func TestGonAlternativesSimplification(t *testing.T) {
	root, baseline := os.Getenv("GON_ROOT"), os.Getenv("GON_BASELINE_GO")
	if root == "" || baseline == "" {
		t.Skip("set GON_ROOT and GON_BASELINE_GO for compiler/export executable pairs")
	}
	tool := filepath.Join(root, "gon", "bin", "gon")
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	legacy := simplificationModule(t, simplificationLegacyLib, simplificationLegacyClient)
	modern := simplificationModule(t, simplificationLib, simplificationClient)
	for _, tc := range []struct{ name, tool, dir string }{{"baseline legacy", baseline, legacy}, {"Gon legacy", tool, legacy}, {"Gon modern", tool, modern}} {
		t.Run(tc.name, func(t *testing.T) {
			if out := simplificationCommand(t, tc.tool, tc.dir, "run", "."); string(out) != "PASS\n" {
				t.Fatalf("observable behavior differs: %q", out)
			}
		})
	}
	t.Run("imported bodies without inlining", func(t *testing.T) {
		if out := simplificationCommand(t, tool, modern, "run", "-gcflags=example/lib=-l", "."); string(out) != "PASS\n" {
			t.Fatalf("observable behavior differs: %q", out)
		}
	})
	archive := filepath.Join(t.TempDir(), "lib.a")
	simplificationCommand(t, tool, modern, "build", "-buildmode=archive", "-o", archive, "./lib")
	fs := token.NewFileSet()
	archiveFile, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer archiveFile.Close()
	reader, err := gcexportdata.NewReader(archiveFile)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || data[0] != 'p' && data[0] != 'u' {
		t.Fatalf("unexpected compiler archive format %q", data[:min(len(data), 1)])
	}
	// The current toolchain writes private compiler data as 'p'. This test
	// exercises its maintained unified decoder directly; public importers
	// continue to use the cmd/export file above and do not accept private data.
	_, unified, err := gcimporter.UImportData(fs, map[string]*types.Package{}, data[1:], "example/lib")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("unified", func(t *testing.T) { assertSimplificationSignatures(t, unified) })
	t.Run("indexed", func(t *testing.T) {
		var data bytes.Buffer
		if err := gcimporter.IExportData(&data, fs, unified); err != nil {
			t.Fatal(err)
		}
		pkg, err := gcimporter.IImportData(token.NewFileSet(), map[string]*types.Package{}, data.Bytes(), "example/lib")
		if err != nil {
			t.Fatal(err)
		}
		assertSimplificationSignatures(t, pkg)
		assertSimplificationClient(t, pkg)
	})
	t.Run("shallow", func(t *testing.T) {
		data, err := gcimporter.IExportShallow(fs, unified, nil)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := gcimporter.IImportShallow(token.NewFileSet(), gcimporter.GetPackagesFromMap(map[string]*types.Package{}), data, "example/lib", nil)
		if err != nil {
			t.Fatal(err)
		}
		assertSimplificationSignatures(t, pkg)
		assertSimplificationClient(t, pkg)
	})
	assertSimplificationClient(t, unified)
	publicExport := strings.TrimSpace(string(simplificationCommand(t, tool, modern, "list", "-export", "-f", "{{.Export}}", "./lib")))
	t.Run("public compiler export", func(t *testing.T) {
		pkg, err := gcimporter.Import(token.NewFileSet(), map[string]*types.Package{}, "example/lib", modern, func(string) (io.ReadCloser, error) { return os.Open(publicExport) })
		if err != nil {
			t.Fatal(err)
		}
		assertSimplificationSignatures(t, pkg)
		assertSimplificationClient(t, pkg)
	})
	t.Run("public client export", func(t *testing.T) {
		simplificationCommand(t, tool, modern, "list", "-export", "-f", "{{.Export}}", ".")
	})
	t.Run("toolchain importer", func(t *testing.T) {
		pkg, err := importer.ForCompiler(token.NewFileSet(), "gc", func(string) (io.ReadCloser, error) { return os.Open(publicExport) }).Import("example/lib")
		if err != nil {
			t.Fatal(err)
		}
		assertSimplificationSignatures(t, pkg)
	})
}

func assertSimplificationClient(t *testing.T, pkg *types.Package) {
	t.Helper()
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "main.go", simplificationClient, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&types.Config{Importer: enumTestImporter{pkg}}).Check("client", fs, []*ast.File{file}, &types.Info{OptionalConversions: map[ast.Expr]types.Type{}}); err != nil {
		t.Fatalf("cached contextual client: %v", err)
	}
}
