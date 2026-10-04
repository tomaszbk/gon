package astutil_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"golang.org/x/tools/go/ast/astutil"
)

func TestGonAlternativesReplacement(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", `package p
type E enum { default Empty; Value(int); Record { Number int } }
func f(e E) int { return switch e { case E.Empty => 0; case E.Value(n) if n>0 => n; case E.Value(_) => 0; case E.Record{Number:n,...} => n } }
func g(e E) { switch e { case _ => { f(first:e,second:e) } }; _=option?; var o int? = (int)(1); o = nil }
`, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	original := map[ast.Node]bool{}
	for node := range ast.Preorder(f) {
		if node != f {
			original[node] = true
		}
	}
	count := 0
	result := astutil.Apply(f, nil, func(cursor *astutil.Cursor) bool {
		if cursor.Node() == nil || cursor.Node() == f {
			return true
		}
		value := reflect.ValueOf(cursor.Node())
		copy := reflect.New(value.Type().Elem())
		copy.Elem().Set(value.Elem())
		cursor.Replace(copy.Interface().(ast.Node))
		count++
		return true
	})
	if count != len(original) {
		t.Fatalf("replaced %d of %d nodes", count, len(original))
	}
	for node := range ast.Preorder(result) {
		if original[node] {
			t.Fatalf("mutable child slot missed %T", node)
		}
	}
}
