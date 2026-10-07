// Copyright 2026 The Gon Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2_test

import (
	"cmd/compile/internal/syntax"
	"strings"
	"testing"

	. "cmd/compile/internal/types2"
)

func TestMatchTypes(t *testing.T) {
	const src = `package p
type State enum { default Idle; Count(int); Named { Label string; N int } }
type Nested enum { default Empty; Item(State) }
var b bool
var state State
var nested Nested
var _ int8 = switch b { case true => 1; case false => 2 }
var _ = switch b { case true => 1; case false => 2.5 }
var _ any = switch b { case true => 1; case false => 2.5 }
var _ = switch state {
case State.Idle => 0
case State.Count(n) if n > 0 => n
case State.Count(_) => 0
case State.Named{Label: _, N: n} => n
}
var _ = switch nested {
case Nested.Empty => "empty"
case Nested.Item(State.Idle) => "idle"
case Nested.Item(State.Count(_)) => "count"
case Nested.Item(State.Named{...}) => "named"
}
func f(s State) int {
switch s {
case State.Idle => { return 0 }
case State.Count(n) => { return n }
case State.Named{N: n, ...} => { return n }
}
}
func g(s State) {
L: switch s { default => { break L } }
}
`
	f := mustParse(src)
	info := &Info{Types: make(map[syntax.Expr]TypeAndValue), Defs: make(map[*syntax.Name]Object), Uses: make(map[*syntax.Name]Object)}
	if _, err := new(Config).Check("p", []*syntax.File{f}, info); err != nil {
		t.Fatal(err)
	}
	var matches []*syntax.MatchExpr
	syntax.Inspect(f, func(n syntax.Node) bool {
		if e, ok := n.(*syntax.MatchExpr); ok {
			matches = append(matches, e)
		}
		return true
	})
	for i, want := range []string{"int8", "float64", "any", "int", "string"} {
		if got := info.Types[matches[i]].Type.String(); got != want {
			t.Errorf("match %d type %s; want %s", i, got, want)
		}
		if info.Types[matches[i]].Value != nil {
			t.Errorf("match %d is constant", i)
		}
	}
	var defs int
	for id, obj := range info.Defs {
		if id.Value == "n" && obj != nil {
			defs++
		}
	}
	if defs != 4 {
		t.Fatalf("pattern n definitions %d, want 4", defs)
	}
}

func TestMatchInvalid(t *testing.T) {
	prefix := `package p; type E enum { default A; B(int); C { X int; Y bool } }; var e E; var b bool; `
	for _, test := range []struct{ src, want string }{
		{`var _ = switch e { case E.A => 1; case E.B(_) => 2 }`, "non-exhaustive"},
		{`var _ = switch e { case E.A if b => 1; case E.B(_) => 2; case E.C{...} => 3 }`, "non-exhaustive"},
		{`var _ = switch e { default => 1; case E.A => 2 }`, "unreachable"},
		{`var _ = switch e { case E.C{X: x} => x; default => 0 }`, "every field"},
		{`var _ = switch e { case E.B(x, x) => x; default => 0 }`, "payload patterns"},
		{`var _ = switch e { case E.C(x, y) => x; default => 0 }`, "record pattern"},
		{`var _ = switch e { case E.B(n) if n => n; default => 0 }`, "guard must be boolean"},
		{`var _ = switch e { case E.C{X: x, Y: x} => x; default => 0 }`, "redeclared"},
		{`var _ = switch e { case E.B(n) => n; default => n }`, "undefined"},
		{`var _ = switch b { case true => 1 }`, "missing false"},
		{`var _ = switch b { case true => 1; case false => "bad" }`, "incompatible"},
		{`func f(){ switch e { default => { fallthrough } } }`, "fallthrough"},
		{`const item = 1; var _ = switch 1 { case item => 2; default => 0 }`, "qualified alternative"},
		{`var _ = switch e { case A => 1; default => 0 }`, "qualified alternative"},
		{`var _ = switch e { case B(n) => n; default => 0 }`, "qualified enum alternative"},
		{`const true = 1; var _ = switch e { case E.B(true) => 1; default => 0 }`, "cannot use true"},
		{`const nil = 1; var _ = switch e { case E.B(nil) => 1; default => 0 }`, "cannot use nil"},
	} {
		f := mustParse(prefix + test.src)
		_, err := new(Config).Check("p", []*syntax.File{f}, nil)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: error %v; want %q", test.src, err, test.want)
		}
	}
}

