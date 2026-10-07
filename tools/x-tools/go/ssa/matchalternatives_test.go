package ssa_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/interp"
	"golang.org/x/tools/go/ssa/ssautil"
)

// TestGonMatchAlternatives checks the paired multi-pattern contract through
// typed flow construction, including common bindings, first-success guards,
// escaping closures, nested optionals and error-tree search evaluation order.
func TestGonMatchAlternatives(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Skip("set GON_ROOT for executable pair fixtures")
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, filepath.Join(root, "test/matchalternatives.dir", variant+".go"), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			pkg, _, err := ssautil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ssa.SanityCheckFunctions|ssa.InstantiateGenerics|ssa.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if code := interp.Interpret(pkg, 0, types.SizesFor("gc", runtime.GOARCH), "main", nil); code != 0 {
				t.Fatalf("SSA execution failed: %d", code)
			}
		})
	}
}
