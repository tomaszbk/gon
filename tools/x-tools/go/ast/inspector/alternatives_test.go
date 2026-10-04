package inspector

import (
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/ast/edge"
	"slices"
	"testing"
)

func TestGonAlternativesInventory(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", `package p
 type Choice enum { default Empty; Value(int); Record { Number int } }
 func f(c Choice) string { return switch c {
 case Choice.Empty => "empty"
 case Choice.Value(n) if n > 0 => "positive"
 case Choice.Value(_) => "other"
 case Choice.Record{Number: n, ...} => "record"
 } }
 func g(c Choice) { switch c { case _ => { f(second: c, first: c) } } }
 func h() { _ = opt?; var o int? = (int)(1); o = nil; var result Result[int,string] = .Ok(3); _ = result; switch o { case nil => {}; case value? => {_ = value} } }
 `, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	in := New([]*ast.File{f})
	var want, got []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n != nil {
			want = append(want, n)
		}
		return true
	})
	in.Preorder(nil, func(n ast.Node) { got = append(got, n) })
	if !slices.Equal(want, got) {
		t.Fatal("inspector order differs from ast.Walk")
	}
	kinds := []ast.Node{(*ast.EnumType)(nil), (*ast.EnumVariant)(nil), (*ast.MatchExpr)(nil), (*ast.MatchStmt)(nil), (*ast.MatchArm)(nil), (*ast.MatchPattern)(nil), (*ast.MatchField)(nil), (*ast.OptionalExpr)(nil), (*ast.ContextualVariantExpr)(nil)}
	var masks []nodeMask
	for _, kind := range kinds {
		mask := typeOf(kind)
		if mask == (nodeMask{}) {
			t.Fatalf("missing mask for %T", kind)
		}
		for _, prev := range masks {
			if mask.intersects(prev) {
				t.Fatalf("aliased mask for %T", kind)
			}
		}
		masks = append(masks, mask)
		visits := 0
		in.Preorder([]ast.Node{kind}, func(n ast.Node) {
			visits++
			if typeOf(n) != mask {
				t.Fatalf("filter for %T returned %T", kind, n)
			}
		})
		if visits == 0 {
			t.Fatalf("missing %T from traversal", kind)
		}
	}
	for cur := range in.Root().Preorder(kinds...) {
		for child := range cur.Children() {
			k, index := child.ParentEdgeKind(), child.ParentEdgeIndex()
			if k.Get(cur.Node(), index) != child.Node() {
				t.Fatalf("bad %s[%d]", k, index)
			}
		}
	}
	for cur := range in.Root().Preorder((*ast.CallExpr)(nil)) {
		call := cur.Node().(*ast.CallExpr)
		for i, name := range call.ArgNames {
			if name != nil && cur.ChildAt(edge.CallExpr_ArgNames, i).Node() != name {
				t.Fatal("missing named-label cursor")
			}
		}
	}
}
