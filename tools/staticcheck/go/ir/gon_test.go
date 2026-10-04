package ir_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGonErrorFlow(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Skip("set GON_ROOT to run compiler/analysis executable pairs")
	}
	for _, name := range []string{"legacy", "modern"} {
		t.Run(name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, filepath.Join(root, "misc/gon/analysisfixtures", name+".go"), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			p, _, err := irutil.BuildPackage(&types.Config{}, fset, types.NewPackage("main", "main"), []*ast.File{f}, ir.SanityCheckFunctions|ir.InstantiateGenerics|ir.GlobalDebug)
			if err != nil {
				t.Fatal(err)
			}
			if p.Func("main") == nil {
				t.Fatal("missing main IR")
			}
		})
	}
}
