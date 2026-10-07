package assertiontree

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

func TestFunctionAssignmentWithoutObjectResolution(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "closure.go", `package p
func host() {
 f, g := func() int { return 1 }, func() int { return 2 }
 _, _ = f(), g()
}`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object)}
	pkg, err := new(types.Config).Check("example.com/closures", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	pass := analysishelper.NewEnhancedPass(&analysis.Pass{Fset: fset, Files: []*ast.File{file}, TypesInfo: info, Pkg: pkg})
	assignment := file.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.AssignStmt)
	for identifier, object := range info.Uses {
		if object.Name() != "f" && object.Name() != "g" {
			continue
		}
		index := 0
		if object.Name() == "g" {
			index = 1
		}
		if got := getFuncLitFromAssignment(identifier, pass); got != assignment.Rhs[index] {
			t.Fatalf("assignment of %s was not resolved to its own function", identifier.Name)
		}
	}
}
