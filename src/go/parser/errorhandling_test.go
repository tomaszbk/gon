package parser

import (
	"go/ast"
	"go/token"
	"testing"
)

func TestErrorHandling(t *testing.T) {
	src := `package p
func f() error {
 x := read()! // propagation
 y := (read()) or err { return err }
 save() or _ {}
 _ = x + y
 return nil
}`
	f, err := ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var exprs []*ast.ErrorExpr
	ast.Inspect(f, func(n ast.Node) bool {
		if e, ok := n.(*ast.ErrorExpr); ok {
			exprs = append(exprs, e)
			if e.Pos() >= e.End() {
				t.Errorf("invalid positions: %v..%v", e.Pos(), e.End())
			}
		}
		return true
	})
	if len(exprs) != 3 || exprs[0].Body != nil || exprs[1].Err.Name != "err" || exprs[2].Err.Name != "_" {
		t.Fatalf("unexpected error expressions: %#v", exprs)
	}
	binding := exprs[1].Err.Obj
	use := exprs[1].Body.List[0].(*ast.ReturnStmt).Results[0].(*ast.Ident)
	if binding == nil || use.Obj != binding || binding.Pos() != exprs[1].Err.Pos() {
		t.Fatal("handler binding was not resolved in its local scope")
	}
}

func TestErrorHandlingLegacySyntax(t *testing.T) {
	for _, src := range []string{
		`package p; type or bool; func f(x or) or { return !x }`,
		`package p; type or interface{ ~int }; type G[P or] struct{ Value P }`,
		`package p; type or[T any] interface{ ~int }; type G[P or[int]] struct{ Value P }`,
		`package p; type or interface{ ~int }; type G[P or | ~string] struct{ Value P }`,
		`package p; type or interface{ ~bool }; func f[T or](x T) T { return ! x }`,
		`package p; func f() { or := true; _ = !or; _ = or != false }`,
		`package p; func f() { x := call()!; _ = x }`,
		"package p; func f() { call()! /* a\nb */\n}",
	} {
		if _, err := ParseFile(token.NewFileSet(), "p.go", src, 0); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
	for _, gap := range []string{"\n", " // comment\n", " /* comment */\n", " /* line\nbreak */"} {
		src := "package p; var x = !" + gap + "true"
		if _, err := ParseFile(token.NewFileSet(), "p.go", src, 0); err == nil {
			t.Errorf("accepted removed multiline prefix negation: %q", src)
		}
	}
}

func TestGenericOrConstraintErrors(t *testing.T) {
	for _, src := range []string{
		`package p; type G[P or]`,
		`package p; type G[P or,]`,
		`package p; type G[P or |] struct{}`,
		`package p; type G[P or[int] struct{}`,
	} {
		if _, err := ParseFile(token.NewFileSet(), "bad.go", src, AllErrors); err == nil {
			t.Errorf("accepted malformed generic constraint: %s", src)
		}
	}
}
