package anonymousfunc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"testing"

	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

func TestClosureWithoutObjectResolution(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "closure.go", `package p
func target(value int) int { return value }
func host(p *int) {
 local := 1
 _ = func(x int) int {
  q := x
  if true { tmp := 1; _ = tmp }
  _ = func() int { return *p + q }
  var s struct { Field int }
  return target(value: local) + s.Field
 }
}`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object), Scopes: make(map[ast.Node]*types.Scope)}
	pkg, err := new(types.Config).Check("example.com/closures", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	var literals []*ast.FuncLit
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncLit); ok {
			literals = append(literals, fn)
		}
		return true
	})
	closures := make(map[*ast.FuncLit][]*VarInfo)
	collectClosure(literals[0], analysishelper.NewEnhancedPass(&analysis.Pass{Fset: fset, Files: []*ast.File{file}, TypesInfo: info, Pkg: pkg}), closures)
	for i, want := range [][]string{{"local", "p"}, {"p", "q"}} {
		var got []string
		for _, variable := range closures[literals[i]] {
			got = append(got, variable.Ident.Name)
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("closure %d captures %v, want %v", i, got, want)
		}
	}
}
