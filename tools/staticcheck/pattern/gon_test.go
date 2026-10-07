package pattern

import (
	"go/parser"
	"testing"
)

func TestGonRepeatedBinding(t *testing.T) {
	pattern := MustParse(`(BinaryExpr same _ same)`)
	for _, source := range []string{
		`$"${1}" == $"${1}"`,
		`(value is E.A) == (value is E.A)`,
		`(call() or err => err) == (call() or err => err)`,
		`(if flag { 1 } else { 2 }) == (if flag { 1 } else { 2 })`,
	} {
		t.Run(source, func(t *testing.T) {
			expr, err := parser.ParseExpr(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, matched := Match(pattern, expr); matched {
				t.Fatal("repeated Gon computation treated as equivalent")
			}
		})
	}
}
