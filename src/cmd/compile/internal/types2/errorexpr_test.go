package types2_test

import (
	"cmd/compile/internal/syntax"
	"strings"
	"testing"

	. "cmd/compile/internal/types2"
)

func TestErrorHandlingTypes(t *testing.T) {
	const prelude = `package p
func one() (int, error) { return 1, nil }
func many() (int, string, error) { return 1, "", nil }
func only() error { return nil }
func plain() int { return 1 }
type E struct{}
func (E) Error() string { return "" }
func concrete() (int, E) { return 1, E{} }
`
	for _, body := range []string{
		`func f() (int, error) { n := one()!; return n, nil }`,
		`func f() (int, error) { n := (one())!; return n, nil }`,
		`func f() (int, string, error) { n, s := many()!; return n, s, nil }`,
		`func f() error { only()!; one()!; many()!; return nil }`,
		`func f() int { n := one() or err { return 0 }; return n }`,
		`func f() { only() or err {}; only() or _ {} }`,
		`func f() { _ = one() or err { panic(err) } }`,
		`func f() (int, error) { return func() int { return one() or err { return 0 } }(), nil }`,
		`func f() error { only() or err { for { break }; switch { default: break } }; return nil }`,
		`type Alias = error; func f() Alias { only()!; return nil }`,
		`func f() error { var error int; _ = error; only()!; return nil }`,
		`func f() error { defer func(int) {}(one()!); go func(int) {}(one()!); return nil }`,
	} {
		if _, err := typecheck(prelude+body, nil, nil); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}

	for _, tt := range []struct{ body, want string }{
		{`func f() error { error(nil)!; return nil }`, "requires a function or method call"},
		{`var _ = one()!`, "only permitted inside a function"},
		{`func f() { only()! }`, "enclosing function"},
		{`func f() error { plain()!; return nil }`, "final result of type error"},
		{`func f() error { concrete()!; return nil }`, "final result of type error"},
		{`func f() E { only()!; return E{} }`, "enclosing function"},
		{`func f() error { true!; return nil }`, "requires a function or method call"},
		{`func f() { _ = one() or err {} }`, "must terminate"},
		{`func f() { n := one() or err { _ = n; return }; _ = n }`, "undefined: n"},
		{`func f() { only() or err {}; _ = err }`, "undefined: err"},
		{`func f() error { _ = func() int { return one()! }; return nil }`, "enclosing function"},
		{`func f() error { const n = one()!; return nil }`, "is not constant"},
		{`func f() error { defer only()!; return nil }`, "expression in defer must be function call"},
		{`func f() error { go only()!; return nil }`, "expression in go must be function call"},
		{`func f() { for { only() or err { break } } }`, "break not in"},
		{`func f() { only() or err { L: return } }`, "labels are not permitted"},
		{`func f() { only() or err { goto L }; L: return }`, "branches to labels"},
	} {
		if _, err := typecheck(prelude+tt.body, nil, nil); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: got %v; want %q", tt.body, err, tt.want)
		}
	}
}

func TestErrorHandlerBindingInfo(t *testing.T) {
	info := &Info{Defs: make(map[*syntax.Name]Object), Uses: make(map[*syntax.Name]Object), Scopes: make(map[syntax.Node]*Scope)}
	_, err := typecheck(`package p; func f() error { return nil }; func g() { f() or failure { _ = failure } }`, nil, info)
	if err != nil {
		t.Fatal(err)
	}
	var binding Object
	for id, obj := range info.Defs {
		if id.Value == "failure" {
			binding = obj
		}
	}
	if binding == nil || !Identical(binding.Type(), Universe.Lookup("error").Type()) {
		t.Fatalf("unexpected binding: %v", binding)
	}
	for id, obj := range info.Uses {
		if id.Value == "failure" && obj == binding {
			return
		}
	}
	t.Error("handler error use does not resolve to binding")
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
		{`func f() error { go only() or err => err; return nil }`, "expression in go must be function call"},
		{`func f() error { defer only() or err => err; return nil }`, "expression in defer must be function call"},
		{`func f() error { _ = func() int { return one() or err => err }; return nil }`, "enclosing function"},
	} {
		if _, err := typecheck(prelude+tc.source, nil, nil); err == nil || !strings.Contains(err.Error(), tc.want) { t.Errorf("%s: got %v, want %s", tc.source, err, tc.want) }
	}
}
