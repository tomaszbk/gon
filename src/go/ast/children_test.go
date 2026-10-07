package ast_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"testing"
)

func TestChildren(t *testing.T) {
	// Include handlers, lazy branches, function boundaries and object links.
	f, err := parser.ParseFile(token.NewFileSet(), "children.go", `package p
 func read() (int, error) { return 1, nil }
 func f(c bool) (int, error) {
  x := if c { read()! } else { read() or err { return 0, err } }
  g := func() int { return x }
  return g(), nil
 }`, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var visit func(ast.Node)
	visit = func(n ast.Node) {
		var want []ast.Node
		ast.Inspect(n, func(c ast.Node) bool {
			if c == n {
				return true
			}
			if c != nil {
				want = append(want, c)
			}
			return false
		})
		got := slices.Collect(ast.Children(n))
		if !slices.Equal(got, want) {
			t.Fatalf("%T children = %v, want %v", n, got, want)
		}
		count := 0
		for range ast.Children(n) {
			count++
			break
		}
		if len(got) > 0 && count != 1 {
			t.Fatal("iterator did not stop")
		}
		if !slices.Equal(slices.Collect(ast.Children(n)), got) {
			t.Fatal("iterator not reusable")
		}
		for _, child := range got {
			visit(child)
		}
	}
	visit(f)
	x, y, z := ast.NewIdent("c"), ast.NewIdent("a"), ast.NewIdent("b")
	cond := &ast.CondExpr{Cond: x, Then: y, Else: z}
	if got := slices.Collect(ast.Children(cond)); !reflect.DeepEqual(got, []ast.Node{x, y, z}) {
		t.Fatal(got)
	}
	binding := ast.NewIdent("err")
	body := &ast.BlockStmt{}
	handled := &ast.ErrorExpr{X: x, Err: binding, Body: body}
	if got := slices.Collect(ast.Children(handled)); !reflect.DeepEqual(got, []ast.Node{x, binding, body}) {
		t.Fatal(got)
	}
	for _, test := range []struct {
		n        ast.Node
		children []ast.Node
	}{
		{&ast.ErrorExpr{X: x, Err: binding, Context: z}, []ast.Node{x, binding, z}},
		{&ast.LambdaExpr{Params: []*ast.Ident{x, y}, Body: z}, []ast.Node{x, y, z}},
		{&ast.LambdaExpr{Params: []*ast.Ident{x}, Block: body}, []ast.Node{x, body}},
		{&ast.NilGuardExpr{X: x}, []ast.Node{x}},
		{&ast.SafeNavExpr{X: x}, []ast.Node{x}},
	} {
		if got := slices.Collect(ast.Children(test.n)); !reflect.DeepEqual(got, test.children) {
			t.Fatal(got)
		}
	}
	// Ident.Obj is a semantic backlink, not a child.
	x.Obj = &ast.Object{Decl: cond}
	if got := slices.Collect(ast.Children(x)); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestChildrenIncomplete(t *testing.T) {
	for _, src := range []string{"package p; var x = if c { a } else {", "package p; func f() { x := read() or err {", "package p; func f() error { x := read() or err =>"} {
		f, err := parser.ParseFile(token.NewFileSet(), "broken.go", src, parser.AllErrors)
		if err == nil {
			t.Fatal("expected syntax error")
		}
		var walk func(ast.Node)
		walk = func(n ast.Node) {
			for child := range ast.Children(n) {
				walk(child)
			}
		}
		walk(f)
	}
}
