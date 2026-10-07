package types_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	. "go/types"
)

func TestErrorHandling(t *testing.T) {
	const declarations = `package p
 type E = error
 type other interface { Error() string }
 type concrete struct{}
 func (concrete) Error() string { return "error" }
 func read() (int, E) { return 1, nil }
 func pair() (int, string, error) { return 1, "", nil }
 func save() error { return nil }
 func wrong() (int, concrete) { return 0, concrete{} }
 func use(int, string) {}
 `
	for _, body := range []string{
		`func f() error { x := read()!; _ = x; return nil }`,
		`func f() E { x := (read())!; _ = x; return nil }`,
		`func f() error { a, b := pair()!; use(a, b); use(pair()!); save()!; return nil }`,
		`func f() int { x := read() or err { return 0 }; return x }`,
		`func f() { save() or err {}; save() or _ {} }`,
		`func f() (int, error) { return read() or err { return 0, err }, nil }`,
		`func f() error { x := read() or err { for { break }; return err }; _ = x; return nil }`,
		`func f() error { x := read() or err { func(){ L: for { break L } }(); return err }; _ = x; return nil }`,
		`func f() error { err := 1; x := read() or err { return err }; _, _ = err, x; return nil }`,
		`func f() { _ = read() or err { select {} } }`,
		`func generic[T any](x T) (T, error) { return x, nil }; func f() error { x := generic(1)!; _ = x; return nil }`,
	} {
		if _, err := typecheck(declarations+body, nil, nil); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
	for _, test := range []struct{ body, want string }{
		{`var x = read()!`, "only permitted inside a function"},
		{`func f() { _ = read()! }`, "enclosing function"},
		{`func f() error { x := save()!; _ = x; return nil }`, "no value"},
		{`func f() error { x := 42; _ = read() or err { x := x; _ = x }; return nil }`, "must terminate"},
		{`func f() other { _ = read()!; return nil }`, "enclosing function"},
		{`func f() error { _ = wrong()!; return nil }`, "requires a final result of type error"},
		{`func f() error { x := 1; _ = x!; return nil }`, "requires a function or method call"},
		{`func f() error { _ = error(nil)!; return nil }`, "function or method call"},
		{`func f() { x := read() or err {}; _ = x }`, "must terminate"},
		{`func f() { save() or err {}; _ = err }`, "undefined: err"},
		{`func f() error { g := func() { save()! }; g(); return nil }`, "enclosing function"},
		{`func f() error { const x = read()!; return nil }`, "is not constant"},
		{`func f() { for { save() or err { break } } }`, "break not in"},
		{`func f() { save() or err { goto L }; L: return }`, "branches to labels are not permitted"},
		{`func f() { save() or err { L: return } }`, "labels are not permitted"},
	} {
		if _, err := typecheck(declarations+test.body, nil, nil); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: got %v, want %q", test.body, err, test.want)
		}
	}
	info := &Info{Types: make(map[ast.Expr]TypeAndValue), Defs: make(map[*ast.Ident]Object)}
	if _, err := typecheck(declarations+`func f() { x := read() or err { panic(err) }; _ = x }`, nil, info); err != nil {
		t.Fatal(err)
	}
	found := false
	for expr, tv := range info.Types {
		if e, ok := expr.(*ast.ErrorExpr); ok {
			found = true
			if !Identical(tv.Type, Typ[Int]) || ExprString(e) != "read() or err {…}" {
				t.Errorf("incorrect handler type or expression string: %v / %s", tv.Type, ExprString(e))
			}
			if obj := info.Defs[e.Err]; obj == nil || !Identical(obj.Type(), Universe.Lookup("error").Type()) {
				t.Errorf("missing error binding definition: %v", obj)
			}
		}
	}
	if !found {
		t.Fatal("no ErrorExpr type information")
	}
}

func TestErrorHandlingEval(t *testing.T) {
	const src = `package p
func read() (int, error) { return 1, nil }
func outer() error {
 /* outer */
 later := 1
 _ = later
 _ = func() {
  /* inner */
 }
 _ = read() or err {
  /* handler */
  return err
 }
 return nil
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := new(Config).Check("p", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	file := fset.File(f.Pos())
	pos := func(marker string) token.Pos { return file.Pos(strings.Index(src, marker)) }
	for _, test := range []struct {
		marker, expr, want string
	}{
		{"/* outer */", "read()!", ""},
		{"/* handler */", "read()!", ""},
		{"/* inner */", "read()!", "enclosing function"},
		{"package", "read()!", "only permitted inside a function"},
		{"outer()", "read()!", "only permitted inside a function"},
		{"/* outer */", "read() or err { local := err; return local }", ""},
		{"/* outer */", "read() or err { _ = read() or inner { return err }; return err }", ""},
		{"/* outer */", "read() or err { _ = later; return err }", "undefined: later"},
		{"/* outer */", "read() or err { _ = before; before := err; return before }", "undefined: before"},
		{"/* inner */", "read() or err { return }", ""},
	} {
		tv, err := Eval(fset, pkg, pos(test.marker), test.expr)
		if test.want != "" {
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("at %s: Eval(%q) = %v, want %q", test.marker, test.expr, err, test.want)
			}
		} else if err != nil || !Identical(tv.Type, Typ[Int]) {
			t.Errorf("at %s: Eval(%q) = (%v, %v), want int", test.marker, test.expr, tv.Type, err)
		}
	}
}

func TestErrorContextTypes(t *testing.T) {
	const prelude = `package p
func one() (int, error) { return 1, nil }
func many() (int, string, error) { return 1, "", nil }
func only() error { return nil }
func wrap(err error) error { return err }
func consume(int, int) {}
type E struct{}
func (E) Error() string { return "" }
`
	for _, source := range []string{
		`func f() (int, error) { n := one() or err => wrap(err); return n, nil }`,
		`func f() (int, string, error) { n, s := many() or err => wrap(err); return n, s, nil }`,
		`func f() error { only() or _ => nil; one() or err => E{}; return nil }`,
		`type Alias = error; func f() Alias { only() or err => err; return nil }`,
		`func f() error { err := 3; consume(one() or err => wrap(err), err); return nil }`,
		`func f() error { _ = one() or err => func() error { return err }(); return nil }`,
		`func f() error { consume(one() or err => wrap(err), one() or other => wrap(other)); return nil }`,
		`func f() error { one() or err => func() error { only() or nested => wrap(nested); return err }(); return nil }`,
	} {
		if _, err := typecheck(prelude+source, nil, nil); err != nil { t.Errorf("%s: %v", source, err) }
	}
	for _, tc := range []struct{ source, want string }{
		{`func f() { one() or err => err }`, "enclosing function"},
		{`func f() error { one() or err => 1; return nil }`, "error context"},
		{`func f() error { one() or err => many(); return nil }`, "multiple-value"},
		{`func f() error { one() or err => err; _ = err; return nil }`, "undefined: err"},
		{`func f() error { n := one() or err => wrap(n); return nil }`, "undefined: n"},
		{`func f() error { _ = func() int { return one() or err => err }; return nil }`, "enclosing function"},
	} {
		if _, err := typecheck(prelude+tc.source, nil, nil); err == nil || !strings.Contains(err.Error(), tc.want) { t.Errorf("%s: got %v, want %s", tc.source, err, tc.want) }
	}
}
