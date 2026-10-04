package types_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestNamedArguments(t *testing.T) {
	prelude := `package p
func f(a int, b string) {}
func v(prefix string, values ...int) {}
func generic[T any](first T, second []T) T { return first }
`
	for _, test := range []struct{ call, error string }{
		{`f(b: "b", a: 1)`, ""},
		{`f(1, b: "b")`, ""},
		{`v(prefix: "p", values: []int{1}...)`, ""},
		{`v(prefix: "p")`, ""},
		{`generic(second: []int{1}, first: 2)`, ""},
		{`f(a: 1, a: 2)`, "duplicate argument"},
		{`f(a: 1)`, "missing argument"},
		{`f(c: 1, b: "b")`, "unknown argument"},
		{`f(a: 1, "b")`, "positional argument"},
		{`v(prefix: "p", values: 1)`, "named variadic"},
		{`int(value: 1)`, "function signature"},
		{`len(value: "x")`, "function signature"},
		{`var x func(int, string) = f; x(a: 1, b: "b")`, "unknown argument"},
		{`func(a int, b string) {}(b: "b", a: 1)`, ""},
		{`f(a: func() (int, int) { return 1, 2 }(), b: "b")`, "single-valued"},
	} {
		t.Run(test.call, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "named.go", prelude+"func g(){"+test.call+"}", parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
			var messages []string
			conf := types.Config{Error: func(err error) { messages = append(messages, err.Error()) }}
			_, err = conf.Check("p", fset, []*ast.File{f}, info)
			if test.error == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(strings.Join(messages, "\n"), test.error) {
				t.Fatalf("want %s; got %v", test.error, messages)
			}
		})
	}
}
