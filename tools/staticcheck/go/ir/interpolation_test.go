package ir_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"testing"
)

func TestGonInterpolation(t *testing.T) {
	const source = `package p
import alias "fmt"
func add(first,second int) int { return first+second }
func run(c bool,xs []int) string { return $"head ${if c {add(second: 1,first: 2)} else {0}} ${xs[1:2]:%v} ${$"nested: ${3}"}" }
`
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "p.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	pkg, info, err := irutil.BuildPackage(&types.Config{Importer: importer.ForCompiler(fs, "source", nil)}, fs, types.NewPackage("p", "p"), []*ast.File{file}, ir.SanityCheckFunctions|ir.InstantiateGenerics)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Interpolations) != 2 {
		t.Fatalf("typed interpolation calls: %d", len(info.Interpolations))
	}
	calls := 0
	for _, block := range pkg.Func("run").Blocks {
		for _, instruction := range block.Instrs {
			if call, ok := instruction.(*ir.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "Sprintf" {
				calls++
			}
		}
	}
	if calls != 2 {
		t.Fatalf("want 2 fmt.Sprintf calls, got %d", calls)
	}
}
