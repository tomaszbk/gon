package types_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestOptionResultOperators(t *testing.T) {
	cases := []struct{ source, error string }{
		{`type User struct{Name string};type Node struct{User User?};func f(x *Node)string{return x?.User?.Name ?? "none"}`, "explicit boundary"},
		{`type User struct{Name string};type Node struct{User *User};func f(x Node?)string{return x?.User?.Name ?? "none"}`, "explicit boundary"},
		{`type User struct{Name string};func f(x (*User)?)string{return (x ?? nil)?.Name ?? "none"}`, ""},
		{`type User struct{Name string};type Node struct{User User?};func f(x *Node)string{var value User?;if x!=nil{value=x.User};return value?.Name ?? "none"}`, ""},

		{`func f(x int?) string? { n:=x?; return (string?)((string)(string(rune(n)))) }`, ""},
		{`func f(x int?) int {return x ?? 3}`, ""},
		{`func f(){var x int?;x ??= 3}`, ""},
		{`func f(m map[string]int?) {m["key"] ??= 3}`, ""},
		{`func makeOption() int? {return (int?)(nil)};func f(){makeOption() ??= 3}`, "cannot assign"},
		{`func f(){var x int?;x ??= (int?)((int)(3))}`, "cannot use"},
		{`type User struct{Name string};func f(x User?) string{return x?.Name ?? "none"}`, ""},
		{`type User struct{Name string};func f(x User?) any{return x?.Name}`, ""},
		{`func f(x Result[int,string]) Result[string,string] {n:=x!;return Result[string,string].Ok(string(rune(n)))}`, ""},
		{`func f(x Result[int,string]) int {return x or e {panic(e)}}`, ""},
		{`func f(x Result[int,error]) Result[int,any] {n:=x!;return Result[int,any].Ok(n)}`, ""},
		{`type Counter struct{};func(c *Counter)Increment(){};func f(x Counter?){x?.Increment()}`, "pointer method"},
		{`type Counter struct{};func(c *Counter)Increment(){};func f(x (*Counter)?){x?.Increment()}`, ""},
		{`func f(x int?) int {return x?}`, "enclosing function"},
		{`func f(x Result[int,string]) int {return x!}`, "enclosing Result"},
		{`func f(x Result[int,string]) Result[int,error] {n:=x!;return Result[int,error].Ok(n)}`, "assignable error"},
		{`func f(x Result[int,string]) int {return x or e {_ = e}}`, "must terminate"},
		{`func f(x Result[int,string]) int {return x ?? 0}`, "operator ??"},
		{`type Option[T any] struct{value T};func f(x Option[int]) Option[int] {_ = x?;return x}`, "optional value"},
	}
	for _, test := range cases {
		t.Run(test.source, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "operators.go", "package p;"+test.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			var errs []string
			conf := types.Config{Error: func(err error) { errs = append(errs, err.Error()) }}
			_, err = conf.Check("p", fset, []*ast.File{f}, nil)
			if test.error == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(strings.Join(errs, "\n"), test.error) {
				t.Fatalf("want %s got %v", test.error, errs)
			}
		})
	}
}
