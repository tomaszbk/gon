package cfg

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestGonFeatureControlFlow(t *testing.T) {
	for _, test := range []struct {
		body     string
		noReturn bool
	}{
		{`var f func() error = () => { g()!; return nil }; panic(0)`, true},
		{`var f func() error = () => g()!; panic(0)`, true},
		{`_ = p?.M(g()!); panic(0)`, false},
		{`_ = p?.M(g() or err { panic(err) }); panic(0)`, true},
		{`_ = p ?? g()!; panic(0)`, false},
		{`p ??= g()!; panic(0)`, false},
		{`_ = **p ?? g()!; panic(0)`, false},
	} {
		t.Run(test.body, func(t *testing.T) {
			g := newCondCFG(t, test.body)
			if g.NoReturn() != test.noReturn {
				t.Fatalf("NoReturn = %v, want %v\n%s", g.NoReturn(), test.noReturn, g.Format(token.NewFileSet()))
			}
		})
	}
}

func TestGonNilAbsentPaths(t *testing.T) {
	g := newCondCFG(t, `_ = p?.M(arg()!) ?? fallback()!`)
	entry := g.Blocks[0]
	if len(entry.Succs) != 2 || entry.Succs[1].Kind != KindNilFallback {
		t.Fatalf("nil receiver must go directly to fallback\n%s", g.Format(token.NewFileSet()))
	}
	present, absent := entry.Succs[0], entry.Succs[1]
	if len(present.Succs) != 2 || present.Succs[0].Kind != KindErrorHandler {
		t.Fatal("missing argument propagation")
	}
	if reaches(absent, present.Succs[0]) {
		t.Fatal("nil receiver evaluates guarded argument")
	}
	if !reaches(present, absent) {
		t.Fatal("nil result of a present call cannot reach fallback")
	}

	g = newCondCFG(t, `_ = **p ?? fallback()`)
	entry = g.Blocks[0]
	if len(entry.Succs) != 2 || entry.Succs[1].Kind != KindNilFallback ||
		len(entry.Succs[0].Succs) != 2 || entry.Succs[0].Succs[1] != entry.Succs[1] {
		t.Fatalf("both guarded dereferences must share the fallback\n%s", g.Format(token.NewFileSet()))
	}
}

func TestGonOptionalTypeControlFlow(t *testing.T) {
	const src = `package p
func identity[T any](value T) T { return value }
func f() { var value int?; _ = identity[int?](value); panic(0) }
func g() int? { return (int)(1) }
func h(value int?) int? { var x int? = (int)(value?); _ = x; panic(0) }
`
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, "p.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, OptionalConversions: map[ast.Expr]types.Type{}}
	if _, err := new(types.Config).Check("p", fs, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls[1:] {
		fn := decl.(*ast.FuncDecl)
		graph := NewWithTypes(fn.Body, func(call *ast.CallExpr) bool { return false }, info)
		propagates := false
		for _, block := range graph.Blocks {
			if block.Kind == KindOptionAbsent {
				propagates = true
			}
		}
		if propagates != (fn.Name.Name == "h") {
			t.Fatalf("%s: unexpected propagation %v\n%s", fn.Name.Name, propagates, graph.Format(fs))
		}
	}
}
