package unused

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// Keep the CLI's U1000 analyzer precise on Gon syntax, rather than merely
// checking that structural traversal does not panic.
func TestGonUnused(t *testing.T) {
	// A named pointer receiver makes safe navigation exercise a field/method
	// reference while still keeping the fixture independent of std imports.
	const source = `package p
const live = 7
const dead = 9
type choice enum { default Empty; Number(int); Record { Value int } }
type maybe = int?
type pointer struct { Value int }
func (p *pointer) Compare(n int) int { return p.Value + n }
func keep(n int) int { return n + live }
func unused() int { return dead }
func load(n int) (int, error) { return n, nil }
func Run(input maybe, ptr *pointer, fail bool) (int, error) {
	var callback func(int) int = (n) => keep(n)
	n := load(callback(1)) or err => err
	if input is value? && value > 0 { n += value }
	n += ptr?.Compare(1) ?? 0
	n += if fail { keep(2) } else { 3 }
	v := choice.Record{Value: n}
	switch v {
	case choice.Empty, choice.Number(_) => { n++ }
	case choice.Record{Value: value} => { n += value }
	}
	return switch input { case nil => n; case value? => value }, nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "gon.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope), Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, err := new(types.Config).Check("p", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	g := newGraph(fset, []*ast.File{file}, pkg, info, nil, nil, DefaultOptions)
	g.entry()
	result := (&SerializedGraph{nodes: g.nodes}).Results()
	unused := make(map[string]bool)
	for _, obj := range result.Unused {
		unused[obj.ShortName] = true
	}
	if len(unused) != 2 || !unused["unused"] || !unused["dead"] {
		t.Fatalf("unused declarations = %v, want unused and dead; full result: %+v", unused, result)
	}
}