func TestMatchPatternNames(t *testing.T) {
	const src = `package p
type E enum { default Empty; Number(int); Flags(bool, *int) }
func f(e E, b bool, p *int) int {
    const item = 99
    true, false, nil := 10, 20, 30
    _ = item
    _ = switch b { case true => true; case false => false }
    _ = switch p { case nil => nil; default => 0 }
    return switch e {
    case E.Empty => nil
    case E.Number(item) => item
    case E.Flags(true, nil) => true
    case E.Flags(false, _) => false
    case E.Flags(_, _) => nil
    }
}
`
	f := mustParse(src)
	info := &Info{Types: make(map[syntax.Expr]TypeAndValue), Defs: make(map[*syntax.Name]Object), Uses: make(map[*syntax.Name]Object)}
	if _, err := new(Config).Check("p", []*syntax.File{f}, info); err != nil {
		t.Fatal(err)
	}
	contextual, bindings := 0, 0
	syntax.Inspect(f, func(node syntax.Node) bool {
		p, ok := node.(*syntax.MatchPattern)
		if !ok {
			return true
		}
		n, ok := p.Value.(*syntax.Name)
		if !ok {
			return true
		}
		switch n.Value {
		case "true", "false", "nil":
			contextual++
			if info.Uses[n] != Universe.Lookup(n.Value) || info.Defs[n] != nil {
				t.Errorf("pattern %s must refer to its predeclared value, got use %v and definition %v", n.Value, info.Uses[n], info.Defs[n])
			}
			if tv, ok := info.Types[n]; !ok || tv.Type == nil {
				t.Errorf("missing type information for contextual pattern %s", n.Value)
			}
		case "item":
			bindings++
			if _, ok := info.Defs[n].(*Var); !ok || info.Uses[n] != nil {
				t.Errorf("payload item must bind a new variable, got definition %v and use %v", info.Defs[n], info.Uses[n])
			}
		}
		return true
	})
	if contextual != 6 || bindings != 1 {
		t.Fatalf("pattern counts: contextual=%d bindings=%d, want 6 and 1", contextual, bindings)
	}
}

const matchInterfacePrefix = `package p
type Reader interface{ Read([]byte) (int, error) }
type E enum { default A; B(int); C { N int; Msg string } }
func (E) Error() string { return "" }
func (E) Read([]byte) (int, error) { return 0, nil }
type F enum { default X; Y(error) }
func (F) Error() string { return "" }
type N enum { default A }
type P enum { default A }
func (*P) Error() string { return "" }
type S struct{ X int }
var err error
var r Reader
var a any
var o int?
`

