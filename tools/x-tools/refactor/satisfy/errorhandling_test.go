// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package satisfy

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestErrorHandling(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", `package p
 type I interface{ M() }
 type T struct{}
 func (T) M() {}
 func sink(I) {}
 func pair() (int, string, error) { return 1, "", nil }
 func read() (int, error) { return 1, nil }
 func save() error { return nil }
 func f() error {
  a, b := pair()!
  _, _ = a, b
  c := read() or err { sink(T{}); return err }
  _ = c
  save()!
  save() or err { sink(T{}) }
  return nil
 }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, err := new(types.Config).Check("p", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatal(err)
	}
	var finder Finder
	finder.Find(info, []*ast.File{f})
	constraint := Constraint{LHS: pkg.Scope().Lookup("I").Type(), RHS: pkg.Scope().Lookup("T").Type()}
	if !finder.Result[constraint] {
		t.Fatal("interface assignment inside error handler was not visited")
	}
}

// A failed Result propagated into a function returning error converts its
// payload to error, so a concrete error type satisfies error there.
func TestResultToErrorPropagation(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", `package p
 type E struct{}
 func (E) Error() string { return "" }
 func read() Result[int, E] { return .Ok(1) }
 func f() (int, error) {
  n := read()!
  return n, nil
 }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, err := new(types.Config).Check("p", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatal(err)
	}
	var finder Finder
	finder.Find(info, []*ast.File{f})
	constraint := Constraint{LHS: types.Universe.Lookup("error").Type(), RHS: pkg.Scope().Lookup("E").Type()}
	if !finder.Result[constraint] {
		t.Fatalf("Result payload converted to error was not recorded: %v", finder.Result)
	}
}
