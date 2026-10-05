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

// TestNamedArgumentsRecordedTypes checks that a named call finalizes the types of its
// arguments like the equivalent positional call: untyped constants and the
// untyped boolean results of comparisons are converted to their parameter
// types even if the written order differs from the parameter order.
func TestNamedArgumentsRecordedTypes(t *testing.T) {
	const prefix = `package p
type B bool
func f(valid bool, n int, r rune, s string) {}
func fb(valid B, n float64) {}
func fa(v, w any) {}
func fp(p *int, e error) {}
func generic[T any](ok bool, v T) {}
func sum[T ~int | ~float64](a, b T) T { return a + b }
func variadic(ok bool, rest ...int) {}
func call(ok bool, fn func(int) bool) {}
type T struct{}
func (t T) M(ok bool, n int) {}
type I interface{ M(ok bool, n int) }
var a, b, x = 1, 2, 3
var i I
`
	for _, test := range []struct {
		params           []string
		positional, call string
		want             []string // argument types in parameter order; "" if not checked
	}{
		{[]string{"valid", "n", "r", "s"}, `f(a < b, 1<<3, 'a', "z")`, `f(s: "z", r: 'a', n: 1<<3, valid: a < b)`, []string{"bool", "int", "rune", "string"}},
		{[]string{"valid", "n", "r", "s"}, `f(a == b, 1, 2, "")`, `f(a == b, s: "", r: 2, n: 1)`, []string{"bool", "int", "rune", "string"}},
		{[]string{"valid", "n", "r", "s"}, `f(!(a == b) && a > 0 || b > 1, 1, 2, "")`, `f(n: 1, valid: !(a == b) && a > 0 || b > 1, s: "", r: 2)`, []string{"bool", "int", "rune", "string"}},
		{[]string{"valid", "n", "r", "s"}, `f(1 == 1, 1, 2, "a"+"b")`, `f(s: "a"+"b", r: 2, valid: 1 == 1, n: 1)`, []string{"bool", "int", "rune", "string"}},
		{[]string{"valid", "n"}, `fb(a < b, 1)`, `fb(n: 1, valid: a < b)`, []string{"p.B", "float64"}},
		{[]string{"v", "w"}, `fa(a < b, 1<<3)`, `fa(w: 1<<3, v: a < b)`, []string{"bool", "int"}},
		{[]string{"v", "w"}, `fa('a', 1.5)`, `fa(w: 1.5, v: 'a')`, []string{"rune", "float64"}},
		{[]string{"v", "w"}, `fa(1<<x, "s")`, `fa(w: "s", v: 1<<x)`, []string{"int", "string"}},
		{[]string{"v", "w"}, `fa(nil, nil)`, `fa(w: nil, v: nil)`, nil},
		{[]string{"p", "e"}, `fp(nil, nil)`, `fp(e: nil, p: nil)`, nil},
		{[]string{"ok", "v"}, `generic(a < b, 1)`, `generic(v: 1, ok: a < b)`, []string{"bool", "int"}},
		{[]string{"ok", "v"}, `generic(a < b, a < b)`, `generic(v: a < b, ok: a < b)`, []string{"bool", "bool"}},
		{[]string{"ok", "v"}, `generic(a < b, 'a')`, `generic(v: 'a', ok: a < b)`, []string{"bool", "rune"}},
		{[]string{"a", "b"}, `sum(1, 2.5)`, `sum(b: 2.5, a: 1)`, []string{"float64", "float64"}},
		{[]string{"ok", "rest"}, `variadic(a < b, []int{1}...)`, `variadic(ok: a < b, rest: []int{1}...)`, []string{"bool", "[]int"}},
		{[]string{"ok", "rest"}, `variadic(a < b)`, `variadic(ok: a < b)`, []string{"bool"}},
		{[]string{"ok", "fn"}, `call(a < b, func(int) bool { return a < b })`, `call(fn: func(int) bool { return a < b }, ok: a < b)`, []string{"bool", "func(int) bool"}},
		{[]string{"ok", "n"}, `T{}.M(a < b, 1)`, `T{}.M(n: 1, ok: a < b)`, []string{"bool", "int"}},
		{[]string{"ok", "n"}, `i.M(a < b, 1)`, `i.M(n: 1, ok: a < b)`, []string{"bool", "int"}},
		{[]string{"t", "ok", "n"}, `T.M(T{}, a < b, 1)`, `T.M(n: 1, ok: a < b, t: T{})`, []string{"p.T", "bool", "int"}},
	} {
		t.Run(test.call, func(t *testing.T) {
			// argumentTypes returns the recorded type and constant value of
			// each argument of the first call in the body of g, by parameter.
			argumentTypes := func(stmt string) map[string]string {
				f := mustParse(prefix + "func g(){" + stmt + "}")
				info := &types2.Info{Types: map[syntax.Expr]types2.TypeAndValue{}}
				conf := types2.Config{Importer: defaultImporter()}
				if _, err := conf.Check("p", []*syntax.File{f}, info); err != nil {
					t.Fatalf("%s: %v", stmt, err)
				}
				decl := f.DeclList[len(f.DeclList)-1].(*syntax.FuncDecl)
				var call *syntax.CallExpr
				syntax.Inspect(decl.Body, func(n syntax.Node) bool {
					if c, ok := n.(*syntax.CallExpr); ok && call == nil {
						call = c
					}
					return call == nil
				})
				types := map[string]string{}
				for i, arg := range call.ArgList {
					name := test.params[i]
					if i < len(call.ArgNames) && call.ArgNames[i] != nil {
						name = call.ArgNames[i].Value
					}
					tv := info.Types[arg]
					if tv.Type == nil {
						t.Fatalf("%s: no type recorded for argument %s", stmt, syntax.String(arg))
					}
					types[name] = tv.Type.String()
					if tv.Value != nil {
						types[name] += " " + tv.Value.String()
					}
				}
				return types
			}
			want, got := argumentTypes(test.positional), argumentTypes(test.call)
			for name, typ := range want {
				if got[name] != typ {
					t.Errorf("argument %s: named call has type %q; positional call has %q", name, got[name], typ)
				}
			}
			for i, typ := range test.want {
				name := test.params[i]
				if typ != "" && !strings.HasPrefix(got[name], typ) {
					t.Errorf("argument %s: got type %q, want %q", name, got[name], typ)
				}
				if typ != "" && strings.HasPrefix(got[name], "untyped") {
					t.Errorf("argument %s: type %q is untyped", name, got[name])
				}
			}
		})
	}
}
