package typesinternal_test

import (
	"go/ast"
	"go/parser"
	"go/types"
	"golang.org/x/tools/internal/typesinternal"
	"testing"
)

func TestGonNoEffects(t *testing.T) {
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	for _, tt := range []struct {
		src  string
		pure bool
	}{
		{"if c { 1 } else { 2 }", true},
		{"if c { <-ch } else { 2 }", false},
		{"if c { 1 } else { f() }", false},
		{"f()!", false},
		{"(int?)(1)?", false},
		{"switch v { case E.A => 1; default => f() }", false},
		{"f() or err { panic(err) }", false},
		{"func() { f() }", true},
	} {
		expr, err := parser.ParseExpr(tt.src)
		if err != nil {
			t.Fatal(err)
		}
		if got := typesinternal.NoEffects(info, expr); got != tt.pure {
			t.Errorf("%s: NoEffects=%v, want %v", tt.src, got, tt.pure)
		}
	}
}
