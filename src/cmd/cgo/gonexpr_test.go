//go:build !compiler_bootstrap

package main

import (
	"go/ast"
	"go/parser"
	"testing"
)

func TestGonErrorBoundary(t *testing.T) {
	for _, test := range []struct {
		src   string
		found bool
	}{
		{`p?.M(read()!)`, true}, {`p ?? read()!`, true},
		{`consume((x) => read(x)!)`, false}, {`consume(() => { read()! })`, false},
		{`consume(func() { read()! })`, false},
		{`consume((x) => read(x)!, read()!)`, true},
		{`consume(option?)`, true}, {`option? ?? fallback()`, true},
		{`consume(() => option?)`, false}, {`consume(func() { option? })`, false},
	} {
		e, err := parser.ParseExpr(test.src)
		if err != nil {
			t.Fatal(err)
		}
		if got := findErrorExpr(e) != nil; got != test.found {
			t.Errorf("%q: got %v", test.src, got)
		}
	}
}

func TestGonTargetType(t *testing.T) {
	for _, test := range []struct {
		src  string
		need bool
	}{
		{`() => nil`, true}, {`p?.Field`, true}, {`(p?.Field)`, true}, {`a ?? b`, true},
		{`(a ?? b)`, true}, {`a+b`, false}, {`func() {}`, false},
		{`.Some(C.int(3))`, true}, {`.Ok(C.f())`, true}, {`.None`, true},
	} {
		e, err := parser.ParseExpr(test.src)
		if err != nil {
			t.Fatal(err)
		}
		if got := needsGonTargetType(e); got != test.need {
			t.Errorf("%q: got %v", test.src, got)
		}
	}
}

func TestGonWalk(t *testing.T) {
	for _, src := range []string{`(x) => C.f(x)`, `() => { C.f() }`, `p?.Field ?? C.f()`, `f?(C.f())`, `C.f()?`, `.Ok(C.f())`, `.Some(C.f())`} {
		e, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		f := new(File)
		var refs int
		f.walk(e, ctxExpr, func(_ *File, n any, _ astContext) {
			if s, ok := n.(*ast.SelectorExpr); ok {
				if x, ok := s.X.(*ast.Ident); ok && x.Name == "C" {
					refs++
				}
			}
		})
		if refs != 1 {
			t.Errorf("%q: found %d C references", src, refs)
		}
	}
}
