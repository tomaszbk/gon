package parser

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

func TestLambdaSyntax(t *testing.T) {
	for _, src := range []string{
		`() => 42`, `(x) => x + 1`, `(x, y,) => x + y`, `(_, or) => or`,
		`(x) => { return x }`, `(x) => (y) => x + y`, `(x) => if x { f()! } else { g()! }`,
		"(x,\ny,\n) =>\nx + y", `(p) => load(p) or err { return nil }`,
	} {
		e, err := ParseExpr(src)
		if err != nil {
			t.Errorf("%q: %v", src, err)
			continue
		}
		l, ok := e.(*ast.LambdaExpr)
		if !ok {
			t.Fatalf("%q: got %T", src, e)
		}
		if (l.Body == nil) == (l.Block == nil) || l.Pos() >= l.End() {
			t.Fatalf("invalid lambda: %#v", l)
		}
	}
	e, err := ParseExpr(`1 + (x) => x + 2`)
	if err != nil {
		t.Fatal(err)
	}
	b := e.(*ast.BinaryExpr)
	l := b.Y.(*ast.LambdaExpr)
	if _, ok := l.Body.(*ast.BinaryExpr); !ok {
		t.Fatal("lambda body did not extend to the right")
	}
	f, err := ParseFile(token.NewFileSet(), "p.go", `package p; var x = (a, b) => { return a + b }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	l = f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.LambdaExpr)
	ret := l.Block.List[0].(*ast.ReturnStmt).Results[0].(*ast.BinaryExpr)
	for i, use := range []*ast.Ident{ret.X.(*ast.Ident), ret.Y.(*ast.Ident)} {
		if use.Obj != l.Params[i].Obj || use.Obj == nil || use.Obj.Pos() != l.Params[i].Pos() {
			t.Fatal("lambda parameter not resolved")
		}
	}
}

func TestLambdaInvalidSyntax(t *testing.T) {
	for _, src := range []string{`x => x`, `(x, y)`, `()`, `(x + 1) => x`, `(x int) => x`, `(x ...int) => x`, "(x)\n=> x", `(x) =>`, `(x) => (x, nil)`} {
		if _, err := ParseExpr(src); err == nil {
			t.Errorf("accepted %q", src)
		}
	}
}

func TestNullSafetySyntax(t *testing.T) {
	e, err := ParseExpr(`p?.Next?.M(a)[i].Field`)
	if err != nil {
		t.Fatal(err)
	}
	chain, ok := e.(*ast.SafeNavExpr)
	if !ok {
		t.Fatalf("got %T", e)
	}
	var guards int
	ast.Inspect(chain, func(n ast.Node) bool {
		if g, ok := n.(*ast.NilGuardExpr); ok {
			guards++
			if g.End() != g.Question+1 || g.Pos() >= g.End() {
				t.Fatal("invalid guard positions")
			}
		}
		return true
	})
	if guards != 2 {
		t.Fatalf("got %d guards", guards)
	}
	e, err = ParseExpr(`(p?.Next).Field`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := e.(*ast.SelectorExpr); !ok {
		t.Fatalf("parentheses failed to end chain: %T", e)
	}
	e, err = ParseExpr(`f?(g())`)
	if err != nil {
		t.Fatal(err)
	}
	call := e.(*ast.SafeNavExpr).X.(*ast.CallExpr)
	if call.Lparen != call.Fun.(*ast.NilGuardExpr).Question+1 {
		t.Fatal("safe-call parenthesis position")
	}
	e, err = ParseExpr(`a ?? b ?? c`)
	if err != nil {
		t.Fatal(err)
	}
	b := e.(*ast.BinaryExpr)
	if b.Op != token.COALESCE {
		t.Fatal(b.Op)
	}
	if right, ok := b.Y.(*ast.BinaryExpr); !ok || right.Op != token.COALESCE {
		t.Fatal("?? is not right associative")
	}
	for _, src := range []string{
		`f?()`, `p?.M(f?())!`, `p?.M() or err { return err }`, `*p?.Timeout ?? fallback`,
		`(a + b) ?? c`, `a ?? (b + c)`, `(a ?? b) + c`, `a + (b ?? c)`,
		`if c { a } else { b } ?? d`, `a ?? (x) => x + 1`,
	} {
		if _, err := ParseExpr(src); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
	f, err := ParseFile(token.NewFileSet(), "p.go", `package p; func f() { m[key()] ??= init() }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if f.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.AssignStmt).Tok != token.COALESCE_ASSIGN {
		t.Fatal("missing ??= assignment")
	}
}

func TestNullSafetyInvalidSyntax(t *testing.T) {
	for _, src := range []string{`p?.`, `p?.(T)`, `a ?? b + c`, `a + b ?? c`, `a ?? b || c`, `a || b ?? c`} {
		_, err := ParseExpr(src)
		if err == nil {
			t.Errorf("accepted %q", src)
		}
		if strings.Contains(src, "??") && err != nil && !strings.Contains(err.Error(), "cannot mix ??") {
			t.Errorf("unexpected mixed-operator diagnostic: %v", err)
		}
	}
}

func TestLambdaNullSafetyLegacySyntax(t *testing.T) {
	for _, src := range []string{
		`package p; func f() { (a), b = 1, 2; _ = (*T)(x); _ = (x + y)*z }`,
		`package p; var f = func(any, any) bool { return new(int) != nil }`,
		`package p; var x = "?. ?( ?? ??= =>" // ?. ?? =>`,
	} {
		if _, err := ParseFile(token.NewFileSet(), "p.go", src, 0); err != nil {
			t.Errorf("legacy %q: %v", src, err)
		}
	}
}
