package types2_test

import (
	"cmd/compile/internal/syntax"
	"cmd/compile/internal/types2"
	"strings"
	"testing"
)

func TestNamedArguments(t *testing.T) {
	const prefix = `package p
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
		{`var x func(int, string) = f; x(a: 1, b: "b")`, "unknown argument"},
		{`f(a: func() (int, int) { return 1, 2 }(), b: "b")`, "single-valued"},
	} {
		t.Run(test.call, func(t *testing.T) {
			info := &types2.Info{Uses: map[*syntax.Name]types2.Object{}}
			_, err := typecheck(prefix+"func g(){"+test.call+"}", nil, info)
			if test.error == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.error) {
				t.Fatalf("want %s; got %v", test.error, err)
			}
		})
	}
}
