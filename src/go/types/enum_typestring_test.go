package types_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	. "go/types"
	"testing"
)

func TestEnumTypeString(t *testing.T) {
	for _, want := range []string{
		"enum{default hidden; Ready}",
		"enum{Pair(int, string); default Empty}",
		"enum{default Record{private int; Public string}; Empty}",
		"enum{default Empty; Record{}}",
		`enum string{default Unknown(string); Teacher = "teacher"; Student = "student"}`,
	} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "enum.go", "package p;type E "+want, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		pkg, err := (&Config{}).Check("p", fset, []*ast.File{file}, nil)
		if err != nil {
			t.Fatal(err)
		}
		typ := pkg.Scope().Lookup("E").Type().Underlying()
		if got := TypeString(typ, nil); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		expr := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type
		if got := ExprString(expr); got != want {
			t.Errorf("ExprString got %q, want %q", got, want)
		}
	}
	pkg := mustTypecheck("package p;type Box[T any] enum{default Empty; Full(T)};var x Box[int]", nil, nil)
	if got := TypeString(pkg.Scope().Lookup("x").Type().Underlying(), nil); got != "enum{default Empty; Full(int)}" {
		t.Fatal(got)
	}
}
