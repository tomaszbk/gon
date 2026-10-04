// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser_test

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestAlternativesSyntax(t *testing.T) {
	const src = `package p
// Payment records the selected alternative.
type Payment[T any] enum {
    default Pending
    Rejected(string, T)
    Paid { Receipt string; Amount int64 }
}
func f(p Payment[int]) string {
    label := switch p {
    case Payment[int].Pending => "pending"
    case Payment[int].Rejected(reason, _) if reason != "" => reason
    case Payment[int].Rejected(_, _) => "rejected"
    case Payment[int].Paid{Receipt: receipt, ...} => receipt
    }
    switch p {
    case Payment[int].Pending => { println("pending") }
    default => { println(label) }
    }
    return label
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "enum.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	enum := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.EnumType)
	if len(enum.Variants) != 3 || !enum.Variants[0].Default.IsValid() || enum.Variants[1].Record || !enum.Variants[2].Record {
		t.Fatalf("bad enum: %#v", enum)
	}
	if got := len(enum.Variants[1].Payload.List); got != 2 {
		t.Fatalf("payload count: %d", got)
	}
	body := f.Decls[1].(*ast.FuncDecl).Body
	match := body.List[0].(*ast.AssignStmt).Rhs[0].(*ast.MatchExpr)
	if len(match.Arms) != 4 || match.Arms[1].Guard == nil || !match.Arms[3].Pattern.Rest.IsValid() {
		t.Fatalf("bad match: %#v", match)
	}
	if _, ok := body.List[1].(*ast.MatchStmt); !ok {
		t.Fatalf("statement: %T", body.List[1])
	}
	var out bytes.Buffer
	if err := format.Node(&out, fset, f); err != nil {
		t.Fatal(err)
	}
	formatted := out.Bytes()
	if _, err := parser.ParseFile(token.NewFileSet(), "formatted.go", formatted, parser.ParseComments); err != nil {
		t.Fatalf("formatted source: %s\n%v", formatted, err)
	}
	again, err := format.Source(formatted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(formatted, again) {
		t.Fatalf("format not stable:\n%s\nthen:\n%s", formatted, again)
	}
}

func TestAlternativesNestedPatterns(t *testing.T) {
	x, err := parser.ParseExpr(`switch x {
case Option[Option[int]].Some(Option[int].Some(value)) => value
case Option[Option[int]].Some(Option[int].None) => 0
case Option[Option[int]].None => -1
}`)
	if err != nil {
		t.Fatal(err)
	}
	m := x.(*ast.MatchExpr)
	if len(m.Arms[0].Pattern.Args[0].Args) != 1 {
		t.Fatalf("nested pattern: %#v", m.Arms[0].Pattern)
	}
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for c := range ast.Children(n) {
			walk(c)
		}
	}
	walk(m)
}

func TestAlternativesLegacySyntax(t *testing.T) {
	const src = `package p
type enum int
type X enum
type Y = enum
type Z enum
func f(x int) {
    enum := x
    switch enum { case f1()+f2(): println(enum); default: }
    switch x := enum; x { case 1, 2: println(x) }
    switch interface{}(x).(type) { case int: }
}
`
	f, err := parser.ParseFile(token.NewFileSet(), "legacy.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls[:4] {
		if _, ok := d.(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.Ident); !ok {
			t.Fatalf("enum unexpectedly contextual: %#v", d)
		}
	}
}

func TestAlternativesInvalidSyntax(t *testing.T) {
	for _, src := range []string{
		`package p; type E enum; { default V }`,
		`package p; type E = enum { default V }`,
		`package p; type E enum { default V(x int) }`,
		`package p; type E enum { default V { Embedded } }`,
		`package p; type E enum { default V { *Embedded } }`,
		`package p; type E enum { default V { X int "tag" } }`,
		`package p; var x = switch y { case E.V => 1; default: 2 }`,
		`package p; func f() { switch y { case E.V => 1 } }`,
		`package p; var x = switch y { case E.V{X: x, ..., Y: y} => x }`,
		`package p; var x = switch y { case E.V{X} => 1 }`,
		`package p; var x = switch y { case f()+g() => 1 }`,
		`package p; var x = switch init(); y { default => 1 }`,
	} {
		t.Run(strings.ReplaceAll(src, " ", "_"), func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "bad.go", src, parser.AllErrors)
			if err == nil {
				t.Fatalf("accepted invalid syntax: %s", src)
			}
			// Malformed input must remain traversable for editor consumers.
			if f != nil {
				ast.Inspect(f, func(ast.Node) bool { return true })
			}
		})
	}
}

func TestOptionSyntax(t *testing.T) {
	for _, src := range []string{`x?`, `read()?`, `(x?).Name`, `p? .Field`, `f? ()`} {
		x, err := parser.ParseExpr(src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		var found bool
		ast.Inspect(x, func(n ast.Node) bool {
			if _, ok := n.(*ast.OptionalExpr); ok {
				found = true
			}
			return true
		})
		if !found {
			t.Errorf("%s: missing propagation node", src)
		}
	}
	for _, src := range []string{`package p; func f() { x := read()?\n_ = x }`, `package p; func f() { x := read()? /* end */\n_ = x }`} {
		src = strings.ReplaceAll(src, `\n`, "\n")
		if _, err := parser.ParseFile(token.NewFileSet(), "option.go", src, 0); err != nil {
			t.Fatal(err)
		}
	}
	for _, src := range []string{`p?.Field`, `f?(1)`, `p ?? 0`} {
		x, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(x, func(n ast.Node) bool {
			if _, ok := n.(*ast.OptionalExpr); ok {
				t.Errorf("longest match changed: %s", src)
			}
			return true
		})
	}
}

func TestMatchObjectScopes(t *testing.T) {
	const src = `package p; type E enum { default A; B(int) }; func f(e E) int { return switch e { case E.B(n) if n > 0 => n; default => 0 } }`
	f, err := parser.ParseFile(token.NewFileSet(), "scope.go", src, parser.DeclarationErrors)
	if err != nil {
		t.Fatal(err)
	}
	m := f.Decls[1].(*ast.FuncDecl).Body.List[0].(*ast.ReturnStmt).Results[0].(*ast.MatchExpr)
	binding := m.Arms[0].Pattern.Args[0].Value.(*ast.Ident)
	use := m.Arms[0].Value.(*ast.Ident)
	if binding.Obj == nil || use.Obj != binding.Obj || binding.Obj.Pos() != binding.Pos() {
		t.Fatal("pattern binding was not resolved in its arm")
	}
	for _, id := range f.Unresolved {
		if id.Name == "n" || id.Name == "A" || id.Name == "B" {
			t.Errorf("unexpected unresolved pattern/declaration %s", id.Name)
		}
	}
}

func TestAlternativesGroupedRecord(t *testing.T) {
	const source = `package p; type E enum { default V { X, Y int } }`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "grouped.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	field := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.EnumType).Variants[0].Payload.List[0]
	if len(field.Names) != 2 || field.Names[0].Name != "X" || field.Names[1].Name != "Y" || field.Type.(*ast.Ident).Name != "int" {
		t.Fatalf("grouped payload field: %#v", field)
	}
	var out bytes.Buffer
	if err := format.Node(&out, fset, file); err != nil {
		t.Fatal(err)
	}
	again, err := format.Source(out.Bytes())
	if err != nil || !bytes.Equal(again, out.Bytes()) || !strings.Contains(out.String(), "X, Y int") {
		t.Fatalf("grouped payload print not preserved: %v\n%s\n%s", err, out.Bytes(), again)
	}
}

func TestSimplifiedAlternativesSyntax(t *testing.T) {
	const src = `package p
 type Ptr = *int?
 type OptionalPtr = (*int)?
 type OptionalSlice = ([]int)?
 type Nested = (int?)?
 type F func(int?, (*int)?, (int?)?) (int?, Result[int?, error])
 type OptionalReturns func(*int) (*int)?
 type SliceReturns func([]int) ([]int)?
 type NestedReturns func(int?) (int?)?
 func groupedPointer(value *int) (*int)? { return (*int)(value) }
 func groupedSlice(value []int) ([]int)? { return value }
 func groupedNested(value int?) (int?)? { return value }
 func legacyTuple() (first *int, second []int) { return nil, nil }
 var a int? = (int)(3)
 var b int? = nil
 func f(x int?, y (int?)?, z []int?) (value Result[int?, error]) {
     var p (*int)? = (*int)(nil)
     _ = p
     return .Ok(if true { (int)(x ?? 0) } else { nil })
 }
 func g() Result[int, error] { return .Err(nil) }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "simplified.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var optional, variants int
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.OptionalExpr:
			optional++
		case *ast.ContextualVariantExpr:
			variants++
			if node.Pos() != node.Dot || node.End() <= node.Pos() {
				t.Errorf("invalid contextual positions: %#v", node)
			}
			var children []ast.Node
			for child := range ast.Children(node) {
				children = append(children, child)
			}
			if len(children) != len(node.Args)+1 || children[0] != node.Name {
				t.Errorf("invalid contextual children: %#v", children)
			}
			if node.Name.Obj != nil {
				t.Errorf("contextual variant resolved as ordinary identifier: %s", node.Name.Name)
			}
		}
		return true
	})
	if optional < 10 || variants != 2 {
		t.Fatalf("unexpected nodes: %d option, %d contextual", optional, variants)
	}
	ptr := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StarExpr)
	if _, ok := ptr.X.(*ast.OptionalExpr); !ok {
		t.Fatalf("*int? has wrong association: %T", ptr.X)
	}
	optionalPtr := file.Decls[1].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.OptionalExpr)
	if _, ok := optionalPtr.X.(*ast.ParenExpr); !ok {
		t.Fatalf("(*int)? lost grouping: %T", optionalPtr.X)
	}
	var out bytes.Buffer
	if err := format.Node(&out, fset, file); err != nil {
		t.Fatal(err)
	}
	formatted := out.Bytes()
	if _, err := parser.ParseFile(token.NewFileSet(), "formatted.go", formatted, 0); err != nil {
		t.Fatalf("formatted syntax failed: %v\n%s", err, formatted)
	}
	again, err := format.Source(formatted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, formatted) {
		t.Fatalf("unstable format:\n%s\n%s", formatted, again)
	}
}

