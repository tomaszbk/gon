package parser_test

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"testing"
)

func TestNamedArguments(t *testing.T) {
	src := `package p; func f(a, b int) {}; func g(){ f(1, b: 2); f(b: 2, a: 1) }`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "named.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			count++
			if len(call.ArgNames) != 2 {
				t.Fatalf("labels: %#v", call.ArgNames)
			}
		}
		return true
	})
	if count != 2 {
		t.Fatalf("calls: %d", count)
	}
	var out bytes.Buffer
	if err := format.Node(&out, fset, f); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "roundtrip.go", out.Bytes(), parser.SkipObjectResolution); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("f(b: 2, a: 1)")) {
		t.Fatal(out.String())
	}
}
