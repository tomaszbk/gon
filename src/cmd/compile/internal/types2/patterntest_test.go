package types2_test

import (
	"cmd/compile/internal/syntax"
	. "cmd/compile/internal/types2"
	"strings"
	"testing"
)

func TestPatternTestTypes(t *testing.T) {
	const source = `package p
 type Shape enum {default Empty;Circle(int);Record{Radius int}}
 func (Shape) Error()string{return "shape"}
 type Flag bool
 var _ Flag = Shape.Empty is Shape.Circle(_)
 func valid[T any](value T?)bool{return value is nil}
 func simple(value int?)int{if value is number? {return number};return 0}
 func chained(value Shape, other int?)int{
  if value is Shape.Circle(number) && number>0 && other is second? && second>number{return second}
  return 0
 }
 func record(err error)int{if err is Shape.Record{Radius: radius} && radius>0{return radius};return 0}
 func optionalRecord(value Shape?)int{if value is Shape.Record{Radius:radius}? && radius>0{return radius};return 0}
 func repeated(value Shape)bool{if value is Shape.Record{...} is true{return true};return false}
 func unit(value Shape)bool{if value is Shape.Empty {} else if value is Shape.Circle(_) {return true};return !(value is Shape.Record{...})}
 func shadow(value int?)int{number:=42;if value is number? {return number}else{return number}}
 func contextual(is int)bool{return is==3}
 `
	info := &Info{Defs: make(map[*syntax.Name]Object), Uses: make(map[*syntax.Name]Object), Scopes: make(map[syntax.Node]*Scope)}
	if _, err := typecheck(source, nil, info); err != nil {
		t.Fatal(err)
	}
	var bindings int
	for ident, obj := range info.Defs {
		if ident.Value == "number" && obj != nil && strings.Contains(obj.Parent().String(), "if pattern bindings") {
			bindings++
		}
	}
	if bindings != 3 {
		t.Fatalf("pattern scopes bindings=%d", bindings)
	}
}

func TestPatternTestInvalid(t *testing.T) {
	const prefix = `package p;type E enum{default A;B(int)};var optional int?;var e E;`
	for _, tc := range []struct{ source, want string }{
		{`var _ = optional is number?`, "bindings require an if"},
		{`func f(){if optional is number? || true {_=number}}`, "bindings require an if"},
		{`func f(){if !(optional is number?) {_=number}}`, "bindings require an if"},
		{`func f(){for optional is number? {_=number}}`, "bindings require an if"},
		{`func use(bool){};func f(){use(optional is number?)}`, "bindings require an if"},
		{`func f(){if optional is number? {} else {_=number}}`, "undefined"},
		{`func f(){if optional is number? {};_=number}`, "undefined"},
		{`func f(){if optional is number? && optional is number? {_=number}}`, "redeclared"},
		{`var _ = optional is _`, "always matches"},
		{`func f(){if optional is number {_=number}}`, "always matches"},
		{`type Unit enum{default Only};var value Unit;var _ = value is Unit.Only`, "always matches"},
		{`var _ = e is E.Missing`, "unknown"},
		{`var _ = optional is "wrong"`, "cannot use"},
	} {
		_, err := typecheck(prefix+tc.source, nil, nil)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error=%v;want %q", tc.source, err, tc.want)
		}
	}
}
