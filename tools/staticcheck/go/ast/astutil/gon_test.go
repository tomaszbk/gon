package astutil_test

import (
	"go/ast"
	"go/parser"
	"honnef.co/go/tools/go/ast/astutil"
	"testing"
)

type unknownExpr struct{ ast.Expr }

func TestGonTransformSafety(t *testing.T) {
	for _, src := range []string{".Some(1)", ".None", "int?", "if c { 1 } else { 2 }", "f()!", "f() or err { panic(err) }", "(x) => x", "() => { f() }", "p?.Field", "f?()", "Option[int].Some(1)?", "switch e { case E.A(x) => x; default => 0 }", "f(right: 1, left: 2)"} {
		a, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		b, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := astutil.CopyExpr(a); ok {
			t.Fatalf("unexpected unsupported copy: %s", src)
		}
		if astutil.Equal(a, b) {
			t.Fatalf("unproven equality: %s", src)
		}
	}
	if _, ok := astutil.CopyExpr(&unknownExpr{}); ok {
		t.Fatal("unknown copy accepted")
	}
	if astutil.Equal(&unknownExpr{}, &unknownExpr{}) {
		t.Fatal("unknown equality accepted")
	}
	for _, src := range []string{"x + 1", "p ?? q"} {
		a, _ := parser.ParseExpr(src)
		b, ok := astutil.CopyExpr(a)
		if !ok || !astutil.Equal(a, b) || a.(*ast.BinaryExpr).Op != b.(*ast.BinaryExpr).Op {
			t.Fatalf("binary copy/equality changed %s", src)
		}
	}
}
