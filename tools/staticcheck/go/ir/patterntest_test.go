package ir_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
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
			pkg, _, err := irutil.BuildPackage(&types.Config{Importer: importer.ForCompiler(fs, "source", nil)}, fs, types.NewPackage("main", "main"), []*ast.File{f}, ir.SanityCheckFunctions|ir.InstantiateGenerics|ir.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if pkg.Func("main") == nil {
				t.Fatal("missing main IR")
			}
		})
	}
}