func TestSimplifiedAlternativesSyntaxErrors(t *testing.T) {
	for _, source := range []string{
		`package p; var x int??`,
		`package p; func f() (one int, two int)? { return nil }`,
		`package p; func f() (number int)? { return nil }`,
		`package p; func f() ()? { return nil }`,
		`package p; var x = .`,
		`package p; var x = .Some(value: 1)`,
		`package p; var x = .Some(values...)`,
		`package p; var x = switch y { case .Some(v) => v; default => 0 }`,
	} {
		if _, err := parser.ParseFile(token.NewFileSet(), "invalid.go", source, parser.AllErrors); err == nil {
			t.Errorf("accepted invalid syntax: %s", source)
		}
	}
}

func TestSimplifiedAlternativesMultiline(t *testing.T) {
	const source = `package p
func f() Result[int, error] {
 return .Ok(
 // payload
 42,
 )
}
`
	formatted, err := format.Source([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	again, err := format.Source(formatted)
	if err != nil {
		t.Fatalf("formatted contextual construction failed: %v\n%s", err, formatted)
	}
	if !bytes.Equal(formatted, again) {
		t.Fatalf("unstable formatting:\n%s\n%s", formatted, again)
	}
	if !bytes.Contains(formatted, []byte("// payload")) {
		t.Fatalf("comment lost:\n%s", formatted)
	}
}
