package ssa_test

import (
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/interp"
)

// TestGonPatternTest checks the paired pattern-test contract through
// typed flow construction, including if and && binding scopes,
// short-circuiting, nested optionals and error-tree search evaluation order.
func TestGonPatternTest(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Skip("set GON_ROOT for executable pair fixtures")
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, filepath.Join(root, "test/patterntest.dir", variant+".go"), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			conf := loader.Config{Fset: fs, ParserMode: parser.SkipObjectResolution}
			conf.CreateFromFiles("main", f)
			loaded, err := conf.Load()
			if err != nil {
				t.Fatal(err)
			}
			// Keep imported initializers executable, even when an import is
			// currently used only as a generic constraint.
			prog := ssa.NewProgram(fs, ssa.SanityCheckFunctions|ssa.InstantiateGenerics|ssa.GlobalDebug)
			for _, info := range loaded.AllPackages {
				prog.CreatePackage(info.Pkg, info.Files, &info.Info, info.Importable)
			}
			prog.Build()
			pkg := prog.Package(loaded.Created[0].Pkg)
			if code := interp.Interpret(pkg, 0, types.SizesFor("gc", runtime.GOARCH), "main", nil); code != 0 {
				t.Fatalf("SSA execution failed: %d", code)
			}
		})
	}
}
