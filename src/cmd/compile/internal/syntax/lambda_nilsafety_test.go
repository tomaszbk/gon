package syntax

import (
	"strings"
	"testing"
)

func parseNewSyntax(t *testing.T, src string) *File {
	t.Helper()
	f, err := Parse(NewFileBase("features.go"), strings.NewReader("package p; "+src), nil, nil, CheckBranches)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestLambdaSyntax(t *testing.T) {
	valid := []string{
		`var f = () => 1`,
		`var f = (x) => x + 1`,
		`var f = (x,y,) => { return x+y }`,
		`var f = (or,fn,_) => if or { fn } else { 0 }`,
		"var f = (x,\n y,) =>\n x+y",
		`var f = (p) => load(p)!`,
		`var f = (p) => load(p) or err { return nil }`,
		`var f = (x) => (y) => x+y`,
		`func f(){if apply((x)=>{return T{x}}) {}}`,
	}
	for _, src := range valid {
		t.Run(src, func(t *testing.T) {
			f := parseNewSyntax(t, src)
			var text strings.Builder
			if _, err := Fprint(&text, f, 0); err != nil {
				t.Fatal(err)
			}
			if _, err := Parse(NewFileBase("roundtrip.go"), strings.NewReader(text.String()), nil, nil, CheckBranches); err != nil {
				t.Fatalf("roundtrip %q: %v", text.String(), err)
			}
			Inspect(f, func(n Node) bool {
				if e, ok := n.(*LambdaExpr); ok {
					if (e.Body == nil) == (e.Block == nil) {
						t.Error("lambda must have exactly one body")
					}
					if !e.Arrow.IsKnown() || !e.Rparen.IsKnown() || e.Arrow.Cmp(e.Rparen) <= 0 {
						t.Error("lambda punctuation positions")
					}
					if StartPos(e).Cmp(EndPos(e)) >= 0 {
						t.Error("lambda extent")
					}
				}
				return true
			})
		})
	}
	for _, src := range []string{`var f = x => x`, `var f = (x int) => x`, `var f = (x,...y) => x`, `var f = (a+b) => 1`, `var f = (a,b)`, "var f = (x)\n=> x", `func f(){ for { var cb = () => { break }; _ = cb } }`} {
		errors := 0
		if _, err := Parse(NewFileBase("bad.go"), strings.NewReader("package p; "+src), func(error) { errors++ }, nil, CheckBranches); err == nil && errors == 0 {
			t.Errorf("accepted invalid lambda %q", src)
		}
	}
	// The legacy operand alternatives and unnamed function parameters remain valid.
	parseNewSyntax(t, `var a = (x); var b = (*T)(x); var c = func(int,int)bool{return true}; func f(){(a),b=1,2}`)
}

func TestNilSafetySyntax(t *testing.T) {
	for _, src := range []string{
		`var x = p?.A.B(f())[0]`, `var x = p?.A?.B`, `var x = (p?.A).B`,
		`var x = f?(p?.A)`, `var x = p?.Load()!?.Next`, `var x = p?.Load() or err {return nil}`,
		`var x = a ?? b ?? c`, `var x = (p?.N ?? 0) + 1`, `var x = *p ?? 0`,
		`var x = if yes {p} else {q}?.A`, `var x = p ?? () => 1`,
		"var x = p?.\nA ??\nother?(\n)", `func f(){m[key()] ??= value()}`,
	} {
		t.Run(src, func(t *testing.T) {
			f := parseNewSyntax(t, src)
			var text strings.Builder
			Fprint(&text, f, 0)
			if _, err := Parse(NewFileBase("roundtrip.go"), strings.NewReader(text.String()), nil, nil, CheckBranches); err != nil {
				t.Fatalf("roundtrip %q: %v", text.String(), err)
			}
			Inspect(f, func(n Node) bool {
				switch e := n.(type) {
				case *NilGuardExpr:
					if StartPos(e).Cmp(e.Question) >= 0 {
						t.Error("guard extent")
					}
				case *SafeNavExpr:
					if StartPos(e) != StartPos(e.X) {
						t.Error("chain start position")
					}
				}
				return true
			})
		})
	}
	f := parseNewSyntax(t, `var x = a ?? b ?? c`)
	op := f.DeclList[0].(*VarDecl).Values.(*Operation)
	if op.Op != Coalesce {
		t.Fatal("missing coalescing operation")
	}
	if right, ok := op.Y.(*Operation); !ok || right.Op != Coalesce {
		t.Fatal("coalescing must group right")
	}
	for _, src := range []string{`var x = p?.(T)`, `var x = p?.5`, `var x = a ?? b + c`, `var x = a || b ?? c`, "var x = p\n?.A", "var x = p\n?? q"} {
		if _, err := Parse(NewFileBase("bad.go"), strings.NewReader("package p; "+src), func(error) {}, nil, 0); err == nil {
			t.Errorf("accepted invalid nil syntax %q", src)
		}
	}
	parseNewSyntax(t, `func f() { _ = p? .A; _ = p?[0] }`)
	parseNewSyntax(t, `var or = "?. ?? ??= =>"; var n = .5; var b = !x; var c = ! x; var d = x!=y; var a = f(xs...)`)
}

func TestLambdaNilSafetyTokens(t *testing.T) {
	var s scanner
	s.init(strings.NewReader("=>\n?.\n?(\n??\n??=\n!\n! x\n!=\n"), errh, 0)
	want := []token{_FatArrow, _SafeDot, _SafeLparen, _Operator, _AssignOp, _Operator, _Semi, _Operator, _Name, _Semi, _Operator, _EOF}
	for i, tok := range want {
		s.next()
		if s.tok != tok {
			t.Fatalf("token %d: got %v, want %v", i, s.tok, tok)
		}
		if (i == 3 || i == 4) && s.op != Coalesce {
			t.Error("coalescing operator")
		}
	}
}