func TestMatchInterfaceSubject(t *testing.T) {
	src := matchInterfacePrefix + `
type alias = error
type Wrap enum { default Z; W(error) }
var aliased alias
var wrapped Wrap
var _ = switch err {
case E.A => 1
case E.B(n) if n > 0 => n
case E.B(_) => 2
case E.C{N: n, ...} => n
case F.Y(inner) => len(inner.Error())
case F.X => 3
default => 0
}
var _ = switch r { case E.A => "a"; case E.C{Msg: m, N: _} => m; default => "d" }
var _ = switch a { case E.B(n) => n; case F.Y(_) => 0; default => 0 }
var _ = switch aliased { case E.A => 1; default => 0 }
var _ = switch wrapped { case Wrap.W(E.B(n)) => n; case Wrap.W(_) => 0; default => 1 }
func g(err error, want error) string {
	switch err {
	case E.A => { return "a" }
	case _ if err == want => { return "w" }
	default => { return "d" }
	}
}
`
	f := mustParse(src)
	info := &Info{Types: make(map[syntax.Expr]TypeAndValue), Defs: make(map[*syntax.Name]Object), Uses: make(map[*syntax.Name]Object)}
	if _, err := new(Config).Check("p", []*syntax.File{f}, info); err != nil {
		t.Fatal(err)
	}
	var patterns int
	syntax.Inspect(f, func(n syntax.Node) bool {
		p, ok := n.(*syntax.MatchPattern)
		if !ok {
			return true
		}
		sel, ok := p.Value.(*syntax.SelectorExpr)
		if !ok {
			return true
		}
		patterns++
		if _, ok := info.Uses[sel.Sel].(*Var); !ok && info.Uses[sel.Sel] == nil {
			t.Errorf("%v: variant %s has no use", sel.Pos(), sel.Sel.Value)
		}
		typ := info.Types[p.Value].Type
		if typ == nil || EnumOf(typ) == nil || typ.String() != info.Types[sel.X].Type.String() {
			t.Errorf("%v: pattern %s has type %v, want its enum qualifier", sel.Pos(), sel.Sel.Value, typ)
		}
		return true
	})
	if patterns != 15 {
		t.Fatalf("variant patterns %d, want 15", patterns)
	}
}

func TestMatchInterfaceSubjectInvalid(t *testing.T) {
	for _, test := range []struct{ src, want string }{
		{`var _ = switch err { case E.A => 1; case E.B(_) => 2; case E.C{...} => 3 }`, "non-exhaustive match: missing _"},
		{`var _ = switch err { case E.A => 1 }`, "enum alternatives never cover the interface type error"},
		{`var _ = switch a { case E.A => 1 }`, "interface type any"},
		{`var _ = switch err { case N.A => 1; default => 0 }`, "N does not implement error"},
		{`var _ = switch err { case P.A => 1; default => 0 }`, "pointer receiver"},
		{`var _ = switch r { case F.X => 1; default => 0 }`, "F does not implement Reader"},
		{`var _ = switch err { case S.X => 1; default => 0 }`, "qualified by an enum type"},
		{`type Result[T,E any] enum { default Ok(T); Err(E) }; var _ = switch err { case Result[int, error].Ok(v) => v; default => 0 }`, "can never match interface error"},
		{`var _ = switch err { case value? => 1; default => 0 }`, "presence pattern requires an optional value"},
		{`var _ = switch err { case v => 1; default => 0 }`, "top-level pattern requires a qualified alternative"},
		{`var _ = switch err { case E.A => 1; case E.A => 2; default => 0 }`, "unreachable match arm"},
		{`var _ = switch err { case E.B(1) => 1; case E.B(n) => n; case E.B(_) => 2; default => 0 }`, "unreachable match arm"},
		{`var _ = switch err { default => 0; case E.A => 1 }`, "unreachable match arm"},
		{`var _ = switch err { case E.Z => 1; default => 0 }`, "unknown or inaccessible alternative Z"},
		{`var _ = switch err { case E.B => 1; default => 0 }`, "positional pattern requires 1 payload patterns"},
		{`var _ = switch err { case E.C{N: n} => n; default => 0 }`, "every field"},
		{`var _ = switch err { case E.B(x) => x; default => x }`, "undefined: x"},
		{`func h[T any](v T) int { return switch v { case E.A => 1; default => 0 } }`, "value pattern"},
		{`var _ = switch o { case E.A => 1; default => 0 }`, "value pattern must be a literal or qualified constant"},
	} {
		f := mustParse(matchInterfacePrefix + test.src)
		_, err := new(Config).Check("p", []*syntax.File{f}, nil)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: error %v; want %q", test.src, err, test.want)
		}
	}
}
