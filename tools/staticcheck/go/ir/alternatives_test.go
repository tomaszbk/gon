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

func TestGonAlternatives(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Skip("set GON_ROOT for executable pair fixtures")
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, filepath.Join(root, "misc/gon/analysisfixtures/alternatives_"+variant+".go"), nil, parser.SkipObjectResolution)
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
func TestGonSyntaxSimplification(t *testing.T) {
	root := os.Getenv("GON_ROOT")
	if root == "" {
		t.Skip("set GON_ROOT for executable pair fixtures")
	}
	for _, variant := range []string{"legacy", "modern"} {
		t.Run(variant, func(t *testing.T) {
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, filepath.Join(root, "misc/gon/analysisfixtures/simplification_"+variant+".go"), nil, parser.SkipObjectResolution)
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
