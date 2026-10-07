package types2_test

import (
	"cmd/compile/internal/types2"
	"strings"
	"testing"
)

func TestOptionalOperators(t *testing.T) {
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
		{`type Counter struct{};func(c *Counter)Increment(){};func f(x Counter?){x?.Increment()}`, "pointer method"},
		{`type Counter struct{};func(c *Counter)Increment(){};func f(x (*Counter)?){x?.Increment()}`, ""},
		{`func f(x int?) int {return x?}`, "enclosing function"},
		{`type Option[T any] struct{value T};func f(x Option[int]) Option[int] {_ = x?;return x}`, "optional value"},
	}
	for _, test := range cases {
		t.Run(test.source, func(t *testing.T) {
			var errs []string
			conf := types2.Config{Error: func(err error) { errs = append(errs, err.Error()) }}
			_, err := typecheck("package p;"+test.source, &conf, nil)
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
