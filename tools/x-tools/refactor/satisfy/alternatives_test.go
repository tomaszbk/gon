package satisfy_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/refactor/satisfy"
)

func TestGonAlternatives(t *testing.T) {
	const source = `package p
type I interface { M() }
type P int
func (P) M() {}
type E enum { default Empty; Record { Value I }; Positional(I) }
func matched(e E) I { return switch e { case E.Empty => P(0); case E.Record{Value: v} => v; case E.Positional(v) => v } }
func use() { e:=E.Record{Value:P(1)}; switch e { case E.Empty => {}; case E.Record{Value:v} => { v.M() }; case E.Positional(v) => { v.M() } }; _=matched(e) }
func errorType(r Result[int,P]) Result[int,I] { n:=r!; return Result[int,I].Ok(n) }
func optional(o I?) I { return o ?? P(1) }
func assign(o I?) { o ??= P(2) }
func construct() I? { return (I)(P(3)) }
func implicit() I? { return P(4) }
func implicitCond(c bool) I? { return if c { P(4) } else { P(5) } }
func implicitMatch(c bool) I? { return switch c { case true => P(6); case false => nil } }
func result() Result[I,error] { return .Ok(P(5)) }
`
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "p.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{OptionalConversions: map[ast.Expr]types.Type{}, Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	p, err := new(types.Config).Check("p", fs, []*ast.File{f}, info)
	if err != nil {
		t.Fatal(err)
	}
	var finder satisfy.Finder
	finder.Find(info, []*ast.File{f})
	want := satisfy.Constraint{LHS: p.Scope().Lookup("I").Type(), RHS: p.Scope().Lookup("P").Type()}
	if !finder.Result[want] {
		t.Fatalf("missing payload/match/coalesce/propagation constraint %v in %v", want, finder.Result)
	}
}
