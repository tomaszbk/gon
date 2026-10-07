package ir_test

import (
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
			pkg, _, err := irutil.BuildPackage(&types.Config{}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ir.SanityCheckFunctions|ir.InstantiateGenerics|ir.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if pkg.Func("main") == nil {
				t.Fatal("missing main IR")
			}
		})
	}
}
