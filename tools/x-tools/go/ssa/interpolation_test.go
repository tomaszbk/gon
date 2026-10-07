package ssa_test

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"reflect"
	"testing"
)

func TestGonInterpolation(t *testing.T) {
	const source = `package p
import alias "fmt"
func add(first,second int) int { return first+second }
func generic[T any](x T) string { f := func() string { return $"closure ${x}" }; return $"generic ${x} ${f()}" }
func use() string { return generic(3) }
func run(c bool,xs []int) string { return $"head \${literal} 50% \n ${if c {add(second: 1,first: 2)} else {0}:%[1]v} ${xs[1:2]:%v} ${$"nested: ${3}"}" }
`
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "p.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	pkg, info, err := ssautil.BuildPackage(&types.Config{Importer: importer.ForCompiler(fs, "source", nil)}, fs, types.NewPackage("p", "p"), []*ast.File{file}, ssa.SanityCheckFunctions|ssa.InstantiateGenerics)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Interpolations) != 4 {
		t.Fatalf("typed interpolation calls: %d", len(info.Interpolations))
	}
	calls := 0
	for _, block := range pkg.Func("run").Blocks {
		for _, instruction := range block.Instrs {
			if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "Sprintf" {
				calls++
			}
		}
	}
	if calls != 2 {
		t.Fatalf("want 2 fmt.Sprintf calls, got %d", calls)
	}
	// Interpolations is an optional types.Info map. Existing SSA clients
	// requesting the standard maps must still be able to build Gon code.
	info.Interpolations = nil
	program := ssa.NewProgram(fs, ssa.SanityCheckFunctions|ssa.InstantiateGenerics)
	var create func(*types.Package)
	created := make(map[*types.Package]bool)
	create = func(p *types.Package) {
		if created[p] {
			return
		}
		created[p] = true
		program.CreatePackage(p, nil, nil, true)
		for _, imported := range p.Imports() {
			create(imported)
		}
	}
	for _, imported := range pkg.Pkg.Imports() {
		create(imported)
	}
	withoutMetadata := program.CreatePackage(pkg.Pkg, []*ast.File{file}, info, true)
	withoutMetadata.Build()
	if len(withoutMetadata.Func("run").Blocks) == 0 {
		t.Fatal("interpolation without metadata did not build")
	}
	if len(withoutMetadata.Func("use").Blocks) == 0 {
		t.Fatal("generic interpolation without metadata did not build")
	}
	formats := func(p *ssa.Package) map[string]int {
		result := make(map[string]int)
		for _, block := range p.Func("run").Blocks {
			for _, instruction := range block.Instrs {
				if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() != nil && call.Common().StaticCallee().Name() == "Sprintf" {
					result[constant.StringVal(call.Common().Args[0].(*ssa.Const).Value)]++
				}
			}
		}
		return result
	}
	if want, got := formats(pkg), formats(withoutMetadata); !reflect.DeepEqual(want, got) {
		t.Fatalf("interpolation formatting changed without metadata: want %v, got %v", want, got)
	}
}
