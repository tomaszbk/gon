package code_test

import (
	"go/ast"
	"go/parser"
	"honnef.co/go/tools/analysis/code"
	"testing"
)

type unknownExpr struct{ ast.Expr }

func TestGonEffects(t *testing.T) {
	for _, tt := range []struct {
		src     string
		effects bool
	}{
		{"if c { 1 } else { 2 }", false},
		{".Some(1)", false},
		{".Some(f())", true},
		{".Ok(f()!)", true},
		{".None", false},
		{"if c { f() } else { 2 }", true},
		{"if c { 1 } else { f() }", true},
		{"f()!", true},
		{"f() or err { panic(err) }", true},
		{"func() { f() }", false},
		{"() => f()", false},
		{"() => { f()! }", false},
		{"p?.Field", true},
		{"f?(g())", true},
		{"p ?? f()", true},
	} {
		e, err := parser.ParseExpr(tt.src)
		if err != nil {
			t.Fatal(err)
		}
		if got := code.MayHaveSideEffects(nil, e, nil); got != tt.effects {
			t.Errorf("%s: got %v", tt.src, got)
		}
	}
	if !code.MayHaveSideEffects(nil, &unknownExpr{}, nil) {
		t.Fatal("unknown expressions must be conservative")
	}
	if !code.MayHaveSideEffects(nil, &ast.BadExpr{}, nil) {
		t.Fatal("invalid expressions must be conservative")
	}
}
